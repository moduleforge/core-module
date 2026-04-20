# core-module — TODO

Status: `[ ]` not started · `[~]` in progress · `[x]` done · `[!]` blocked

- [x] **Phase 1 — Bootstrap core-module skeleton** (depends on: none)
  - [x] 1.1 Create top-level `user-components/go.work`
  - [x] 1.2 Scaffold `core-module/model/` (go.mod, atlas.hcl, sqlc.yaml, Makefile, .gitignore)
  - [x] 1.3 Scaffold `core-module/api/` (go.mod, Makefile, empty audit/service/httpapi packages)
  - [x] 1.4 Scaffold `core-module/gui/` (package.json, tsconfig, tsup config, empty src/index.ts)
  - [x] 1.5 Root `Makefile` with `link-core`, `unlink-core`, aggregated `build` / `test`
  - [x] 1.6 Root `README.md` documenting go.work + yalc onboarding

- [x] **Phase 2 — Extract model** (depends on: 1)
  - [x] 2.1 Copy migrations 0000–0005 + queries into core-module/model
  - [x] 2.2 sqlc build → core-module/model/db/
  - [x] 2.3 atlas migrate hash → atlas.sum
  - [x] 2.4 Compile-check core-module/model

- [x] **Phase 3 — Wire model into users-module, drop duplicates** (depends on: 2)
  - [x] 3.1 Add `require github.com/moduleforge/core-model` to users-module/api/go.mod
  - [x] 3.2 Switch users-module/api call sites to coredb for core tables
  - [x] 3.3 Delete 0000–0005 from users-module/model/migrations
  - [x] 3.4 Renumber users-module migrations 0006–0015 → 0100–0109 (git mv)
  - [x] 3.5 Delete entity-table query files from users-module/model/queries
  - [x] 3.6 Add `compose` Makefile target in users-module/model
  - [x] 3.7 Point users-module/model atlas.hcl + sqlc.yaml at composed schema/migrations
  - [x] 3.8 Regenerate sqlc (`omit_unused_structs: true`)
  - [x] 3.9 Update users-module/api/Dockerfile (go mod download + copy from $GOMODCACHE)
  - [x] 3.10 `make migrate.hash` on composed dir
  - [x] 3.11 `make test` green in both modules

- [x] **Phase 4 — Build core-module/api** (depends on: 2)
  - [x] 4.1 Define `core-module/api/audit` package (Writer interface)
  - [x] 4.2 Define `core-module/api/service/principal.go` (Principal + PrincipalExtractor)
  - [x] 4.3 Implement entity services in `core-module/api/service/` (tx-accepting CRUD for each)
  - [x] 4.4 Implement `core-module/api/httpapi` (NewRouter + handlers)
  - [x] 4.5 Unit tests (service tx/audit, httpapi auth paths) — coverage service 77.1%, httpapi 71.7%
  - [x] 4.6 OpenAPI fragment `core-module/api/openapi.fragment.yaml`

- [ ] **Phase 5 — Wire users-module/api to consume core-module/api** (depends on: 4 and 3)
  - [ ] 5.1 Add `require github.com/moduleforge/core-api` to users-module/api/go.mod
  - [ ] 5.2 `users-module/api/internal/auth/core_adapter.go` (PrincipalExtractor impl)
  - [ ] 5.3 Make users-module/api/internal/audit satisfy core's Writer interface
  - [ ] 5.4 main.go: construct core services + router; mount at /v1; remove /v1/self routes
  - [ ] 5.5 Rework handlers/users.go admin-create to call core service inside pgx tx
  - [ ] 5.6 Delete handlers/self.go
  - [ ] 5.7 Update handlers/auditlog.go to resolve entity UUIDs via coredb or core service
  - [ ] 5.8 Integration test: PUT /v1/self end-to-end, audit actor correct

- [ ] **Phase 6 — Extract UI components into core-module/gui** (depends on: 1)
  - [ ] 6.1 Extract ProfileEditor from users-module/gui/src/app/profile/page.tsx
  - [ ] 6.2 Extract NaturalPersonForm, CorporationForm, ServiceAccountForm
  - [ ] 6.3 Extract shadcn primitives (Button, Input, Label, Card, Badge, Alert) into core-module/gui/src/ui/
  - [ ] 6.4 tsup build (ESM + types)
  - [ ] 6.5 yalc publish
  - [ ] 6.6 users-module/gui consumes via yalc; replace inline code with imports
  - [ ] 6.7 Update users-module/gui Tailwind content glob
  - [ ] 6.8 Visual smoke test (/profile, /admin/users/[uuid])

- [ ] **Phase 7 — Verification + cleanup** (depends on: all)
  - [ ] 7.1 make test green in all six packages
  - [ ] 7.2 make dev.start full smoke
  - [ ] 7.3 atlas migrate status shows 0000–0005 then 0100–0109
  - [ ] 7.4 grep confirms no natural_persons/legal_entities in users-module/api/internal/handlers
  - [ ] 7.5 No sqlc-generated delta in users-module/model after clean rebuild
  - [ ] 7.6 Audit log entries correct for profile edits via core handlers
  - [ ] 7.7 Update users-module CLAUDE.md / summary.md noting core-module dependency
  - [ ] 7.8 Archive/mark-superseded any prior schema-only plan content

## Reports

Drop progress notes into `report.<N>.<topic>.md` in this directory as work proceeds.
