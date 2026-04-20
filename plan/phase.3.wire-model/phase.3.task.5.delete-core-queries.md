# Phase 3, Task 5 — Delete core query files from users-module/model/queries

## Context
These files have moved to core-module/model/queries. users-module must no longer ship duplicates.

## Acceptance
- Delete:
  - `users-module/model/queries/entities.sql`
  - `users-module/model/queries/legal_entities.sql`
  - `users-module/model/queries/natural_persons.sql`
  - `users-module/model/queries/corporations.sql`
  - `users-module/model/queries/service_accounts.sql`
- users-module's own query files (`users.sql`, `auth_local.sql`, `apps.sql`, etc.) remain.

## How to verify
- `ls users-module/model/queries/` shows no entity/legal_entities/natural_persons/corporations/service_accounts files.
- `make build` in users-module/model regenerates `db/` without those entries (after 3.8).

## Notes
- Run after 3.2 so handlers have already moved to coredb.
