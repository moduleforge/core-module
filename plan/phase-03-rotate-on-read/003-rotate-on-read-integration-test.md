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
