# Overview

## Purpose and scope

Give mod-core's `fieldcrypto` package real key-rotation support, closing followup item `iQt1`.
Today `fieldcrypto.Cipher` holds exactly one 32-byte key and stored ciphertext carries no key
identity, so a leaked `CORE_FIELD_KEY_HEX` cannot be retired without a from-scratch data migration.
This plan introduces a versioned ciphertext format, an indefinite set of decrypt-only retired keys
alongside one active encrypt key, transparent re-encryption on read, and the admin HTTP surface that
actually triggers a rotation — the behavior the package doc has promised since it was written but
never implemented.

Scope is mod-core only: `api/internal/fieldcrypto/`, its façade `api/fieldcrypto/`, the
`model/migrations/` and `model/queries/` changes needed for multi-key storage, `model/db/`
regeneration, mod-core's own call sites in `api/service/`, a new admin handler in `api/httpapi/`, and
the manifest and OpenAPI entries that expose it. No file outside
`/Users/zane/playground/moduleforge/mod-core` is edited by any task in this plan; cross-repo
consequences are recorded under [Deferred and flagged](#deferred-and-flagged) instead.

## Current status

Planning is **complete**. All four open decisions are settled and recorded in `plan/notes/`; the
design is specified to the level of concrete DDL, SQL, Go signatures, route tables, and status
mappings, and the task documents consume that design rather than re-deriving it.

Five phases are registered with twelve tasks between them. No phase has begun. Phase 1 begins first and
has no pre-conditions beyond a working `sqlc` (v1.28.0, pinned in `model/Makefile`) and Docker for
`cd model && make lint`'s ephemeral shadow Postgres. Phases 3 and 4 each carry an integration task
that needs a real Postgres reachable through `DATABASE_URL`.

## Overview

Five phases, strictly sequential. Phase boundaries are drawn so each one leaves the module compiling
and its tests passing.

**Phase 1 — Multi-Key Storage Model Layer**
([`phases/key-store-model.md`](./phases/key-store-model.md)). Establishes the persistence contract
every later phase codes against: the replacement `field_crypto_keys` table, its seven queries, and
the narrow blob-only write-back queries re-encrypt-on-read needs.

- `001-replace-field-crypto-keys-schema` — rewrite migration `0017` in place with the multi-key DDL,
  update the blob-format comments in `0010`/`0011`, and rewrite `model/queries/field_crypto_keys.sql`
  with all seven queries.
- `002-add-blob-rotation-write-back-queries` — add the two `:execrows` compare-and-swap updates for
  `natural_persons.ssn` and `corporations.ein`. **Parallel-eligible with 001** — different files,
  different tables.
- `003-regenerate-model-db-and-querier-stubs` — `make gen`, commit `model/db/`, and repair all six
  in-repo `coredb.Querier` stub implementations. Depends on both 001 and 002.

**Phase 2 — Versioned Multi-Key Cipher**
([`phases/versioned-cipher.md`](./phases/versioned-cipher.md)). The security-critical centre: the
version-tagged, AAD-bound blob format and a reloadable keyring.

- `001-multi-key-cipher-core` — rewrite `api/internal/fieldcrypto` (key types, `keySet`, wire format,
  `Encrypt`/`Decrypt`/`DecryptWithRotation`/`Reload`/`BlobVersion`, bootstrap, staleness handling,
  package doc, all three test files), plus the mechanical `ctx` threading that keeps the api module
  building.
- `002-facade-key-store-adapter` — give `api/fieldcrypto` its real `coredb`-typed `FieldKeyQuerier`
  and the adapter that keeps the internal package model-free and the manifest's `cipher` block
  unchanged. Depends on 001.

**Phase 3 — Re-Encrypt-On-Read Call Sites**
([`phases/rotate-on-read.md`](./phases/rotate-on-read.md)). The user-visible half: reading a value
stored under a retired key leaves it re-encrypted under the active key.

- `001-rotating-cipher-helper` — the new `api/service/rotating_cipher.go`, which owns persistence,
  the write-back transaction, and the whole failure policy in one four-branch core.
- `002-wire-decrypt-call-sites` — convert the six decrypt sites and four encrypt sites, change three
  service field types and one variadic parameter, and compose the wrapper inside `service.New`
  without changing its exported signature. Depends on 001.
- `003-rotate-on-read-integration-test` — prove the headline success criterion against a real
  database, across both encrypted columns, including grace expiry and the compromised-key branch.
  Depends on 002.

**Phase 4 — Admin Key-Rotation API**
([`phases/admin-rotation-api.md`](./phases/admin-rotation-api.md)). Without this the machinery is
inert: nothing can create a second key version. HTTP only — mod-core produces no binary, so a CLI
would mean introducing its first executable and a second authorization story.

- `001-field-crypto-key-handler` — `FieldCryptoKeyHandler`, four wildcard-grant-admin-only routes,
  the rotation transaction, post-commit `Reload`, status mapping, and observer dispatch.
- `002-manifest-and-openapi-routes` — the `coreFieldCryptoKeyHandler` service block, its route entry,
  and the four documented routes. **Parallel-eligible with 001** — neither file participates in the
  Go build and the constructor signature is fixed by the design.
- `003-rotation-endpoint-integration-test` — the concurrency claim: two simultaneous rotations give
  one 201 and one 409, never two active keys. Depends on 001.

**Phase 5 — Documentation Updates.** One task, `001-update-architecture-docs`, bringing `AGENTS.md`
and the other in-repo reference docs in line with the delivered design — in particular the
operator-visible `CORE_FIELD_KEY_HEX` precedence change, the fact that the database now holds every
key version, and the laziness of rotation.

## Clarified requirements

### What must change

1. **Versioned ciphertext.** Every stored blob is `version(4) || nonce(12) || ciphertext || tag(16)`,
   with the 4 version bytes bound as AEAD additional authenticated data. New, breaking on-disk
   format.
2. **Multi-key cipher.** One active key encrypts everything new; an indefinite number of retired keys
   decrypt only and are never selected for encryption. The key set is reloadable at runtime.
3. **Re-encrypt on read.** A read of a non-active-version blob transparently re-encrypts it under the
   active key and persists the replacement, via a compare-and-swap update in its own short
   transaction.
4. **Per-key failure policy.** A standard rotation's retired key tolerates a failed write-back (log,
   return the plaintext); a key flagged compromised fails the read whenever the replacement cannot be
   durably persisted and the compromised ciphertext is not verifiably already gone.
