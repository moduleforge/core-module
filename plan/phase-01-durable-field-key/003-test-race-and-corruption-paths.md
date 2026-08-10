# Test Race And Corruption Paths

## Purpose and scope

Prove `NewFromEnvOrGenerate` (added in
[`002-implement-auto-generate-cipher.md`](./002-implement-auto-generate-cipher.md))
is correct on every branch — including the two branches that are hard to get
right by inspection alone: a concurrent first-boot race resolving to exactly
one winning key, and a corrupted/unreadable persisted key failing loudly
rather than being silently regenerated. Two layers: fast unit tests against a
fake `FieldKeyQuerier` for branch coverage, and a real-Postgres integration
test (following this repo's existing `//go:build integration` precedent) for
the properties only a real DB-level uniqueness constraint can actually prove.

No standard skill covers this; follow the [`## Procedure`](#procedure) below.

## Requirements

1. **Unit tests** in a new file, `api/internal/fieldcrypto/generate_test.go`
   (package `fieldcrypto_test`, alongside the existing
   `fieldcrypto_test.go`), using a small hand-written fake implementing
   `FieldKeyQuerier` (2 methods — no `stubQuerier`-style boilerplate needed,
   per `002`'s Requirement 1 design). Cover at least:
   - **Env-var-wins, DB never touched.** `CORE_FIELD_KEY_HEX` set to a valid
     key; the fake's `GetFieldCryptoKey`/`InsertFieldCryptoKeyIfAbsent`
     methods both call `t.Fatal` if invoked. Assert `NewFromEnvOrGenerate`
     returns successfully without ever calling the fake — proving the
     env-var short-circuit happens before any DB access, not merely that it
     wins some later comparison.
   - **Env-var set but invalid, DB never touched.** Same fake-panics-if-called
     setup, with `CORE_FIELD_KEY_HEX` set to a non-hex or wrong-length value.
     Assert an error is returned and, again, the fake is never called —
     invalid-but-present must behave like `NewFromEnv` today (error, no
     fallback to generation).
   - **Absent, first fetch finds nothing, generate-and-persist succeeds.**
     Fake's `GetFieldCryptoKey` returns `pgx.ErrNoRows`;
     `InsertFieldCryptoKeyIfAbsent` echoes back whatever `[]byte` it was
     called with. Assert success and that the returned `Cipher` round-trips
     `Encrypt`/`Decrypt` correctly (proves the generated 32-byte key is
     actually usable, not just non-nil).
   - **Absent, but insert loses the race.** Fake's `GetFieldCryptoKey`
     returns `pgx.ErrNoRows` on the first call; `InsertFieldCryptoKeyIfAbsent`
     returns `pgx.ErrNoRows` (simulating `ON CONFLICT DO NOTHING` skipping);
     a second `GetFieldCryptoKey` call (track call count in the fake) returns
     a distinct, fixed 32-byte "winner" key. Assert `NewFromEnvOrGenerate`
     returns a `Cipher` built from the *winner's* key, not from whatever
     random candidate it generated internally — the only way to observe this
     from outside is that the returned cipher's `Encrypt` output is decryptable
     by a second `Cipher` built directly via `NewFromKey(winnerKey)` (or
     structure the fake/test to compare the actual bytes passed to
     `NewFromKey` via a seam / exported constructor variant if a cleaner
     assertion point exists — implementer's choice, but the assertion must
     distinguish "used its own candidate" from "used the winner's key").
   - **Absent, persisted key exists but is corrupt (wrong length).** Fake's
     `GetFieldCryptoKey` returns a `[]byte` of length other than 32 (e.g. 16
     or 40) and `nil` error. Assert `NewFromEnvOrGenerate` returns a
     non-nil error and that `InsertFieldCryptoKeyIfAbsent` is never called —
     proving the corrupt-read path fails loudly rather than falling through
     to "absent" and regenerating over it.
   - **Absent check itself errors (e.g. connection failure).** Fake's
     `GetFieldCryptoKey` returns a non-`pgx.ErrNoRows` error (e.g.
     `errors.New("connection refused")`). Assert `NewFromEnvOrGenerate`
     returns a non-nil error wrapping it, and `InsertFieldCryptoKeyIfAbsent`
     is never called — the critical invariant that only a *confirmed-absent*
     read (via `pgx.ErrNoRows`, never any other error) triggers generation.
   - **Insert itself errors for a reason other than the conflict.** Fake's
     `GetFieldCryptoKey` returns `pgx.ErrNoRows`;
     `InsertFieldCryptoKeyIfAbsent` returns a non-`pgx.ErrNoRows` error.
     Assert `NewFromEnvOrGenerate` returns a non-nil error and does not call
     `GetFieldCryptoKey` a second time (no reason to re-fetch on a genuine
     insert failure, only on a lost-race conflict).

2. **Integration test** in a new file,
   `api/internal/fieldcrypto/generate_integration_test.go`
   (`//go:build integration`, package `fieldcrypto_test`), structurally
   following `api/authz/setup/grant_table_integration_test.go`'s established
   pattern (`TestMain`, `checkIntegrationPrerequisites` — docker + goose —,
   `resolvePostgresHost`/`postgresReachable`, a dedicated shadow database
   reset via goose against `model/migrations`, `os.Exit(0)` skip-not-fail
   when prerequisites are unmet). Use a distinct shadow DB name, e.g.
   `core_field_key_verify_dev`, to avoid colliding with the `authz/setup`
   suite's `core_grant_verify_dev` on the same shared Postgres container.
   Cover:
   - **Real fetch-or-generate round trip.** Against a freshly migrated
     (empty `field_crypto_keys`) shadow DB, call `NewFromEnvOrGenerate` with
     `coredb.New(pool)` as the `FieldKeyQuerier` (real generated queries, no
     fake). Assert success, and that a second call against the *same* DB
     returns a `Cipher` built from the *same* persisted key (encrypt with
     the first, decrypt with the second, or compare via a `NewFromKey`-built
     reference — implementer's choice) — proving the round trip through the
     real table, not just the Go-level control flow.
   - **Real concurrent-race proof.** Truncate/reset `field_crypto_keys` to
     empty, then launch N (e.g. 8) goroutines each calling
     `NewFromEnvOrGenerate` concurrently against independent `*coredb.Queries`
     values sharing the pool. Assert: (a) every goroutine succeeds, (b)
     exactly one row exists in `field_crypto_keys` afterward
     (`SELECT count(*) FROM field_crypto_keys` = 1), and (c) every goroutine's
     returned `Cipher` used the *same* key (cross-check via
     encrypt-with-one/decrypt-with-another for a sample of pairs, or by
     comparing against the one row's `key_bytes` directly via a raw query).
     This is the test that actually exercises the DB-level uniqueness
     constraint under real concurrency — the unit tests above can only prove
     the Go-level control flow handles a *simulated* lost race correctly,
     not that Postgres's constraint genuinely serializes real concurrent
     writers.
   - **Real corruption-detection proof.** Seed `field_crypto_keys` directly
     with a row via a raw `pool.Exec` bypassing the `octet_length = 32`
     CHECK's normal insert path is not possible (the CHECK constraint
     rejects it) — instead, prove the CHECK constraint itself: attempt a raw
     `INSERT INTO field_crypto_keys (id, key_bytes) VALUES (1, <16 random
     bytes>)` directly via `pool.Exec` and assert it fails with a constraint
     violation. This proves the DB-level defense-in-depth guard from
     `001-add-field-crypto-keys-table.md` actually rejects a short key at
     the schema level, independent of the Go-level length check
     `NewFromEnvOrGenerate`/`NewFromKey` also performs.

## Validation

- `cd api && go test ./internal/fieldcrypto/...` (unit tests, no build tag)
  passes, including every existing pre-`002` test in `fieldcrypto_test.go`
  and this task's new `generate_test.go` cases.
- `cd api && go vet ./internal/fieldcrypto/...` and `gofmt -l
  internal/fieldcrypto/` (via `make lint`) are clean.
- Integration test run (requires Docker + the shared Postgres container +
  `goose` on `PATH`, per the `authz/setup` precedent's own run instructions):

  ```sh
  cd api && \
    CORE_DEV_PG_HOST=<resolved-per-precedent> \
    go test -tags=integration -p 1 -v ./internal/fieldcrypto/...
  ```

  All `TestFieldKey*`-prefixed (or equivalently named) integration tests
  pass. If Docker/the shared container/`goose` are unavailable in the
  execution environment, the test must skip (exit 0, matching
  `checkIntegrationPrerequisites`'s existing skip-not-fail contract) rather
  than fail — confirm this by running without Docker available and observing
  a skip, not a failure.
- `go test ./...` (no tags) from `api/` still passes as a whole — confirms
  the new integration test file's `//go:build integration` tag correctly
  excludes it from the default unit-test run.

## Assumptions

- The shared Postgres container and `goose` availability in the actual
  execution environment for this task may or may not match the environment
  the `authz/setup` precedent was authored in; the integration test's
  prerequisite-check-and-skip contract exists precisely so this task can
  still be validated (unit tests always run; integration tests run when the
  environment supports them, skip cleanly otherwise).
- Reusing the shared Postgres container (`users-module-postgres`, per the
  precedent) is fine — this task's shadow DB name is distinct from
  `core_grant_verify_dev`, so no collision.

## References

- `api/authz/setup/grant_table_integration_test.go` — the structural
  precedent this task's integration test follows (`TestMain`, prerequisite
  checks, host resolution, shadow-DB reset via goose against
  `model/migrations`). Read in full before writing the new file.
- `api/internal/fieldcrypto/fieldcrypto_test.go` — the existing unit test
  file's conventions (`unsetEnv` helper, `t.Setenv`, table-style subtests via
  `t.Run`) this task's `generate_test.go` should match stylistically.
- `api/internal/fieldcrypto/fieldcrypto.go` (post-`002`) —
  `NewFromEnvOrGenerate`, `FieldKeyQuerier`, `fromPersistedOrGenerated` — the
  code under test.
- `model/migrations/0017_field_crypto_keys.sql` (from
  `001-add-field-crypto-keys-table.md`) — the `octet_length(key_bytes) = 32`
  CHECK this task's real corruption-detection proof exercises directly.

## Procedure

1. Write `generate_test.go` (Requirement 1); run and confirm every case
   passes and, for the "never touched"/"never called a second time" style
   assertions, confirm they would actually fail if the corresponding
   fail-loudly guard were removed (a quick manual sanity check, not a
   permanent mutation test).
2. Write `generate_integration_test.go` (Requirement 2), adapting the
   `authz/setup` precedent's `TestMain` scaffolding to this suite's own
   shadow DB name and migrations directory (already mod-core's own
   `model/migrations`, so no path-resolution changes needed beyond the DB
   name).
3. Run the Validation commands; fix and re-run until green (or cleanly
   skipped, for the integration tier, if prerequisites are unavailable).
4. Commit both new test files together as this task's change.

## Status

**Outcome:** succeeded. Date: 2026-08-10.

- `api/internal/fieldcrypto/generate_test.go` — new file, 7 unit tests (one
  table-driven with 2 subtests) against a hand-written `fakeFieldKeyQuerier`
  (2 methods), covering every branch in Requirement 1: env-wins/DB-untouched,
  env-invalid/DB-untouched, absent+generate+persist, absent+lost-race
  (asserted via decrypt-with-a-`NewFromKey(winnerKey)`-built reference
  Cipher, proving the returned Cipher used the winner's key and not its own
  candidate), corrupt persisted-key length (16 and 40 bytes, table-driven),
  absent-check read error (wraps via `errors.Is`), and insert-error-for-other-
  reason (asserts no re-fetch). Per the Procedure's step 1, I manually
  broke and restored (via a scratch backup, diffed identical afterward) two
  of the fail-loudly guards in `fieldcrypto.go` — the env-var short-circuit
  and the no-second-`Get`-on-genuine-insert-failure invariant — and confirmed
  the corresponding tests actually fail without them, before leaving the
  source unmodified.
- `api/internal/fieldcrypto/generate_integration_test.go` — new file
  (`//go:build integration`), following `grant_table_integration_test.go`'s
  `TestMain`/prerequisite-check/host-resolution/shadow-DB-reset pattern
  against a dedicated `core_field_key_verify_dev` shadow DB. Three tests:
  real fetch-then-refetch round trip, a real 8-goroutine concurrent race
  against `coredb.New(pool)` (asserting exactly one persisted row and that
  every goroutine's Cipher decrypts correctly against a reference Cipher
  built from that row's actual `key_bytes`), and a raw-SQL proof that the
  `octet_length(key_bytes) = 32` CHECK genuinely rejects a 16-byte key
  (asserted via the Postgres `23514` check_violation SQLSTATE, not just a
  generic error).
- All validation commands passed, including a full real run of the
  integration suite (not just the skip path) — see Assumptions note below on
  the one environment-specific wrinkle encountered getting there.

### Assumption/environment note for the manager

This sandbox's `localhost:5432` is occupied by a native (non-Docker)
Postgres process that owns the loopback-specific bind, which shadows the
`users-module-postgres` container's published port for both `127.0.0.1` and
`::1` — so `CORE_DEV_PG_HOST=localhost` (and the code's own `localhost`-first
`resolvePostgresHost` preference) resolves to the wrong server here and
fails with "role \"users\" does not exist" during the shadow-DB reset. This
is a pre-existing, environment-specific condition: the **already-landed**
`api/authz/setup/grant_table_integration_test.go` precedent fails
identically in this same sandbox for the identical reason, confirming it is
not something this task's new code introduced. Worked around for validation
purposes only (no source change) by pointing `CORE_DEV_PG_HOST` at the host
machine's LAN IP instead of `localhost`, which reaches the container's
actual published port; all three new integration tests then passed cleanly,
repeatedly, and under `-race`. Separately confirmed the skip-not-fail
contract itself (Validation's last integration bullet) by re-running with
`docker` removed from `PATH`: exits 0 as "ok", not a failure. No code change
made for either point — recorded here per this task's own Assumptions
about environment variability, and flagged for the manager as a
sandbox/CI-environment characteristic (native Postgres on `:5432`) that a
future integration-test run in a differently-shaped environment (e.g. CI)
may or may not encounter.
