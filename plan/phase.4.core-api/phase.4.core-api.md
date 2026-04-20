# Phase 4 — Build core-module/api

## Goal
Implement the two-layer Go API for core-module: a `service/` package with tx-accepting entity CRUD (consumable directly for cross-module transactions), and an `httpapi/` package exposing a mountable chi subrouter. Establish the audit + principal injection interfaces so consumers (users-module today, future apps later) wire in their own auth and audit implementations.

## Preconditions
- Phase 2 complete: core-module/model/db/ generated.
- Phase 3 complete: users-module/api building against core-model for the 5 tables (but still using its own handlers — this phase builds the replacement API).

## Outputs
- `core-module/api/audit/audit.go` — `Writer` interface.
- `core-module/api/service/principal.go` — `Principal` struct + `PrincipalExtractor` interface.
- `core-module/api/service/entity.go`, `legal_entity.go`, `natural_person.go`, `corporation.go`, `service_account.go` — CRUD services with tx semantics.
- `core-module/api/httpapi/router.go` — `NewRouter(deps Deps) chi.Router`.
- `core-module/api/httpapi/{self,natural_persons,corporations,service_accounts,entities}.go` — handlers.
- `core-module/api/httpapi/response.go` — JSON/Error helpers.
- `core-module/api/service/*_test.go` — unit tests per service.
- `core-module/api/httpapi/*_test.go` — auth-path tests using `httptest`.
- `core-module/api/openapi.fragment.yaml` — OpenAPI 3.0.3 fragment describing `/entities/*` routes.

## Routes exposed (final)
| Verb | Path | Purpose | Auth |
|---|---|---|---|
| GET  | `/entities/self` | Caller's entity + subtype | authenticated |
| PUT  | `/entities/self` | Update caller's profile (natural_person / corporation fields) | authenticated |
| POST | `/entities/natural-persons` | Create | admin |
| GET  | `/entities/natural-persons/{uuid}` | Read | authenticated (owner or admin) |
| PUT  | `/entities/natural-persons/{uuid}` | Update | authenticated (owner or admin) |
| POST | `/entities/corporations` | Create | admin |
| GET  | `/entities/corporations/{uuid}` | Read | authenticated |
| PUT  | `/entities/corporations/{uuid}` | Update | admin |
| POST | `/entities/service-accounts` | Create | admin |
| GET  | `/entities/service-accounts/{uuid}` | Read | admin |
| PUT  | `/entities/service-accounts/{uuid}` | Update | admin |
| DELETE | `/entities/{uuid}` | Archive | admin |
| GET  | `/entities/{uuid}` | Read any entity + resolve subtype | authenticated |

(All are mounted under whatever prefix the consumer chooses — typically `/v1`.)

## Hard rules
- **No dependency on users-module.** core-module/api imports only core-model + stdlib + third-party deps.
- **No auth middleware in core-module.** Consumer middleware populates context; core's `PrincipalExtractor` reads it.
- **Services accept `pgx.Tx` or `db.DBTX`** so consumer code can compose multi-module transactions.
- **Audit is injected.** Services call the `audit.Writer` interface; no direct DB access to audit_log (which lives in users-module).
- **Service method signatures match the shape of the handler logic currently in `users-module/api/internal/handlers/{self,users}.go`.** Move logic, don't re-derive.

## Tasks
- 4.1 audit.Writer interface
- 4.2 Principal + PrincipalExtractor
- 4.3 Entity services (5 types)
- 4.4 httpapi router + handlers
- 4.5 Unit tests
- 4.6 OpenAPI fragment

## How to verify
- `cd core-module/api && make test` exits 0.
- `go build ./...` in core-module/api exits 0.
- `go vet ./...` exits 0.
- The openapi fragment validates (`npx @redocly/cli lint openapi.fragment.yaml`).

## Notes
- Keep the handler layer thin: decode → authorize → call service → encode.
- Service method `UpdateByEntityUUID` takes a `Principal` argument so business rules (e.g., "non-admin can only update own profile") can be checked in the service too — defense in depth.
