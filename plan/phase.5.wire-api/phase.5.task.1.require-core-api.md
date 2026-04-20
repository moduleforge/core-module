# Phase 5, Task 1 — Require core-api in users-module/api

## Acceptance
- `users-module/api/go.mod` has `require github.com/moduleforge/core-api v0.0.0-local`.
- `go build ./...` succeeds with go.work resolving locally.

## How to verify
- `go list -m github.com/moduleforge/core-api` resolves to the local path.
- Build clean.
