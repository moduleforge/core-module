# Multi-Key Versioned Cipher Core

## Purpose and scope

Rewrite `api/internal/fieldcrypto` from a single-key value into a reloadable multi-key keyring with a
version-tagged, AAD-bound blob format, and export the rotation-detection surface Phases 3 and 4
consume. This is the security-critical centre of the plan.

Files this task owns:

- `api/internal/fieldcrypto/fieldcrypto.go` — rewritten.
- `api/internal/fieldcrypto/fieldcrypto_test.go`, `generate_test.go`,
  `generate_integration_test.go` — reworked; new files may be added if the package splits cleanly.
- `api/fieldcrypto/fieldcrypto.go` — **minimally** updated to keep compiling (see requirement 9);
  [`002-facade-key-store-adapter.md`](./002-facade-key-store-adapter.md) owns its real shape.
- `api/service/{profile,natural_person,corporation,legal_entity}.go` and any `api/httpapi` caller —
  **mechanically** updated to pass `ctx` (see requirement 10); Phase 3 owns their real rewiring.

No standard skill covers this work. The design is fully specified in the referenced notes; implement
it as written rather than re-deriving it.

## Requirements

1. **Key types.** Declare `KeyRecord` (`Version uint32`, `KeyBytes []byte`, `RetiredAt`,
   `DecryptableUntil`, `CompromisedAt` as `*time.Time`) and the two-method `KeyStore` interface
   (`LoadUsableKeys(ctx) ([]KeyRecord, error)`, `InsertInitialKey(ctx, keyBytes []byte) (KeyRecord,
   error)`), exactly as
   [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-2-surface--apiinternalfieldcrypto)
   gives them. **This package must not import `github.com/moduleforge/core-model/db`** — keeping it
   model-free is what lets the façade absorb that dependency and leaves the manifest's `cipher`
   service block unchanged.
2. **Wire format.** `version(4) || nonce(12) || ciphertext || tag(16)`, version encoded big-endian
   with `binary.BigEndian.PutUint32`, and those exact 4 prefix bytes passed verbatim as the AEAD
   additional authenticated data (they are *not* part of the ciphertext handed to `AEAD.Open`).
   Minimum valid blob length is 32. `Encrypt(ctx, "")` still returns `[]byte{}` with no version
   prefix, unchanged from today. A decoded version of `0` is a malformed blob and fails immediately
   with no reload attempt, because the identity sequence starts at 1.
3. **Reloadable key set.** `Cipher` holds an immutable `keySet` snapshot behind an
   `atomic.Pointer[keySet]`, swapped wholesale so readers never observe a partial update, plus a
   reload mutex that collapses concurrent reloads into one query and a last-load-attempt timestamp.
   The snapshot carries the active version, a version-keyed map of built `cipher.AEAD` values, and
   its load time.
4. **The three ctx-carrying methods plus `Reload` and `BlobVersion`**, with the signatures given in
   the design note:
   - `Encrypt(ctx, plaintext string) ([]byte, error)` — encrypts only under the active key.
   - `Decrypt(ctx, blob []byte) (string, error)` — plaintext only, for callers that cannot rotate.
   - `DecryptWithRotation(ctx, blob []byte) (string, Rotation, error)`.
   - `Reload(ctx) error` — exported; Phase 4's rotation handler calls it post-commit.
   - `BlobVersion(blob []byte) (uint32, error)` — package-level; rejects length < 32 and version 0.
5. **The `Rotation` value type** with `FromVersion`, `ToVersion`, `Blob`, `MustPersist`, and
   `Needed() bool`. Its zero value means "nothing to do", which is what an empty blob and an
   already-current blob both produce. `MustPersist` is computed **inside the cipher**, from the
   `CompromisedAt` field of the `KeyRecord` that decrypted the blob — no caller ever reads
   `compromised_at`, and no caller re-derives policy.
