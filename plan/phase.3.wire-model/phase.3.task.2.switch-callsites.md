# Phase 3, Task 2 — Switch users-module/api call sites to coredb

## Context
Handlers currently import `github.com/moduleforge/users-module/model/db` for core tables. Switch these specific queries to `github.com/moduleforge/core-model/db` (aliased `coredb`). Leaves users/apps/audit/etc. on the users-module db package.

## Inputs — files confirmed to touch core tables
- `users-module/api/internal/handlers/self.go` — uses `GetLegalEntityByEntityID`, `GetNaturalPersonByLegalEntityID`, `GetServiceAccountByEntityID`, `UpdateNaturalPerson`.
- `users-module/api/internal/handlers/users.go` — uses `CreateEntity`, `CreateLegalEntity`, `CreateNaturalPerson`, `GetLegalEntityByEntityID`, `GetNaturalPersonByLegalEntityID`, `UpdateNaturalPerson`, `ArchiveEntity`.
- `users-module/api/internal/handlers/auditlog.go` — uses `GetEntityByUUID`.

Use grep to confirm: `grep -nR "GetEntityByID\|CreateEntity\|GetLegalEntity\|CreateLegalEntity\|GetNaturalPerson\|CreateNaturalPerson\|UpdateNaturalPerson\|ArchiveEntity\|GetServiceAccount\|CreateCorporation\|GetCorporation\|CreateServiceAccount" users-module/api/internal/`

## Acceptance
- For each identified call, change the receiver from `h.q` (users-module Queries) to a new `h.coreQ` field (core-module Queries), or — simpler — add a separate `coreQueries *coredb.Queries` dependency and inject it via constructor.
- Handler constructors accept both `*db.Queries` (users-module) and `*coredb.Queries` (core-module).
- `cmd/server/main.go` instantiates both Queries against the same pgx pool and passes both to handler constructors.
- `go build ./...` exits 0.
- All existing tests still pass.

## How to verify
- `grep -nR "h.q\.GetEntityByID\|h.q\.CreateEntity\|h.q\.GetLegalEntityByEntityID" users-module/api/internal/` returns nothing (all migrated to `h.coreQ`).
- `go test ./...` green.

## Notes
- This is transitional. Phase 5 replaces these calls with core-module service calls. Keep the diff mechanical — don't refactor beyond the import swap.
- If a method has a slightly different signature in coredb (unlikely, since schema is identical), stop and flag.
