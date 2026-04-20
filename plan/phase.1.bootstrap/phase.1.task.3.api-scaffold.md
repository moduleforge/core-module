# Phase 1, Task 3 — Scaffold core-module/api

## Context
`core-module/api` will hold the audit interface, entity services, and the mountable chi subrouter. Phase 4 fills these packages; this task creates the skeleton.

## Acceptance
- `core-module/api/go.mod` — module `github.com/moduleforge/core-api`, same Go version as users-module/api. Require the following (matching users-module/api versions where possible):
  - `github.com/go-chi/chi/v5`
  - `github.com/google/uuid`
  - `github.com/jackc/pgx/v5`
  - `github.com/moduleforge/core-model` (resolved via go.work)
- `core-module/api/Makefile` — canonical targets (`build`, `test`, `lint`, `lint-fix`). Model up `users-module/api/Makefile`.
- Placeholder packages with a single `doc.go`:
  - `core-module/api/audit/doc.go` — `// Package audit defines the Writer interface core services use to record changes.`
  - `core-module/api/service/doc.go` — `// Package service exposes tx-aware entity CRUD for consumer apps.`
  - `core-module/api/httpapi/doc.go` — `// Package httpapi exposes a mountable chi subrouter serving core entity routes.`
- No `cmd/` — this module is a library.

## How to verify
- `cd core-module/api && go build ./...` exits 0.
- `go list -m all` resolves `core-model` locally via go.work.

## Reference
- Existing api structure: `users-module/api/internal/` — mirror the idiom where useful.
