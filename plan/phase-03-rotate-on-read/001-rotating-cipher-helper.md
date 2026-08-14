# Rotating Cipher Write-Back Helper

## Purpose and scope

Build `api/service/rotating_cipher.go`: the column-agnostic helper that pairs the multi-key cipher
with a write handle, persists a re-encrypted blob, and adjudicates the per-key failure policy in one
place. This is the half of the hybrid wiring that owns persistence; the pure rotation primitive
already exists in `fieldcrypto` from Phase 2.

Files this task owns:

- `api/service/rotating_cipher.go` — new.
- `api/service/rotating_cipher_test.go` — new.

This task does **not** change any existing service or call site;
[`002-wire-decrypt-call-sites.md`](./002-wire-decrypt-call-sites.md) does. No standard skill covers
this work.

## Requirements

1. **`RotatingCipher` and its constructor**, per
   [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-3-surface--apiservicerotating_ciphergo):
   a `*fieldcrypto.Cipher`, a `txhelper.DB` write handle (nil disables write-back, which is what
   tests use), a `func(pgx.Tx) coredb.Querier` defaulting to `coredb.New`, and a `*slog.Logger`
   defaulting to `slog.Default()`. `NewRotatingCipher(c, db, log)` fills the defaults.
2. **`Encrypt(ctx, plaintext)`** forwards straight to the underlying cipher — write paths need no
   rotation.
3. **`DecryptSSN(ctx, entityID, blob)` and `DecryptEIN(ctx, entityID, blob)`** are two-line wrappers
   over a shared `decryptRotating(ctx, col, entityID, blob)` core. Both return `""` for an absent
   value, exactly as `Cipher.Decrypt` does today.
4. **The `blobColumn` descriptor** carries everything the core needs to know about one encrypted
   column: a `name` used only in log fields and error text, a `swap` closure calling the Phase 1
   compare-and-swap update, and a `current` closure re-reading the stored blob via the existing
   `GetNaturalPersonByEntityID` / `GetCorporationByEntityID` queries. Declare `ssnColumn` and
   `einColumn` as package-level values. Adding a third encrypted column later must be one more struct
   literal and one more two-line method — keep it that way.
5. **`decryptRotating` — the four-branch policy core**, reproduced from the design note. Its branch
   structure *is* the failure policy and must not be restructured:
   - fast path — an error, or `!rot.Needed()`, returns immediately;
   - write-back persisted — log at `Debug`, return the plaintext;
   - write-back failed and `rot.MustPersist` — return an error naming the column and the source
     version, wrapping the write error; the read fails;
   - write-back failed otherwise — log at `Warn`, return the plaintext. A standard rotation inside
     its grace window is opportunistic.
6. **`writeBack` runs in its own short read-write transaction on the helper's own pool-backed
   handle** — never on the caller's `coredb.Querier`, never in a goroutine. The reasoning is
   decisive and must be preserved in a code comment: a failed statement inside a caller-owned
   transaction aborts that transaction in Postgres, so "tolerate the failure and return the
   plaintext" would leave the caller holding a doomed transaction — precisely on the read-only
   replica and read-only-transaction cases that motivated the policy in the first place.
7. **`SET LOCAL lock_timeout = '250ms'` as the transaction's first statement.** A caller-owned
   transaction may already hold a row lock on the same row; that wait is not a lock cycle, so
   Postgres's deadlock detector will not break it. The timeout converts a hang into an ordinary
   write-back failure the policy branch already knows how to adjudicate. This is the reason the
   write-back uses an explicit transaction rather than a bare autocommit statement.
8. **`verifyStale` — the compromised-key refinement.** A CAS matching zero rows means the stored blob
   changed underneath the reader. Under a standard key that is a benign skip with no extra query
   (log `outcome=stale`). Under a compromised key, re-read the current blob **inside the same
   transaction** and decide:
   - empty stored value ⇒ nothing remains under the old key ⇒ treat as persisted;
   - `fieldcrypto.BlobVersion(cur) != rot.FromVersion` ⇒ someone else already re-encrypted it ⇒
     treat as persisted;
   - still carrying `FromVersion` ⇒ genuinely unpersisted ⇒ error, and the read fails.

   Document the two properties that make this sound: under READ COMMITTED a zero-row CAS always means
   a committed change rather than a lock miss, and a stored blob still carrying `FromVersion` after a
   committed change is not reachable by any legitimate writer, since every write path encrypts under
   the active key. Also document the one conservative false negative — a caller at `REPEATABLE READ`
   or `SERIALIZABLE` sees its own snapshot and reports a genuine race as unpersisted, failing the
   read, which is the safe direction.
9. **No observer dispatch on the write-back.** A re-encryption changes stored bytes and nothing else;
   the actor may hold only `read` permission, and a rotation sweep would put one audit row in the log
   per first read of every un-rotated row. Its record is a structured log line carrying
   `event=fieldcrypto.rotate_on_read`, the column name, entity id, from/to versions, whether the
   source key was compromised, and an outcome of `persisted`, `stale`, or `error`, at `Debug`,
   `Warn`, and `Error` respectively. State the "no observer" decision in a code comment so a future
   reader does not add one back.
10. **Rotation is never retried** inside the helper — a skipped rotation is retried by construction on
    the next read of that row. The write-back also is not atomic with the read and does not need to
    be; the CAS on the exact blob that was read is what makes it safe. Note both in comments.
