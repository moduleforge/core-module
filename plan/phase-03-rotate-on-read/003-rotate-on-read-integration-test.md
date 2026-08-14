# Rotate-On-Read Integration Test

## Purpose and scope

Prove the plan's headline success criterion against a real Postgres database: reading a value whose
blob carries a non-active key version returns the correct plaintext **and** leaves the stored blob
re-encrypted under the active key, and a second read finds it already current. Unit tests with fakes
cannot establish this — the compare-and-swap, the separate write-back transaction, and the row-level
persistence are exactly the parts a fake elides.

Files this task owns:

- A new build-tagged integration test under `api/service/` (e.g.
  `api/service/rotating_cipher_integration_test.go`).

Depends on [`002-wire-decrypt-call-sites.md`](./002-wire-decrypt-call-sites.md). No standard skill
covers this work.

## Requirements

1. **Follow the existing integration-test convention.** `api/internal/fieldcrypto/generate_integration_test.go`
   and `api/authz/setup/grant_table_integration_test.go` are the two in-repo precedents: an
   `integration` build tag and a real database reached through the standard `DATABASE_URL`. Match
   their tagging, skip-when-unconfigured behavior, and setup/teardown style rather than inventing a
   new harness.
2. **Exercise a real rotation end to end**, in this order:
   - bootstrap a cipher against the real `field_crypto_keys` table so it holds version 1;
   - write an encrypted SSN (and separately an EIN) through the service layer, so the stored blob
     genuinely carries version 1;
   - perform a rotation directly through the Phase 1 queries (`RetireActiveFieldCryptoKey` then
     `InsertActiveFieldCryptoKey` in one transaction — the mandatory order), so a version 2 exists;
   - reload the cipher;
   - read through the service method and assert the returned plaintext is correct;
   - re-read the stored blob straight from the database and assert its 4-byte version prefix now
     decodes to version 2 (use the exported `BlobVersion` helper);
   - read a second time and assert `Rotation` was not needed — no further write occurred. Detect this
     by capturing the row's `updated_at` (or the blob bytes) before and after and asserting it is
     unchanged by the second read.
3. **Cover both encrypted columns** — `natural_persons.ssn` and `corporations.ein` — since each has
   its own `blobColumn` descriptor and its own CAS query. A table-driven test over the two columns is
   preferable to two hand-copied bodies.
4. **Cover the grace-window expiry path.** Retire version 1 with a `decryptable_until` already in the
   past, reload, and assert a read of a version-1 blob fails loudly rather than returning empty or
   wrong plaintext. This is the success criterion that a blob whose version matches no loaded key
   fails visibly.
5. **Cover the compromised-key read.** Retire version 1 as compromised, then read a version-1 blob:
   the read must succeed with a working write handle (the write-back persists), and must fail when
   the write handle is nil or the write-back cannot commit. This is the one policy branch whose
   security value depends on it actually behaving differently from the standard case.
6. **Leave the database clean.** Roll back or delete the rows and key versions the test creates so a
   repeated run against a developer database is idempotent. Key rows in particular accumulate, and
   the one-active-key invariant will make a careless second run fail confusingly.
7. **Do not weaken the default test path.** With the build tag absent, `cd api && make test` must
   behave exactly as before — no new required environment variable, no skipped-test noise in the
   default run.

## Validation

- `cd api && make test` passes unchanged (the new file is excluded by its build tag).
- `cd api && go vet -tags integration ./service/...` compiles the new test.
- `cd api && go test -tags integration ./service/...` passes against a real database. If no database
  is available in the execution environment, say so explicitly in the task report rather than
  reporting a pass — a compiled-but-never-run integration test does not satisfy this task.
- `cd api && make lint` passes.
- Running the integration suite twice in a row against the same database passes both times
  (idempotent cleanup).
- `git diff --stat` shows exactly one new file.

## Assumptions

- Tasks 001 and 002 have landed, and Phase 1's key queries and CAS updates are available through
  `coredb`.
- The test can create `entities` and their `natural_persons` / `corporations` subtype rows through
  the existing service layer or query surface; reuse whatever fixture helper the existing integration
  tests already provide rather than writing a parallel one.
- A rotation performed directly through the queries (rather than through the Phase 4 HTTP endpoint,
  which does not exist yet) is sufficient and correct for this test's purpose.

## References

- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-task-requirements) —
  Phase 3 requirement 6, the test cases named as requirements because they would not be written by
  default.
- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#rotation-transaction) —
  the mandatory retire-then-insert ordering the test's rotation step must follow.
- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#grace-expiry-semantics)
  — expiry behavior, enforced in both SQL and Go.