6. **Failure and staleness behavior**, all three mechanisms from
   [the staleness resolution](../notes/rotation-api-shape.md#schema-open-question-3--stale-key-sets-across-processes):
   - **Grace expiry is re-checked in Go at decrypt time** against the loaded `DecryptableUntil`, not
     only by the SQL load filter. An expired key behaves exactly like an unknown version.
   - **Unknown version ⇒ one reload, then fail loudly.** Rate-limited by the last-load-attempt
     timestamp (suggested minimum interval 5s) so corrupt or hostile blobs cannot amplify into a
     query storm. Never try every loaded key; never return empty plaintext on failure.
   - **`Encrypt` reloads when the held set is older than `keySetTTL`** (suggested 60s) before
     selecting the active key. Without this a replica keeps encrypting under a retired key
     indefinitely, which defeats the whole point of a compromised-key rotation. A reload failure here
     logs at error level and proceeds with the existing set — failing every write because the key
     table is briefly unreachable is worse than a bounded window of stale encryption.
7. **Bootstrap and constructors.**
   - `NewFromEnvOrGenerate(ctx, store KeyStore)` loads via `LoadUsableKeys`; on an empty table it
     bootstraps through `InsertInitialKey`. Zero rows back (surfaced as `pgx.ErrNoRows` by the
     query's two-guard form) means another caller established the key material: re-run
     `LoadUsableKeys` and adopt the winner. If that re-load still finds no active key, fail hard —
     never generate a second time.
   - `CORE_FIELD_KEY_HEX` is a **first-boot-only seed**: on an empty table its 32 decoded bytes are
     what the bootstrap insert persists as version 1. On every later boot, bytes equal to the active
     key's bytes proceed silently; bytes that differ **fail construction loudly**, with an error
     naming `POST /v1/field-crypto-keys/rotations` as the correct way to change the active key. This
     is a deliberate, visible change from today's "env always wins".
   - `NewFromKey(key []byte)` stays exported: a store-less cipher pinned to version 1, holding
     exactly that key, that never reloads and always reports `Rotation{}` from
     `DecryptWithRotation`. This is what keeps `api/service/mock_test.go` and the round-trip tests
     working without a fake store.
   - **Delete `NewFromEnv`.** A cipher built from an env var alone has no version number and cannot
     legally encrypt anything under the DB-as-source-of-truth model; keeping it would ship a
     constructor producing blobs no other process can decrypt.
8. **Rewrite the package doc.** It currently declares key rotation "out of scope for this package"
   — the exact sentence app-mfmanager's deploy docs quote as justification for calling the key
   unrotatable — and states that no AAD is bound to ciphertext. The replacement must state the blob
   layout authoritatively, describe the active/retired key model and the grace and compromised
   semantics, and narrow the AAD sentence to "no *row identity* is bound", which remains true and
   remains a separate, acknowledged gap.
9. **Keep `api/fieldcrypto` compiling, minimally.** Delete its `NewFromEnv` wrapper and adjust its
   aliases and its `NewFromEnvOrGenerate` pass-through just enough that `go build ./...` succeeds.
   Do not build the `coredb` adapter here — task 002 owns it, and will replace whatever placeholder
   this task leaves.
10. **Thread `ctx` at the existing call sites, mechanically only.** `Encrypt`/`Decrypt` gaining a
    `context.Context` breaks the four `Encrypt` and six `Decrypt` sites in `api/service/`. Pass the
    `ctx` already in scope and keep calling `Decrypt` — **do not** introduce `DecryptWithRotation`,
    a write-back, or any policy here. Phase 3 replaces these calls wholesale.
11. **Tests.** Rework all three existing test files, which encode the untagged layout and the
    two-method querier contract, and cover at minimum:
    - round-trip under the active key, and decryption of blobs written under each retired key;
    - encryption always selects the active key, never a retired one;
    - unknown version fails loudly after one reload attempt, and the reload is rate-limited;
    - an expired `DecryptableUntil` makes a previously-usable key stop decrypting without a reload;
    - version 0 and a truncated (< 32 byte) blob are rejected as malformed;
    - tampering with the version prefix fails authentication (the AAD binding actually works);
    - `DecryptWithRotation` returns a zero `Rotation` for an empty blob and for a current blob, and a
      populated one with correct `MustPersist` for a retired and for a compromised source key;
    - the `-race` concurrency exercise is **extended to run `Encrypt`/`Decrypt` concurrently across a
      `Reload`**, which is the new hazard the atomic swap exists to close;
    - `generate_test.go`'s fake now implements `KeyStore`, covering every bootstrap branch including
      the lost-race path;
    - `generate_integration_test.go` (build tag `integration`) is rewritten against the replacement
      schema and still asserts that concurrent first-boot callers converge on one key.
12. **Followup `MnvB` — incidental, not required.** `fromPersistedOrGenerated` is rewritten wholesale
    here, which makes this the natural place to zero key byte slices after use. It is explicitly not
    a requirement and not an acceptance criterion; do it if it falls out cleanly, and say either way
    in the task report.

## Validation

- `cd api && make build` passes.
- `cd api && make test` passes.
- `cd api && make lint` passes (`go vet ./...` plus the `gofmt` check).
- `cd api && go test -race ./internal/fieldcrypto/...` passes, including the new
  concurrent-across-reload exercise.
- `cd api && go vet -tags integration ./internal/fieldcrypto/...` compiles the rewritten integration
  test; run it against a real database if one is available and report the outcome either way.
- `grep -rn "func NewFromEnv(" api/` returns nothing.
- `grep -rn "core-model/db\|coredb" api/internal/fieldcrypto/` returns nothing — the internal package
  stays model-free.
- `grep -n "out of scope" api/internal/fieldcrypto/fieldcrypto.go` returns nothing; the package doc
  no longer disclaims rotation.
- A blob produced by `Encrypt` is at least 32 bytes and its first 4 bytes decode to the active
  version (assert this in a test, not by inspection).
- `moduleforge.module.yaml` is unchanged by this task.

## Metadata

architectural_impact: true

## Assumptions

- Phase 1 has landed: `model/db/` carries the seven key queries, and `ListUsableFieldCryptoKeys`
  returns the full `FieldCryptoKey` row. This task does not call them directly — it codes against its
  own `KeyStore` — but the integration test does.
- At the end of this task the `api/fieldcrypto` façade compiles but does **not** yet satisfy the
  manifest's `queries:coredb` arg source. That gap is intentional and closes in task 002, within the
  same phase. Nothing in mod-core builds the cipher from a `*coredb.Queries`, so nothing in-repo
  detects it.
- The `Cipher` type is used by value-holders elsewhere as `*fieldcrypto.Cipher`; it stays a pointer
  type and stays safe for concurrent use.

## References

- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-2-surface--apiinternalfieldcrypto)
  — the authoritative surface: `KeyRecord`, `KeyStore`, `keySet`, `Cipher`, the three methods,
  `Rotation`, and the behavioral requirements list. Read this section and
  [Schema open question 3](../notes/rotation-api-shape.md#schema-open-question-3--stale-key-sets-across-processes)
  in full.
- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#version-identifier-integer-not-fingerprint)
  — the wire-format consequences, and
  [Grace expiry semantics](../notes/key-store-schema-design.md#grace-expiry-semantics) and
  [First-boot convergence and bootstrap](../notes/key-store-schema-design.md#first-boot-convergence-and-bootstrap).
- [`../notes/key-lifecycle-policy.md`](../notes/key-lifecycle-policy.md) — the settled
  `CORE_FIELD_KEY_HEX` precedence rule and its rationale.
- [`../notes/fieldcrypto-current-state.md`](../notes/fieldcrypto-current-state.md) — what exists
  today, including the three test files and what each encodes.

## Checkpoint hints

- After `KeyRecord`, `KeyStore`, `keySet`, and the wire-format encode/decode helpers compile with
  unit tests for the format alone.
- After `Encrypt`/`Decrypt`/`DecryptWithRotation`/`Reload`/`BlobVersion` are complete and the
  round-trip and unknown-version tests pass.
- After the constructors and bootstrap race handling are complete and `generate_test.go` passes.
- After the package doc rewrite.
- After the façade placeholder and the mechanical `ctx` threading make `cd api && make build` pass.
