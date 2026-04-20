# Phase 1, Task 1 — Create top-level go.work

## Context
All Go modules in this repo (`core-module/model`, `core-module/api`, `users-module/model`, `users-module/api`) must be stitched via a committed `go.work` file so local edits are picked up without republish.

## Acceptance
- File `user-components/go.work` exists and is committed.
- Contents:
  ```
  go 1.23
  
  use (
      ./core-module/model
      ./core-module/api
      ./users-module/api
      ./users-module/model
  )
  ```
- The go-version line matches the highest version used across the module go.mod files (check `users-module/api/go.mod`).
- `users-module/go.work` (pre-existing) must be deleted or its contents merged here — only one go.work for the whole repo.

## How to verify
- `go work sync` at repo root exits 0.
- `go build ./...` from `users-module/api/` still succeeds (no regression).

## Notes
- If `users-module/go.work` exists with different module paths, reconcile before writing the new file. The old one likely only listed users-module paths.