11. **Tests**, covering at minimum the four cases the design flags as ones that would not be written
    by default, plus the happy path:
    - a stale-but-standard-key read whose CAS succeeds: plaintext correct, replacement persisted;
    - a compromised-key read whose CAS loses and whose stored blob still carries the old version:
      **must fail the read**;
    - a compromised-key read whose CAS loses to an already-rotated blob: **must succeed**;
    - a standard-key read whose write-back errors outright: must succeed and log at `Warn`;
    - a write-back attempted with a nil write handle: standard key succeeds, compromised key fails.

## Validation

- `cd api && make build`, `cd api && make test`, and `cd api && make lint` all pass.
- `cd api && go test ./service/... -run RotatingCipher -v` shows all five test cases above passing.
- `grep -n "Observe" api/service/rotating_cipher.go` returns nothing — the write-back dispatches no
  observer.
- `grep -n "lock_timeout" api/service/rotating_cipher.go` shows the `SET LOCAL` statement present.
- No existing file under `api/service/` is modified by this task — `git diff --stat` shows exactly
  two new files.
- The compromised-key failure path returns an error that names the column and the source key version
  (assert in a test).

## Metadata

architectural_impact: true

## Assumptions

- Phase 1's `UpdateNaturalPersonSSNBlob` / `UpdateCorporationEINBlob` exist as `:execrows` queries
  returning an affected-row count, and Phase 2 exports `DecryptWithRotation`, `Rotation` (with
  `Needed()` and `MustPersist`), and `BlobVersion`.
- `txhelper` provides a `Run(ctx, db, func(ctx, tx) error)`-shaped helper and a `DB` interface; check
  the actual names in `api/txhelper/` and use them rather than the illustrative ones in the design
  note.
- `natural_persons.updated_at` and `corporations.updated_at` will now be bumped by a pure read that
  happens to rotate. This is accepted, not worked around: those columns are read by nothing in
  mod-core, and the only client-observable timestamp is `entities.updated_at`, which the write-back
  does not touch. Carry one sentence to that effect in the file's doc comment.

## References

- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-3-surface--apiservicerotating_ciphergo)
  — the full type and function shapes, the `decryptRotating` core verbatim, the write-back
  transaction, and the rejected alternatives. Read
  [Where the write-back runs](../notes/rotation-api-shape.md#where-the-write-back-runs-and-why-not-on-the-callers-querier),
  [CAS returning zero rows](../notes/rotation-api-shape.md#cas-returning-zero-rows--resolving-schema-open-question-6),
  and [Observers](../notes/rotation-api-shape.md#observers-none-on-the-write-back) in full.
- [`../notes/rotation-on-read-call-sites.md`](../notes/rotation-on-read-call-sites.md) — the five
  structural obstacles this helper resolves.
- `api/httpapi/apps.go` — the in-repo reference for the `txhelper.Run` + tx-scoped querier pattern.

## Checkpoint hints

- After the types, the `blobColumn` descriptors, and `Encrypt`/`DecryptSSN`/`DecryptEIN` compile.
- After `decryptRotating` and `writeBack` are complete with the happy-path test passing.
- After `verifyStale` and the two compromised-key tests.

## Status

**succeeded** — 2026-08-13.

Affected files (both new; no existing file under `api/service/` was modified):

- `api/service/rotating_cipher.go`
- `api/service/rotating_cipher_test.go`

Validation: `cd api && make build`, `make test`, and `make lint` all pass;
`go test ./service/... -run RotatingCipher -v` passes (11 test functions, covering the five named
cases plus `Encrypt`, the no-rotation-needed fast path, an absent value, and both columns);
`go test -race ./service/ -run RotatingCipher` passes. `grep -n "Observe" api/service/rotating_cipher.go`
returns nothing; `grep -n "lock_timeout"` shows the `SET LOCAL` statement; `git diff --stat` against the
task's base commit shows exactly the two new files.

Assumptions confirmed against the tree rather than taken on trust: Phase 1's `UpdateNaturalPersonSSNBlob`
/ `UpdateCorporationEINBlob` exist as `:execrows` queries returning `(int64, error)`; Phase 2 exports
`DecryptWithRotation`, `Rotation.Needed()`/`MustPersist`, and `BlobVersion`; `txhelper` provides
`Run(ctx, db, func(ctx, tx) error) error` and the `DB` interface, so the design note's illustrative names
matched the real ones. The `updated_at` sentence is carried in the file's doc comment.

Decisions made inside the task's scope:

- `writeBack` returns `persisted = false` whenever `txhelper.Run` returns an error, rather than
  returning the flag the transaction body set. A commit that fails after a matched compare-and-swap
  would otherwise report a rotation as durably stored when it was rolled back — which the
  compromised-key branch would then treat as success. `decryptRotating`'s four branches are unchanged.
- A lost compare-and-swap under a standard key returns a sentinel error (`errStaleCAS`) from the
  transaction body rather than a nil error with `persisted` false, so the tolerated-miss log line always
  carries a cause and the `MustPersist` branch can never wrap a nil error.
- `outcome=stale` is the single label for every tolerated miss (lost CAS, read-only replica, permission
  error, absent write handle), per the design's three-value outcome vocabulary, with the underlying
  cause carried in the log line's `error` field.
- Tests build a store-backed multi-key `Cipher` from a static in-test `fieldcrypto.KeyStore` (version 1
  retired, optionally compromised; version 2 active), since `NewFromKey` is store-less and never reports
  a rotation. They reuse `mock_test.go`'s `fakeDB`/`fakeTx` and add a `coredb.Querier` stub that
  implements only the four queries the write-back can issue.
