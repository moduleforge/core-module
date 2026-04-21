# Phase 8, Task 3 — Queries and sqlc regeneration

## Context
Blocked by task.2 (migrations). Once the `ssn` / `ein` columns exist, the
sqlc queries and generated Go need to know about them.

## Location
- `core-module/model/queries/natural_persons.sql`
- `core-module/model/queries/corporations.sql`
- Generated output: `core-module/model/db/natural_persons.sql.go`,
  `core-module/model/db/corporations.sql.go`, `db/models.go`.

Do not hand-edit generated files — run sqlc.

## Query edits

### natural_persons.sql

Replace the three existing queries with the versions below. The shape of
every `RETURNING` / `SELECT` list stays aligned with the struct produced
by sqlc.

```sql
-- name: CreateNaturalPerson :one
INSERT INTO natural_persons (entity_id, given_name, family_name, ssn)
VALUES ($1, $2, $3, $4)
RETURNING id, entity_id, given_name, family_name, ssn, created_at, updated_at;

-- name: GetNaturalPersonByEntityID :one
SELECT id, entity_id, given_name, family_name, ssn, created_at, updated_at
FROM natural_persons
WHERE entity_id = $1;

-- name: UpdateNaturalPerson :exec
UPDATE natural_persons
SET given_name = $2,
    family_name = $3,
    ssn = COALESCE($4, ssn)
WHERE entity_id = $1;
```

Rationale for `COALESCE($4, ssn)`:

- `$4 = NULL` means the caller did not supply a new ssn — leave existing
  untouched.
- `$4 = ''::bytea` (zero-length blob) means the caller wants to clear the
  ssn. Because bytea `''` is a distinct non-NULL value, it overwrites the
  existing one. Document this convention in a SQL comment.

Add this comment above the UPDATE:

```sql
-- NOTE: pass NULL for ssn to leave it unchanged; pass an empty bytea
-- ('\x'::bytea / []byte{}) to clear it. A non-empty bytea replaces it.
```

### corporations.sql

Mirror the same pattern:

```sql
-- name: CreateCorporation :one
INSERT INTO corporations (entity_id, legal_name, jurisdiction, ein)
VALUES ($1, $2, $3, $4)
RETURNING id, entity_id, legal_name, jurisdiction, ein, created_at, updated_at;

-- name: GetCorporationByEntityID :one
SELECT id, entity_id, legal_name, jurisdiction, ein, created_at, updated_at
FROM corporations
WHERE entity_id = $1;

-- NOTE: pass NULL for ein to leave it unchanged; pass an empty bytea
-- to clear it. A non-empty bytea replaces it.
-- name: UpdateCorporation :exec
UPDATE corporations
SET legal_name = $2,
    jurisdiction = $3,
    ein = COALESCE($4, ein)
WHERE entity_id = $1;
```

## Regenerate

```sh
cd core-module/model
make build          # expected to invoke sqlc generate
```

Inspect the regenerated files to confirm the new fields:

- `db/models.go` — `NaturalPerson` gains `Ssn []byte`; `Corporation`
  gains `Ein []byte`.
- `db/natural_persons.sql.go`, `db/corporations.sql.go` — params structs
  include the new field; row-returning queries scan it.

## Acceptance
- `make build` in `core-module/model` succeeds.
- `go vet ./...` and `go build ./...` in `core-module/model` succeed.
- Generated `models.go` now contains the new `Ssn` / `Ein` fields as
  `[]byte`.
- No other generated-file diffs beyond what the query changes imply.

## How to verify
```sh
cd core-module/model
make build
grep -n 'Ssn\s\+\[\]byte' db/models.go
grep -n 'Ein\s\+\[\]byte' db/models.go
go build ./...
```

## Notes
- Do NOT add a `GetNaturalPersonSSN` single-column query. The service
  layer will use the existing Get query and pull the field out of the
  struct; a dedicated query adds maintenance burden without a benefit.
- Do not touch any other query files.
