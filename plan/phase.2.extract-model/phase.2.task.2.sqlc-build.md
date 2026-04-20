# Phase 2, Task 2 — sqlc build

## Context
Generate `core-module/model/db/` from the copied queries + migrations.

## Acceptance
- `cd core-module/model && make build` (or `sqlc generate` directly) produces `db/` with:
  - `db.go` (interface, Queries struct)
  - `models.go` (struct types for tables)
  - `entities.sql.go`, `legal_entities.sql.go`, `natural_persons.sql.go`, `corporations.sql.go`, `service_accounts.sql.go`
- `go build ./...` in core-module/model exits 0.
- Generated types match the users-module originals (spot-check struct fields).

## How to verify
- `ls core-module/model/db/` shows the expected files.
- `go build ./...` in core-module/model is clean.
- `diff <(sed 's|users-module/model/db|core-module/model/db|g' users-module/model/db/models.go) core-module/model/db/models.go` — should be substantively identical (modulo package docs, struct field additions from users-module-only tables).

## Notes
- If sqlc flags unused migrations/queries, check sqlc.yaml `omit_unused_structs` is NOT set in core-module/model (we want everything emitted since all 5 queries are used).
