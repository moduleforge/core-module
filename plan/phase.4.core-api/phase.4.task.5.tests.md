# Phase 4, Task 5 — Unit tests

## Context
Two test surfaces: service-level (DB + audit logic) and httpapi-level (auth paths, encoding).

## Acceptance

### Service tests
Use a real Postgres via testcontainers or a shared dev DB (prefer the same pattern users-module/api tests use — check `users-module/api/internal/handlers/*_test.go`). Alternatively, test with sqlmock + a mocked `audit.Writer`.

Tests to include:
- `TestNaturalPersonService_Create_WritesAudit` — create succeeds, audit.Write called with op=`create`, resource=`natural_person`, non-nil `after`.
- `TestNaturalPersonService_Create_TxRollback` — if caller's tx rolls back, entity row is gone. (Skip if using sqlmock — covered implicitly.)
- `TestNaturalPersonService_Update_Forbidden` — non-admin actor updating another entity → `ErrForbidden`.
- `TestNaturalPersonService_Update_SelfAllowed` — non-admin actor updating own entity → success.
- `TestEntityService_Archive_AdminOnly` — non-admin → `ErrForbidden`.

### httpapi tests (httptest)
Use a fake `PrincipalExtractor` and a fake `Services` (interfaces via `emit_interface: true` in sqlc + hand-written service interfaces? — actually: introduce service interfaces in `service/service.go` so httpapi depends on interfaces, not concrete structs. Use mock impls in tests.)

Tests:
- `TestGetSelf_401_WhenNoPrincipal` — extractor returns false → 401.
- `TestGetSelf_200_HappyPath` — extractor returns principal, fake service returns body → 200 with JSON.
- `TestPutSelf_400_BadJSON`.
- `TestCreateNaturalPerson_403_NonAdmin`.
- `TestArchiveEntity_404_NotFound`.

## How to verify
- `go test ./...` in core-module/api exits 0.
- Coverage report shows >70% on service and httpapi packages.

## Notes
- For integration-level service tests, wiring a real Postgres may be heavier than justified for Phase 4. If so, mock at the coredb.Querier level using sqlc's `emit_interface` output and test business logic purely.
