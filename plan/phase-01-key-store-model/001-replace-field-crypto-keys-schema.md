# Replace Field Crypto Keys Schema And Queries

## Purpose and scope

Replace mod-core's single-row `field_crypto_keys` table with the multi-key, versioned design that
the rest of this plan is built on, and replace its two sqlc queries with the seven the cipher, the
read-path write-back, and the admin rotation API need.

Files this task owns:

- `model/migrations/0017_field_crypto_keys.sql` — rewritten in place (not superseded by an `0018`).
- `model/migrations/0010_natural_persons.sql` and `model/migrations/0011_corporations.sql` —
  comment-only edits.
- `model/queries/field_crypto_keys.sql` — rewritten outright.
- `model/README.md` — one added note on the migration reset step.

This task does **not** run `make gen` and does **not** touch `model/db/` or anything under `api/`;
[`003-regenerate-model-db-and-querier-stubs.md`](./003-regenerate-model-db-and-querier-stubs.md)
owns regeneration and the api-side fallout. No standard skill covers this work; follow the
[Procedure](#procedure) below.

## Requirements

1. **Rewrite `model/migrations/0017_field_crypto_keys.sql` in place** with the replacement DDL given
   verbatim in
   [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#replacement-ddl).
   Reproduce that DDL and its comment block as written — the comments are the schema-level design
   record and match the comment density of the surrounding migrations. It comprises:
   - the `field_crypto_keys` table with `version INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY`,
     `key_bytes BYTEA NOT NULL UNIQUE CHECK (octet_length(key_bytes) = 32)`, `created_at`,
     `updated_at`, `retired_at`, `decryptable_until`, `compromised_at`, and the two named CHECK
     constraints `field_crypto_keys_retired_only_flags` and
     `field_crypto_keys_grace_after_retirement`;
   - the `field_crypto_keys_one_active` partial unique index over `((retired_at IS NULL))
     WHERE retired_at IS NULL`, which is both the one-active-key invariant and the first-boot
     convergence arbiter that replaces the old `CHECK (id = 1)`;
   - the `field_crypto_keys_set_updated_at` `BEFORE UPDATE` trigger using the existing
     `set_updated_at()` helper from `0001_helpers.sql`;
   - a `-- +goose Down` section dropping the trigger and the table.
2. **Do not add an `0018` migration.** The clean-break ground rule means no database exists that
   needs an upgrade path, and leaving `0017`'s old comment block standing would document a table
   shape that no longer exists.
3. **Fallback if the pinned tooling rejects the identity syntax.** If `goose validate` or
   `sqlc compile` rejects `GENERATED ALWAYS AS IDENTITY`, fall back to `version SERIAL PRIMARY KEY`
   plus `CHECK (version > 0)`, and say so in the task report. Nothing else in the design depends on
   the identity form. Note that `model/Makefile` pins **sqlc v1.28.0**, not the v1.31.1 the design
   note assumed — verify against the pinned version actually installed.
4. **Comment-only edits to `0010_natural_persons.sql` and `0011_corporations.sql`.** Both document
   their encrypted blob column as holding `nonce || ciphertext || tag`; update both to
   `version || nonce || ciphertext || tag`. Change no DDL in either file.
5. **Rewrite `model/queries/field_crypto_keys.sql`** with exactly seven queries, deleting
   `GetFieldCryptoKey` and `InsertFieldCryptoKeyIfAbsent` outright. The full specification of each is
   in the [sqlc query surface](../notes/key-store-schema-design.md#sqlc-query-surface) table, plus
   the seventh query added by
   [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#routes):
   - `ListUsableFieldCryptoKeys` (`:many`) — full column list, ordered by `version`, filtered
     `retired_at IS NULL OR decryptable_until IS NULL OR decryptable_until > now()`.
   - `InsertInitialFieldCryptoKey` (`:one`) — the two-guard bootstrap insert written exactly as the
     schema note gives it: `WHERE NOT EXISTS (SELECT 1 FROM field_crypto_keys)` **and** an
     untargeted `ON CONFLICT DO NOTHING`. Do not name the partial index as a conflict target.
   - `RetireActiveFieldCryptoKey` (`:one`) — stamps `retired_at = now()`, resolves
     `decryptable_until` from a nullable grace-days argument, sets `compromised_at = now()` when a
     boolean `compromised` argument is true, `WHERE retired_at IS NULL RETURNING version`.
   - `InsertActiveFieldCryptoKey` (`:one`) — inserts the new active row, full column list returned,
     no `ON CONFLICT` clause.
   - `MarkFieldCryptoKeyCompromised` (`:one`) — `SET compromised_at = COALESCE(compromised_at,
     now()) WHERE version = $1 AND retired_at IS NOT NULL RETURNING version, compromised_at`.
   - `SetFieldCryptoKeyDecryptableUntil` (`:one`) — same `now() + n * INTERVAL '1 day'` resolution,
     nullable days to clear the deadline, `WHERE version = $1 AND retired_at IS NOT NULL`.
   - `ListFieldCryptoKeyMetadata` (`:many`) — `version, created_at, updated_at, retired_at,
     decryptable_until, compromised_at ORDER BY version`. **Never selects `key_bytes`**; its narrower
     column list is what makes "no key material crosses the admin boundary" a compile-time property.
6. **Query-authoring mechanics.** Give the five key-material-bearing queries the full column list in
   table order so sqlc reuses the `FieldCryptoKey` model struct instead of emitting per-query `...Row`
   types. Use `sqlc.narg()` for the nullable grace-days arguments and `sqlc.arg(key_bytes)::BYTEA`
   for the bare-`SELECT` bootstrap insert, whose parameter type is otherwise uninferable.
7. **Record the migration reset step in `model/README.md`.** goose stores no per-migration checksum,
   so a developer or CI database that already applied the old `0017` reports "no pending migrations"
   and silently keeps the single-row table. Document both recoveries — `goose -dir migrations
   postgres "$DATABASE_URL" down-to 16` followed by `up`, or recreating the database wholesale (the
   more likely choice, since the blob-format change requires it anyway).

## Validation

- `cd model && make verify` passes (`goose validate` + `sqlc compile`). This is the primary gate: it
  proves the seven new queries type-check against the replacement schema.
- `cd model && make lint` passes — applies every migration in order to an ephemeral shadow Postgres
  (requires Docker). This is what proves the DDL, the partial unique index, the two CHECK
  constraints, and the trigger are all actually valid.
- `grep -n "id = 1" model/migrations/0017_field_crypto_keys.sql` returns nothing.
- `grep -rn "GetFieldCryptoKey\|InsertFieldCryptoKeyIfAbsent" model/queries/` returns nothing.
- `grep -c "^-- name:" model/queries/field_crypto_keys.sql` returns `7`.
- `grep -n "key_bytes" model/queries/field_crypto_keys.sql` shows no occurrence inside the
  `ListFieldCryptoKeyMetadata` body.
- `git diff --stat` shows exactly five changed files: the three migrations, the query file, and
  `model/README.md`. Nothing under `model/db/` or `api/` is touched.
- Manual sanity check against the shadow database, or by inspection: attempting a second row with
  `retired_at IS NULL` violates `field_crypto_keys_one_active`.

## Metadata

architectural_impact: true

## Assumptions

- No deployed database is running the current `0017`, so an in-place edit is safe. This is the
  user's explicit statement and the plan's clean-break ground rule.
- `set_updated_at()` exists in `0001_helpers.sql` and is the same helper every other mutable table's
  trigger uses.
- `model/db/` is stale after this task and the `api` module still compiles, because
  `api/internal/fieldcrypto` declares its own `FieldKeyQuerier` interface rather than importing
  `coredb`. Regeneration is task 003's job.

## References

- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md) — the settled DDL,
  column-by-column rationale, invariant table, first-boot convergence mechanism, migration-path
  decision, and query surface. Read [Replacement DDL](../notes/key-store-schema-design.md#replacement-ddl)
  and [sqlc query surface](../notes/key-store-schema-design.md#sqlc-query-surface) in full.
- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#routes) — the
  `ListFieldCryptoKeyMetadata` seventh query and why it excludes `key_bytes`.
- `model/migrations/0017_field_crypto_keys.sql` — the file being replaced; its comment block explains
  the convergence property the replacement must preserve.
- `AGENTS.md` — [Database migrations](../../../AGENTS.md) and the module-local numbering convention.

## Procedure

1. Read the replacement DDL and the query surface table in `../notes/key-store-schema-design.md`.
2. Rewrite `0017_field_crypto_keys.sql` (Up and Down).
3. Run `cd model && make lint` early — a DDL error is cheapest to find before the queries are
   written.
4. Rewrite `model/queries/field_crypto_keys.sql`.
5. Run `cd model && make verify`, iterating on `sqlc.arg`/`sqlc.narg` annotations until it passes.
6. Apply the comment-only edits to `0010` and `0011`.
7. Add the reset note to `model/README.md`.
8. Re-run both `make verify` and `make lint`.

## Checkpoint hints

- After the `0017` rewrite passes `make lint`.
- After the seven queries pass `make verify`.
- After the `0010`/`0011` comment edits and the `model/README.md` note.
