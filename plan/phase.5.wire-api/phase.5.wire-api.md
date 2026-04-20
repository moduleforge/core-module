# Phase 5 — Wire users-module/api to consume core-module/api

## Goal
Make users-module/api mount core-module's chi subrouter at `/v1` (inside its existing auth-protected group), reroute admin user-create to call core service inside a pgx tx, and delete the now-redundant handlers.

## Preconditions
- Phase 3 complete (users-module uses coredb for the 5 tables).
- Phase 4 complete (core-module/api ships service + httpapi).

## Outputs
- `users-module/api/go.mod` requires `github.com/moduleforge/core-api`.
- `users-module/api/internal/auth/core_adapter.go` implements `service.PrincipalExtractor`.
- `users-module/api/internal/audit/audit.go` satisfies `core-api/audit.Writer` (structurally or via adapter).
- `users-module/api/cmd/server/main.go` constructs core services + router; mounts at `/v1`.
- `/v1/self` routes removed from users-module's handler registrations (served by core).
- `users-module/api/internal/handlers/users.go` admin-create rewired to call core service inside a pgx tx.
- `users-module/api/internal/handlers/self.go` deleted.
- `users-module/api/openapi.yaml` references the core fragment (or inlines the core routes with a doc pointer).
- Integration test covers `PUT /v1/self` end-to-end with audit verification.

## Hard rules
- **No behavioral regression.** Every existing endpoint behaves identically from the client's perspective.
- **Single auth middleware path.** users-module's existing `RequireAuth` must populate a context value that core's `PrincipalExtractor` can read.
- **Single tx for admin user-create.** The flow must remain atomic across users + entities + auth_local.

## Tasks
- 5.1 Require core-api in go.mod
- 5.2 Principal adapter
- 5.3 Audit writer conforms to core interface
- 5.4 Mount core router in main.go
- 5.5 Rework admin user-create
- 5.6 Delete self.go
- 5.7 Fix auditlog.go UUID resolution
- 5.8 Integration test

## How to verify
- `go build ./...` in users-module/api exits 0.
- `make test` in users-module/api exits 0.
- `curl -H "Authorization: Bearer <admin jwt>" http://localhost:8080/v1/self` returns caller profile.
- `curl -X PUT -H "Authorization: ..." -H "Content-Type: application/json" -d '{"given_name":"X"}' /v1/self` updates DB and writes audit row.
- audit_log has actor=admin_user_id, target=admin_entity_id, op=update, resource=natural_person.

## Notes
- If the principal adapter drifts from the existing `auth.UserContext` shape, update the adapter — don't change `UserContext`.
- Removing `/v1/self` registrations from users-module must be done in the same commit as mounting core's router to avoid a broken intermediate state.
