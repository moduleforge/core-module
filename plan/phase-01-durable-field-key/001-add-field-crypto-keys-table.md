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
