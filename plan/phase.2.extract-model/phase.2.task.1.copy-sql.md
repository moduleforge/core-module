# Phase 2, Task 1 — Copy SQL into core-module/model

## Context
Migrations 0000–0005 and matching queries must land in core-module/model without modification.

## Acceptance
- `core-module/model/migrations/` contains:
  - `0000_helpers.sql`
  - `0001_entities.sql`
  - `0002_legal_entities.sql`
  - `0003_natural_persons.sql`
  - `0004_corporations.sql`
  - `0005_service_accounts.sql`
- `core-module/model/queries/` contains:
  - `entities.sql`
  - `legal_entities.sql`
  - `natural_persons.sql`
  - `corporations.sql`
  - `service_accounts.sql`
- Each file byte-identical to its source in `users-module/model/`.

## How to verify
```sh
for f in 0000_helpers 0001_entities 0002_legal_entities 0003_natural_persons 0004_corporations 0005_service_accounts; do
  diff users-module/model/migrations/$f.sql core-module/model/migrations/$f.sql
done
for f in entities legal_entities natural_persons corporations service_accounts; do
  diff users-module/model/queries/$f.sql core-module/model/queries/$f.sql
done
```
All diffs must be empty.

## Notes
- `atlas.sum` from users-module must NOT be copied — core-module's atlas.sum is fresh.
- Do not yet delete the users-module originals — Phase 3 handles the deletion.