- `api/internal/fieldcrypto/generate_integration_test.go` — the in-repo integration-test convention.

## Status

**Outcome: succeeded.** Implemented 2026-08-13.

Added `api/service/rotating_cipher_integration_test.go` (`//go:build integration`, package
`service_test`), following the `TestMain` / prerequisite-check / host-resolution / shadow-DB-reset
pattern of `api/internal/fieldcrypto/generate_integration_test.go` and
`api/authz/setup/grant_table_integration_test.go`, against its own shadow database
(`core_rotate_on_read_verify_dev`).

Three top-level tests:

- `TestRotateOnRead_StandardRotationPersistsAndIsIdempotent` — table-driven over
  `natural_persons.ssn` and `corporations.ein`: bootstraps a real cipher (version 1), writes through
  `RotatingCipher.Encrypt` into a seeded entity/legal_entity/natural_person-or-corporation row,
  rotates directly through `RetireActiveFieldCryptoKey` + `InsertActiveFieldCryptoKey` (mandatory
  order), reloads, reads through the service method and asserts the plaintext, re-reads the raw
  stored blob and asserts its version prefix decodes to the new active version, then reads a second
  time and asserts no further write occurred (`updated_at` unchanged, raw blob bytes unchanged).
- `TestRotateOnRead_GraceWindowExpiredFailsRead` — retires version 1 with a back-dated, already-past
  `decryptable_until` (same technique as `TestFieldKeyRealExpiredGraceWindowStopsDecrypting`), reloads,
  and asserts a read of the still-version-1 blob fails loudly rather than returning empty or wrong
  plaintext.
- `TestRotateOnRead_CompromisedKeyReadRequiresWorkingWriteHandle` — retires version 1 as compromised
  and exercises three independent rows/sub-cases: a working write handle persists the replacement and
  the read succeeds; a nil write handle fails the read; and a write-back that genuinely cannot commit
  (implemented by holding a real competing row lock in a separate uncommitted transaction, so the
  write-back's own `SET LOCAL lock_timeout = '250ms'` genuinely expires with SQLSTATE 55P03) also
  fails the read. All three assert the stored blob was left untouched on failure.

Validation, all executed:

- `cd api && make test` (`go test ./...`, no build tag) — passes unchanged; the new file is excluded
  by its build tag. All 13 packages `ok`.
- `cd api && go vet -tags integration ./service/...` — clean, no findings.
- `cd api && go test -tags=integration -p 1 -v ./service/...` — run against the real
  `users-module-postgres` container. **Database was available and the suite was actually executed**,
  not skipped: `CORE_DEV_PG_HOST=192.168.1.153` (verified via `psql`: plain `localhost` on this host
  resolves to a *different*, host-native Postgres with no `users` role — the same finding
  phase-2/task-001's integration test recorded; the container is reachable at the host's LAN IP).
  Every test in the package passed, including the three new tests and all prior unit/service tests.
- `cd api && make lint` (`go vet ./...` + `gofmt -l .`) — clean.
- Idempotency: ran the new tests twice in a row — once as two separate `go test` invocations (each
  triggering `TestMain`'s drop/recreate of the shadow database), and once with `-count=2` in a single
  process (no DB reset between the two `m.Run()` passes, so the second pass hit whatever the first
  pass's `t.Cleanup` left behind). Both passed both times.
- `git diff --stat` — exactly one new file (744 lines), no other changes.

Assumptions applied (from `## Assumptions` above): tasks 001/002 landed and Phase 1's key queries/CAS
updates are available through `coredb` (confirmed — `RetireActiveFieldCryptoKey`,
`InsertActiveFieldCryptoKey`, `UpdateNaturalPersonSSNBlob`, `UpdateCorporationEINBlob` all present and
used directly); entity/subtype rows are created via the existing query surface, reusing the
`seedNaturalPerson`/`seedCorporation` pattern from `grant_table_integration_test.go` (adapted to carry
a pre-encrypted blob rather than an empty one) rather than a parallel fixture helper; a rotation
performed directly through the Phase 1 queries (not the not-yet-existing Phase 4 HTTP endpoint) is
sufficient and correct for this test's purpose.

One decision beyond the task doc's explicit direction: for the compromised-key "write-back cannot
commit" sub-case, the task doc left the mechanism open. Chose a real competing row lock (a separate
uncommitted transaction holding `SELECT ... FOR UPDATE` on the target row) over a synthetic failure
path, so the write-back's own `SET LOCAL lock_timeout` is exercised for real rather than simulated —
this is the exact lock-ordering hazard `plan/notes/rotation-api-shape.md` documents as the reason that
timeout exists.

Files touched: `api/service/rotating_cipher_integration_test.go` (new).