5. **Replacement key storage.** `field_crypto_keys` is redesigned outright — versioned rows, a
   partial unique index enforcing one active key, an optional absolute grace deadline, and a
   compromise flag — by editing migration `0017` in place.
6. **An admin rotation API.** Four HTTP routes on a new handler, authorized as `manage` with a nil
   target (wildcard-grant admins only), supporting server-side key generation and operator-supplied
   `key_hex`, standard and compromised rotation, after-the-fact compromise flagging, and grace-window
   adjustment.

### What must not change, and what must not be built

- **No backfill or compat path.** Nothing is deployed on the current untagged format. Old blobs are
  not read, detected, or converted; no dual-format decode path is built. Existing databases are
  regenerated from scratch after this lands.
- **No files outside mod-core.** Not app-mfmanager, not app-mfdemo, not app-mftodo, not mod-users, and
  not `docs/mf-standards/` — which is a **git submodule** pointing at the separate
  `docs-mf-standards` repository, and is therefore cross-repo despite sitting under mod-core's
  `docs/`.
- **`moduleforge.module.yaml`'s `cipher` service block is untouched.** The façade adapter exists
  precisely so the constructor name, arg sources, and error return stay identical. Phase 4 adds a
  *separate*, additive handler service block; it does not modify this one.
- **`service.New`'s exported signature is fixed.** Every composing app calls it verbatim from
  generated code.
- **AAD row binding stays out of scope.** The version prefix is now bound as AAD, but binding
  ciphertext to a row id remains a separate, acknowledged gap — the package doc must narrow its
  existing claim to "no *row identity* is bound" rather than delete it.
- **Transport-level protection for the admin routes is out of scope.** App-level authz is what this
  plan delivers; a separate admin listener, ingress allow-list, or mTLS is a deployment concern,
  deferred below.
