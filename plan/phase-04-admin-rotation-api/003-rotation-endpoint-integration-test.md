# Rotation Endpoint Concurrency Integration Test

## Purpose and scope

Prove against a real Postgres database that the rotation endpoint's concurrency claim holds: two
simultaneous rotations produce one 201 and one 409, and the table never holds two active keys. The
whole safety argument for skipping `pg_advisory_xact_lock` rests on the partial unique index and the
row-lock re-evaluation behavior actually behaving as designed, which only a real database can
demonstrate.

Files this task owns:

- A new build-tagged integration test under `api/httpapi/` (e.g.
  `api/httpapi/field_crypto_keys_integration_test.go`).

Depends on [`001-field-crypto-key-handler.md`](./001-field-crypto-key-handler.md). No standard skill
covers this work.

## Requirements

1. **Follow the existing integration-test convention** — an `integration` build tag and a real
   database reached through the standard `DATABASE_URL` — matching
   `api/authz/setup/grant_table_integration_test.go` and
   `api/internal/fieldcrypto/generate_integration_test.go` rather than inventing a new harness.
2. **The headline case: two simultaneous rotations.** Fire two rotation requests concurrently against
   the same handler and database and assert:
   - exactly one returns 201 and exactly one returns 409 — never two 201s, never two 409s, never a
     500;
   - after both complete, `SELECT count(*) FROM field_crypto_keys WHERE retired_at IS NULL` is
     exactly 1;
   - the version count increased by exactly one (the loser's `INSERT` never ran).
3. **Cover the sequential happy path too**, as the baseline the concurrency case is contrasted
   against: a single rotation returns 201, retires the previously-active version with the expected
   `decryptable_until`, and creates a new active version.
4. **Cover the unique-`key_bytes` rejection.** Rotate with an explicit `key_hex` equal to a key
   already on file and assert a 409 rather than a 500 — this is the guard that stops an operator
   re-introducing a key previously retired as compromised.
5. **Cover the compromised rotation's effect on the row**, end to end: `compromised: true` stamps
   `compromised_at` on the retired row and leaves `decryptable_until` NULL, and the same request
   carrying `grace_period_days` is rejected with 400 before any transaction runs.
6. **Leave the database clean and the test idempotent.** Key rows accumulate and the one-active-key
   invariant makes a careless second run fail confusingly; delete or roll back everything the test
   creates so a repeat run passes.
7. **Do not weaken the default test path.** With the build tag absent, `cd api && make test` must
   behave exactly as before — no new required environment variable and no skipped-test noise.

## Validation

- `cd api && make test` passes unchanged (the new file is excluded by its build tag).
- `cd api && go vet -tags integration ./httpapi/...` compiles the new test.
- `cd api && go test -tags integration ./httpapi/...` passes against a real database. If no database
  is available in the execution environment, say so explicitly in the task report rather than
  reporting a pass — a compiled-but-never-run integration test does not satisfy this task.
- The concurrency case is genuinely concurrent (goroutines released together, not run in sequence);
  a reviewer should be able to see the synchronization in the test body.
- Running the integration suite twice in a row against the same database passes both times.
- `cd api && make lint` passes.
- `git diff --stat` shows exactly one new file.

## Assumptions

- Task 001 has landed and the handler is constructible in-process against a real pool, in the style
  the existing httpapi tests already use for their fakes — substituting a real `txhelper.DB` and a
  real `coredb.Querier` for the fakes.
- An `authz.Authorizer` test double that grants wildcard `manage` is available or trivially written;
  this test is about concurrency and persistence, not about the authorization gate, which task 001's
  unit tests already cover.

## References

- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-task-requirements) —
  Phase 3 requirement 7, the concurrency claim named explicitly as a test requirement.
- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#rotation-transaction) —
  why two concurrent rotations resolve safely without extra locking, and why the failure mode is "one
  wins, the other errors and is retried" rather than two active keys.
- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#routes) — the status mapping the
  409 and 400 assertions are checking.

## Status

**Outcome:** succeeded — 2026-08-13.

Added `api/httpapi/field_crypto_keys_integration_test.go` (the only new file; `git diff --stat`
confirms it). Ran against the real `users-module-postgres` container (`CORE_DEV_PG_HOST=192.168.1.153`
— plain `localhost` on this machine resolves to an unrelated host-native Postgres with no `users`
role, confirmed via `psql`), migrated from scratch via goose into a dedicated shadow database
(`core_field_crypto_rotation_verify_dev`), following the TestMain / prerequisite-check /
host-resolution / shadow-DB-reset pattern established by `api/authz/setup/grant_table_integration_test.go`,
`api/internal/fieldcrypto/generate_integration_test.go`, and `api/service/rotating_cipher_integration_test.go`.

Validation:

- `cd api && make test` — passes unchanged (new file excluded by the `integration` build tag; no new
  required env var, no skipped-test noise).
- `cd api && go vet -tags integration ./httpapi/...` — clean.
- `cd api && gofmt -l api` — clean (no output).
- `CORE_DEV_PG_HOST=192.168.1.153 go test -tags=integration -p 1 -v ./httpapi/...` — all tests pass,
  including the five new integration tests and every pre-existing unit test in the package (same test
  binary). Ran twice in a row with `-count=1` against the same database (both passed) and once under
  `-race` restricted to the concurrency test (passed clean).
- `cd api && make lint` — passes (`go vet ./...` + gofmt check).
- `git diff --stat` — exactly one new file.

Notable implementation decision (see `decisions_made` in the task report for full detail): the
headline concurrency test could not rely on a bare goroutine-plus-channel release to force a genuine
race — a first attempt at that shape produced two clean `201`s on every run, because each rotation's
full HTTP-to-commit round trip against a local Postgres is sub-millisecond, so the two requests simply
ran start-to-finish one after another with no actual lock contention. The test instead holds the exact
row lock `RetireActiveFieldCryptoKey`'s `UPDATE` needs in a separate "gatekeeper" transaction, confirms
(by polling `pg_stat_activity`) that both real rotation requests are genuinely queued as waiters on
that lock, then releases it — reproducing the same DB-level contention two truly simultaneous
rotations would hit, deterministically. Verified stable across three repeated runs
(`-count=3`) plus a `-race` run.

Assumptions applied: task 001 (`001-field-crypto-key-handler.md`) has landed and
`FieldCryptoKeyHandler` is constructible in-process against a real pool — confirmed directly
(`api/httpapi/field_crypto_keys.go` exists with all four routes). A locally-defined `wildcardAuthorizer`
(grants every operation unconditionally) stands in for the `authz.Authorizer` test double the task
doc's Assumptions call for; the 401/403 authorization-gate paths are already covered by task 001's
own unit tests (`field_crypto_keys_test.go`), so this suite does not re-cover them.

Files touched:

- `api/httpapi/field_crypto_keys_integration_test.go` (new)
- `plan/phase-04-admin-rotation-api/003-rotation-endpoint-integration-test.md` (this Status section)
