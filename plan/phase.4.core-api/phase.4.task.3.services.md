# Phase 4, Task 3 — Entity services

## Context
Business logic currently lives in `users-module/api/internal/handlers/{self.go,users.go}`. Move it into tx-aware service methods so:
1. users-module's admin user-create can call `NaturalPerson.Create(ctx, tx, input)` inside its own pgx tx.
2. httpapi handlers (Task 4.4) are reduced to decode → call service → encode.

## Inputs — logic to migrate
- `users-module/api/internal/handlers/self.go`:
  - `Get()` — resolve entity kind, load legal_entity / natural_person / corporation / service_account, compose response.
  - `Put()` — update natural_person fields (given_name, family_name) via `UpdateNaturalPerson`.
  - `buildEntityInfo()` helper.
- `users-module/api/internal/handlers/users.go`:
  - Admin `Create()` — creates entity + legal_entity + natural_person in a transaction (users.go:79–105).
  - Admin `Get()`, `Update()` — touch natural_person fields.
  - `ArchiveEntity` call.

## Acceptance
File layout:
- `core-module/api/service/entity.go` — `type EntityService` with `GetByUUID`, `Archive` methods.
- `core-module/api/service/legal_entity.go` — `GetByEntityID`, `Create`.
- `core-module/api/service/natural_person.go` — `Create`, `GetByEntityUUID`, `UpdateByEntityUUID`.
- `core-module/api/service/corporation.go` — `Create`, `GetByEntityUUID`, `UpdateByEntityUUID`.
- `core-module/api/service/service_account.go` — `Create`, `GetByEntityUUID`, `UpdateByEntityUUID`.
- `core-module/api/service/service.go` — `type Services struct { Entity *EntityService; NaturalPerson *NaturalPersonService; ... }` + `New(pool *pgxpool.Pool, aw audit.Writer) *Services`.

Method shape (example — NaturalPersonService.Create):
```go
type CreateNaturalPersonInput struct {
    GivenName  string
    FamilyName string
    // any other natural_person fields
}

type NaturalPersonService struct {
    coreQ coredb.Querier   // core-module/model/db
    aw    audit.Writer
}

// Create inserts entity → legal_entity → natural_person within the given tx.
// Returns the created natural_person's UUID (from the entity row) and the
// populated record.
func (s *NaturalPersonService) Create(
    ctx context.Context,
    tx coredb.DBTX,   // accepts *pgxpool.Pool or pgx.Tx
    actor Principal,
    in CreateNaturalPersonInput,
) (coredb.NaturalPerson, uuid.UUID, error)
```

- Each mutation method calls `s.aw.Write(ctx, op, resource, targetEntityID, before, after)` on success.
- Authorization hooks: `UpdateByEntityUUID` rejects if `actor.IsAdmin == false && actor.EntityID != targetEntityID`. Admin-only methods (Create for NP used for admin onboarding, Archive, etc.) check `actor.IsAdmin` and return a typed `ErrForbidden`.
- Export sentinel errors: `ErrNotFound`, `ErrForbidden`, `ErrInvalidInput`.

## How to verify
- `go build ./service` exits 0.
- Structural check: each service method accepts `coredb.DBTX` (not `*pgx.Conn` or `*pgxpool.Pool` directly) so consumers can pass either.
- Unit tests (Task 4.5) verify audit writes, tx rollback propagation, and authorization checks.

## Notes
- The `CreateNaturalPerson` flow in users-module today calls 3 queries sequentially (CreateEntity → CreateLegalEntity → CreateNaturalPerson). Preserve that exact sequence inside `NaturalPersonService.Create`.
- Do NOT open a tx inside service methods — caller controls tx lifecycle. Method takes `coredb.DBTX` (the interface sqlc emits) and calls `coreQ.WithTx(tx).CreateEntity(...)` if the passed-in tx is a `pgx.Tx`, otherwise uses `coreQ` directly.
- `coredb.Querier` is the generated interface (from `emit_interface: true` in sqlc.yaml); services depend on the interface, not the concrete struct — easier to mock in tests.
