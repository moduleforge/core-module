# Add Field Crypto Keys Table

## Purpose and scope

Add a new, mod-core-owned migration and sqlc query pair for a private,
single-row `field_crypto_keys` table — the persistence home for the
AES-256-GCM field-encryption key when `CORE_FIELD_KEY_HEX` is not supplied
via the environment. This task creates the schema and generated Go query
code only; the fetch-or-generate logic that uses these queries is
[`002-implement-auto-generate-cipher.md`](./002-implement-auto-generate-cipher.md).

No standard skill covers schema/codegen work end-to-end here; follow the
[`## Procedure`](#procedure) below, which is written specifically for this
task.

## Requirements

1. **New migration** at `model/migrations/0017_field_crypto_keys.sql` (the
   next free number after `0016_apps_updated_at.sql`; `0099_access_function_stubs.sql`
   is reserved and must stay last). Goose format (`-- +goose Up` / `-- +goose Down`),
   matching the style of existing files in this directory (e.g.
   `model/migrations/0015_apps.sql`, `model/migrations/0002_types.sql`).

   ```sql
   -- +goose Up

   -- field_crypto_keys is a private, single-row table holding the
   -- AES-256-GCM field-encryption key when CORE_FIELD_KEY_HEX is not
   -- supplied via the environment (see
   -- api/internal/fieldcrypto.NewFromEnvOrGenerate). The id = 1 CHECK plus
   -- PRIMARY KEY makes this table hold at most one row ever: concurrent
   -- first-boot INSERTs race on that constraint, so exactly one writer wins
   -- and every other writer must adopt the winner's key rather than each
   -- independently generating one and picking arbitrarily (see
   -- InsertFieldCryptoKeyIfAbsent's ON CONFLICT (id) DO NOTHING below). The
   -- octet_length CHECK is a defense-in-depth guard against a corrupt or
   -- truncated key ever being persisted through this code path;
   -- NewFromEnvOrGenerate independently validates length on every read and
   -- fails loudly rather than regenerating if a persisted key does not
   -- decode to exactly 32 bytes.
   --
   -- Deliberately not modeled as an entity (no FK to entities.id): this is
   -- bootstrap/operational data unrelated to the domain entity hierarchy,
   -- the same reasoning that keeps goose_db_version_core a bare table.
   CREATE TABLE field_crypto_keys (
     id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
     key_bytes  BYTEA NOT NULL CHECK (octet_length(key_bytes) = 32),
     created_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );

   -- +goose Down

   DROP TABLE IF EXISTS field_crypto_keys;
   ```

2. **New sqlc query file** at `model/queries/field_crypto_keys.sql`:

   ```sql
   -- name: GetFieldCryptoKey :one
   SELECT key_bytes FROM field_crypto_keys WHERE id = 1;

   -- name: InsertFieldCryptoKeyIfAbsent :one
   INSERT INTO field_crypto_keys (id, key_bytes)
   VALUES (1, $1)
   ON CONFLICT (id) DO NOTHING
   RETURNING key_bytes;
   ```

   `GetFieldCryptoKey` returns `pgx.ErrNoRows` when no key has been persisted
   yet. `InsertFieldCryptoKeyIfAbsent` returns `pgx.ErrNoRows` when the
   `ON CONFLICT DO NOTHING` clause skips the insert — i.e. another writer
   already holds the row — which `002-implement-auto-generate-cipher.md`'s
   caller must distinguish from a genuine error and handle by re-fetching the
   winner's key. No `sqlc.yaml` override is needed: `BYTEA` maps to Go
   `[]byte` by sqlc's default pgx/v5 mapping.

3. **Regenerate `model/db/`** via `cd model && sqlc generate` (or
   `make gen`). This adds `GetFieldCryptoKey` and `InsertFieldCryptoKeyIfAbsent`
   to the generated `Queries` struct and the `Querier` interface in
   `model/db/querier.go`, plus any new row/param types in `model/db/models.go`
   or a new `model/db/field_crypto_keys.sql.go` file (sqlc's usual per-query-file
   split). Commit the generated output — `model/db/` is committed to the
   repo per `AGENTS.md`'s code-generation conventions, never hand-edited.

## Validation

- `cd model && make verify` — goose validate + sqlc compile (per `AGENTS.md`'s
  test commands table) succeeds against the new migration and query file.
- `cd model && make lint` — applies migrations (including the new one) to an
  ephemeral Postgres via Docker; confirms `0017_field_crypto_keys.sql` applies
  cleanly on top of `0001`-`0016` and the `-- +goose Down` block reverses
  cleanly (goose's up/down round-trip, exercised however `make lint` already
  does this for the existing migrations).
- `git diff --stat` shows exactly: the new migration file, the new query
  file, and the sqlc-regenerated files under `model/db/` (`querier.go` plus
  whichever per-query files sqlc emits/touches for this addition) — no
  hand-edits to any other `model/db/*.go` file.
- `grep -n "field_crypto_keys" model/db/querier.go` shows both
  `GetFieldCryptoKey` and `InsertFieldCryptoKeyIfAbsent` in the `Querier`
  interface.
- `cd api && go build ./...` still succeeds (api/go.mod's local `replace` on
  `core-model` picks up the regenerated `model/db/` package immediately; no
  version bump needed for this same-repo, local-replace build).

## Metadata

architectural_impact: true

## Assumptions

- `model/db/` regeneration order/content is deterministic given sqlc v1.31.1
  (the version recorded in the existing generated-code header comments); no
  manual reconciliation should be needed beyond running `sqlc generate`.
- The table is named `field_crypto_keys` (plural, matching this repo's other
  table names) even though it will only ever hold one row — consistent with
  the existing convention rather than a special-cased singular name.

## References

- `model/migrations/0015_apps.sql`, `model/migrations/0002_types.sql` — style
  precedent for goose migration files in this repo, including the
  domain-invariant CHECK-constraint pattern this task's `octet_length` check
  follows.
- `model/migrations/0016_apps_updated_at.sql` — the current last numbered
  migration before the reserved `0099` stub file; confirms `0017` is next.
- `model/migrate.go` — `Migrate(ctx, db)` / `TableName = "goose_db_version_core"`;
  unchanged by this task, included for context on how the new migration gets
  applied at composing-app startup.
- `model/sqlc.yaml` — confirms no `db_type: "bytea"` override exists or is
  needed (default `[]byte` mapping).
- `model/db/db.go` — the generated `DBTX`/`Queries`/`New(db DBTX)` shape this
  task's new queries slot into unchanged.

## Procedure

1. Write the migration file (Requirement 1).
2. Write the query file (Requirement 2).
3. Run `cd model && sqlc generate` (Requirement 3); inspect the diff for
   unexpected changes to unrelated generated files.
4. Run the Validation commands above; fix and re-run until green.
5. Commit the migration file, query file, and regenerated `model/db/` output
   together as this task's change.

## Checkpoint hints

- After writing the migration file and confirming `make verify` accepts it.
- After writing the query file and running `sqlc generate`, before moving on
  to validate the full `model/` build.

## Status

Implementation outcome: **succeeded**. Date: 2026-08-10.

- Added `model/migrations/0017_field_crypto_keys.sql` (goose Up/Down,
  `CREATE TABLE field_crypto_keys` with the `id = 1` CHECK and the
  `octet_length(key_bytes) = 32` CHECK, matching the doc comment style of
  `model/migrations/0015_apps.sql` / `0002_types.sql`).
- Added `model/queries/field_crypto_keys.sql` (`GetFieldCryptoKey`,
  `InsertFieldCryptoKeyIfAbsent`).
- Ran `cd model && sqlc generate` (sqlc v1.31.1, matching the assumption
  above); diff is exactly the new migration, the new query file, and the
  sqlc-regenerated `model/db/field_crypto_keys.sql.go` (new file),
  `model/db/models.go` (+`FieldCryptoKey` struct), and
  `model/db/querier.go` (+2 interface methods) — no other generated file
  touched.
- Validation:
  - `cd model && make verify` — passed.
  - `cd model && make lint` — passed (applies `0001`-`0017`+`0099` to an
    ephemeral Postgres via Docker cleanly). `make lint`'s
    `shadow-db-lint.sh` only runs `goose ... up`, not a down/up round
    trip, so the Down-block reversal named in this task's Validation
    section is not actually exercised by `make lint` as it exists today
    (flagged below). I separately verified the Down block manually: `up-to
    16` → `up-by-one` (applies `0017`) → `down` (reverses `0017`) against
    a throwaway container — reversal is clean.
  - `git diff --stat` (staged) shows exactly the 5 expected files (2 new
    source files, 3 sqlc-regenerated files) — no hand-edits elsewhere.
  - `grep -n "field_crypto_keys" model/db/querier.go` — **produces no
    match** (exit 1) because sqlc emits PascalCase Go identifiers
    (`GetFieldCryptoKey`, `InsertFieldCryptoKeyIfAbsent`), never the
    literal snake_case table name, for every table in this file (true of
    every existing entry too, e.g. `legal_entities` → `GetLegalEntityByEntityID`).
    A case-adjusted check (`grep -n -i "FieldCryptoKey"
    model/db/querier.go`) confirms both methods are present in the
    `Querier` interface, satisfying the check's evident intent. Flagged
    for the manager below since the literal command in this task's
    Validation section cannot pass as written.
  - `cd api && go build ./...` — passed (local `replace` on `core-model`
    picks up the regenerated `model/db/` package immediately).
- Assumptions from `## Assumptions` held: sqlc v1.31.1 regeneration was
  deterministic with no manual reconciliation needed; table named
  `field_crypto_keys` (plural) per convention.
- No follow-up work needed for `002-implement-auto-generate-cipher.md`
  beyond what it already anticipates in this task's `## Purpose and scope`.
