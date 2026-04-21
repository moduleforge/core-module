# Phase 8, Task 2 — SQL migrations for ssn / ein

## Context
Add encrypted-at-rest columns for tax ids. Storage is opaque `BYTEA`; the
application encrypts before write and decrypts after read (see task.1 and
task.4). No Postgres extensions; vanilla SQL only.

## Location
`core-module/model/migrations/`

Create two new migration files using the next unused numeric prefixes
(current highest is `0011_service_accounts.sql`):

- `0012_natural_persons_ssn.sql`
- `0013_corporations_ein.sql`

Do not modify any existing migration.

## Migration 0012 — content

```sql
-- ssn holds an AES-256-GCM blob (nonce || ciphertext || tag) produced
-- by the application. NULL means no SSN is recorded. The database never
-- sees plaintext.
ALTER TABLE natural_persons
  ADD COLUMN ssn BYTEA;
```

That's it. No index, no constraint, no default. The column is nullable
on purpose (many records will not have an SSN).

## Migration 0013 — content

```sql
-- ein holds an AES-256-GCM blob (nonce || ciphertext || tag) produced
-- by the application. NULL means no EIN is recorded.
ALTER TABLE corporations
  ADD COLUMN ein BYTEA;
```

## Re-hash atlas.sum

After the two files are written, run:

```sh
cd core-module/model
atlas migrate hash
```

That updates `atlas.sum`. Commit the updated sum file.

## Acceptance
- Two new migration files exist with the SQL above (byte-for-byte).
- `atlas.sum` now covers them (its contents change — that is expected).
- `atlas migrate validate` returns success.

## How to verify
```sh
cd core-module/model
atlas migrate validate
atlas migrate hash --dir file://migrations   # idempotent after first run
```

Plus a dry-run apply against a throwaway db if one is available:
```sh
atlas migrate apply --url "$CORE_TEST_DB_URL"
psql "$CORE_TEST_DB_URL" -c "\d natural_persons" | grep -q 'ssn .*bytea'
psql "$CORE_TEST_DB_URL" -c "\d corporations"    | grep -q 'ein .*bytea'
```

## Notes
- Do NOT add CHECK constraints on blob length — GCM output length depends
  on plaintext size.
- Do NOT add `NOT NULL DEFAULT ''::bytea`. Empty bytea and NULL are
  deliberately distinguished: NULL means "not set", and the app layer
  should not store an empty blob.
- Do NOT edit query files in this task; task.3 handles that.