- **Followup `MnvB` (zeroing key byte slices) is not required.** Phase 2 rewrites
  `fromPersistedOrGenerated`, the exact function it concerns, so its task doc notes it as a natural
  incidental opportunity. It is not a blocking requirement and not an acceptance criterion.

### Success criteria

- A `Cipher` holding one active key and one or more retired keys decrypts blobs written under any of
  them, and encrypts new values only under the active key.
- Reading a value whose blob carries a non-active version leaves the stored blob re-encrypted under
  the active version, and a subsequent read finds it already current.
- A blob whose version matches no loaded key — unknown, or past its grace deadline — fails loudly
  rather than returning empty or wrong plaintext.
- A rotation performed through the admin endpoint retires exactly one key and activates exactly one
  replacement, and two concurrent rotations never leave two active keys.
- A read that cannot persist a re-encryption away from a **compromised** key fails; the same read
  under a standard-rotation key succeeds and logs.
- No response from any admin route contains key material.
- `cd model && make verify`, `cd model && make lint`, `cd api && make test`, `cd api && make lint`,
  and a whole-module `make build` all pass; `model/db/` is regenerated and committed.
- No code path in mod-core reads or writes the old untagged format.

## Settled decisions

All four originally-open decisions are resolved. Each note is the authoritative statement of its
decision; task documents consume them rather than restating the reasoning.

1. **Key-version tag wire format** →
   [`notes/key-version-wire-format.md`](./notes/key-version-wire-format.md). In-blob prefix, not a
   sibling column; bound as AEAD AAD.
2. **Key lifecycle and supply policy** →
   [`notes/key-lifecycle-policy.md`](./notes/key-lifecycle-policy.md). The database is the single
   source of truth for every key version. `CORE_FIELD_KEY_HEX` is a first-boot-only bootstrap seed
   and fails construction loudly on a later mismatch. Rotation is triggered by a new admin endpoint.
3. **Re-encrypt-on-read API shape and failure policy** →
   [`notes/rotation-api-shape.md`](./notes/rotation-api-shape.md). Per-key policy (standard grace
   period vs. compromised fail-fast); hybrid wiring — a pure rotation-aware decrypt in `fieldcrypto`
   plus a column-agnostic persistence helper in `api/service`; the full admin API design.
4. **Replacement key-table schema and migration mechanics** →
   [`notes/key-store-schema-design.md`](./notes/key-store-schema-design.md). Full replacement DDL,
   integer version via `GENERATED ALWAYS AS IDENTITY`, partial unique index for the one-active
   invariant and first-boot convergence, absolute `decryptable_until`, `compromised_at`, and an
   in-place edit of migration `0017` with no `0018`.

Background for all four is in
[`notes/fieldcrypto-current-state.md`](./notes/fieldcrypto-current-state.md) and
[`notes/rotation-on-read-call-sites.md`](./notes/rotation-on-read-call-sites.md).

## Deferred and flagged

Cross-repo and out-of-scope follow-ons this plan deliberately does not perform. Each needs to be
filed as a separate followup; none may become a task here.

### Compile breaks in other repositories — highest severity

**Deleting `fieldcrypto.NewFromEnv()` breaks compilation, not just documentation**, in three
composition roots outside this repository: `app-mfmanager/cmd/server/main.go:127`,
`app-mfdemo/cmd/server/main.go:112`, and `mod-users/api/cmd/server/main.go:264`. Each calls
`NewFromEnv()` directly even though mod-core's manifest declares `NewFromEnvOrGenerate`, and each
must move to `NewFromEnvOrGenerate(ctx, coredb.New(pool))` when it bumps its mod-core pin. Because
there is no backfill path, each must recreate its database at the same time. This is deliberately
recorded as a distinct, higher-severity item, separate from the documentation-staleness entries
below: those go stale silently, this one fails the build.

### Documentation and deployment follow-ons

