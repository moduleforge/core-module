# Phase 3, Task 1 — Require core-model in users-module/api

## Context
users-module/api must pick up `github.com/moduleforge/core-model` so handlers can import `coredb`.

## Acceptance
- `users-module/api/go.mod` has `require github.com/moduleforge/core-model v0.0.0-local` (or a pseudo-version; go.work overrides to local).
- `go.work` at repo root (Phase 1) already lists `./core-module/model`.
- `go build ./...` in users-module/api exits 0 without a downstream network fetch.

## How to verify
- `go list -m github.com/moduleforge/core-model` in users-module/api resolves to the local path.
- `go build ./...` works offline.

## Notes
- Use `go mod edit -require` or hand-edit; don't run `go get` (would try to resolve a real version).
