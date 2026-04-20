# Phase 3, Task 7 — Point atlas.hcl + sqlc.yaml at composed dir

## Context
Both atlas and sqlc must read from the composed dir, not the bare `migrations/`.

## Acceptance
- `users-module/model/atlas.hcl` migration dir changed to `file://schema/migrations`.
- `users-module/model/sqlc.yaml` schema path changed to `./schema/migrations` (queries stay at `./queries`).
- `users-module/model/sqlc.yaml` adds `omit_unused_structs: true` under `gen.go` so core-table types aren't emitted into users-module's db package.

## How to verify
- `make compose && make build` in users-module/model succeeds.
- `users-module/model/db/models.go` (after regen) contains no `Entity`, `LegalEntity`, `NaturalPerson`, `Corporation`, or `ServiceAccount` types.
- `users-module/model/db/` still contains User, AuthLocal, App, AppsUser, AuditLog, EmailCode, PasswordReset, OidcConfig, OidcProvider struct types.

## Notes
- `omit_unused_structs` needs sqlc v1.27+. If older, pin a newer version.