- **app-mfmanager `deploy/README.md`, "Secret generation, loss, and rotation"** *(explicitly
  requested by the user)*. That section states `CORE_FIELD_KEY_HEX` is "irrecoverable-if-lost and
  unrotatable-if-leaked", quotes fieldcrypto's package doc on rotation being out of scope, and tells
  the operator that retiring a leaked key "would first require building that re-encrypt-on-read
  capability in mod-core and then running a data migration". Once this plan lands the
  "unrotatable-if-leaked" half becomes false and must be replaced with the real procedure. It must
  cover: the new admin rotation routes; that `CORE_FIELD_KEY_HEX` is now a first-boot-only seed that
  **fails startup loudly** if it later disagrees with the active key; that **rotation is lazy**, so a
  retired key can only be discarded once every stored blob has actually been read and re-encrypted,
  never after a fixed interval; that after a **compromised** rotation the fleet should be restarted
  rather than relying on the key-set TTL, if a hard guarantee is wanted that no process is still
  encrypting under the leaked key; and that a **database dump now carries every key version**, where
  an env-only operator previously kept key material out of the database entirely. The
  "irrecoverable-if-lost" half stays true and must not be softened. That section's separate "no AAD
  is bound to ciphertext" bullet also needs rescoping, since the version prefix is now AAD-bound
  while row identity still is not.
- **app-mfmanager's other `CORE_FIELD_KEY_HEX` references** — `deploy/prod/README.md`,
  `pre-prod-test.md`, and `docs/architecture/tier2-permissions-boundary-security-design-gcp.md` all
  name the variable and may carry the same unrotatable framing.
- **`docs-mf-standards` repository** (reached through mod-core's `docs/mf-standards/` **submodule**,
  so cross-repo despite the in-tree path). `manifest-spec.md` documents the `cipher` service's
  `NewFromEnvOrGenerate` constructor, and `architecture/secret-durability-design.md` documents the
  single-row `field_crypto_keys` table and the single-key constructor design in three places.
- **mod-users `.env.example:145`** ships a sample `CORE_FIELD_KEY_HEX`. Its accompanying comment
  needs to state that the variable is now a first-boot bootstrap seed only.
- **Composing apps' generated composition roots.** Phase 4 adds a manifest service and route entry
  (additively; the `cipher` block is untouched), so a composing app must regenerate against the new
  mod-core pin to expose the rotation routes. Because there is no backfill path, each app's database
  must also be recreated from scratch — coordinate the pin bump with that.
- **mfgen** — confirm the new `coreFieldCryptoKeyHandler` service block and its `register:` route
  entry render correctly.

### Deliberately deferred capability

- **Transport-level protection for the rotation routes** — a separate admin listener, ingress
  allow-list, or mTLS. Resolved as out of scope: app-level wildcard-grant-admin authz is what this
  plan specifies; network-level restriction is a deployment concern for the app-mfmanager deploy
  docs.
- **Envelope-encrypting `key_bytes` under a KEK supplied by the environment.** With the database as
  single source of truth, a dump compromises data encrypted under *every* key version retroactively.
  This is an accepted consequence of a settled decision, and the natural mitigation is its own piece
  of work.
- **A rotation-progress metric.** The read-path write-back's outcome (`persisted` / `stale` /
  `error`) is currently only a structured log line; a counter is the right observability primitive,
  but mod-core has no metrics facility today.
- **Push-based key-set invalidation** (`LISTEN`/`NOTIFY`) and a configurable key-set TTL. The
  reload-on-unknown-version plus TTL-on-encrypt combination is sufficient; the constant is fine until
  an operator asks otherwise.
- **An `mfmanager`-side CLI wrapper** over the rotation routes. Reasonable, needs nothing from
  mod-core, and is explicitly not mod-core's to build — mod-core produces no binary.
- **A distinct grantable `rotate` operation.** Every admin route authorizes as `manage` because
  operation slugs are registered in mod-authz, another repository. Registering `rotate` there and
  changing one string here is a clean, isolated follow-on if operations wants the granularity.
- **`LegalEntityService` is dead code.** It has no constructor, is absent from the `service.Services`
  aggregate, and its `cipher` field is unexported, so two of the six decrypt call sites are provably
  unreachable. Phase 3 updates them for consistency; deciding whether the service should be
  constructed or deleted is separate work.
