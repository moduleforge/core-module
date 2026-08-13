# Rotate On Read

## Purpose and scope

Phase summary for wiring transparent re-encryption into mod-core's read paths. The wiring option and
the failure-policy branch structure are settled in
[`rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-3-surface--apiservicerotating_ciphergo);
this phase implements them.

## Goals

Deliver the user-visible half of the feature: reading a value stored under a retired key leaves it
re-encrypted under the active key. This is the behavior fieldcrypto's package doc has promised at the
call site since it was written, with no call site ever implementing it.

There is no single choke point — `Decrypt` is a pure function with no database handle and no
knowledge of the row it is reading, and mod-core has six decrypt sites across four files in
`api/service/` spanning two encrypted columns. The settled shape is a hybrid: the pure
rotation-aware primitive lives in `fieldcrypto` (Phase 2), and a new column-agnostic
`RotatingCipher` in `api/service/` owns persistence — its own pool-backed transaction with a
`lock_timeout`, a compare-and-swap update guarded on the exact blob that was read, and the four-way
policy branch that adjudicates a failed write-back. The six call sites become one-line calls with no
policy logic of their own.

The write-back deliberately dispatches **no observer**: a re-encryption changes stored bytes and
nothing else, and firing the `update` observer would attribute a mutation to a reader who may hold
only `read` permission. Its record is a structured log line.

`service.New`'s signature stays fixed — it already receives both the cipher and a `txhelper.DB`, and
composes the `RotatingCipher` internally. That is load-bearing: every composing app calls
`coreservice.New(...)` verbatim from generated code, so no app changes a line for this phase.

## Inputs

- Phase 2's `DecryptWithRotation`, `Rotation` (with `MustPersist`), `BlobVersion`, and the
  ctx-carrying `Encrypt`/`Decrypt`.
- Phase 1's `UpdateNaturalPersonSSNBlob` / `UpdateCorporationEINBlob` compare-and-swap queries and
  the existing `GetNaturalPersonByEntityID` / `GetCorporationByEntityID` reads the stale-CAS
  verification path reuses.
- The settled per-key failure policy in
  [`rotation-api-shape.md`](../notes/rotation-api-shape.md#interpretation-for-planning): a standard
  rotation tolerates a failed write-back (log and return plaintext); a compromised key fails the
  read unless the compromised ciphertext is verifiably already gone.
- The six decrypt sites: `profile.go`'s `ResolveProfileByEntityID` (both branches),
  `legal_entity.go`'s `GetTaxID` (both branches), `natural_person.go`'s `GetDecryptedSSN`, and
  `corporation.go`'s `GetDecryptedEIN` — plus the four `Encrypt` sites in `natural_person.go` and
  `corporation.go`.

## Outputs

- `api/service/rotating_cipher.go`: `RotatingCipher`, `NewRotatingCipher`, the `blobColumn`
  descriptor with `ssnColumn` / `einColumn`, `decryptRotating`, `writeBack`, and `verifyStale`.
- Every mod-core read path that decrypts a field value re-encrypts and persists it under the active
  key when its version is stale, with one consistent policy implementation shared by all six sites.
- Service field types, `ResolveProfileByEntityID`'s variadic parameter, and `api/service/mock_test.go`
  moved to `*RotatingCipher`, with `service.New`'s exported signature unchanged.
- Unit tests for the four cases that would not otherwise be written: a compromised-key read whose CAS
  loses (must fail), a compromised-key read whose CAS loses to an already-rotated blob (must
  succeed), a standard-key read whose write-back errors (must succeed and log), and a write-back
  attempted with a nil write handle.
- An integration test proving the plan's headline success criterion end to end: a stale-version read
  returns correct plaintext and leaves the stored blob re-encrypted, and a second read finds it
  already current.
- `cd api && make test`, `cd api && make lint`, and a whole-module `make build` passing.
