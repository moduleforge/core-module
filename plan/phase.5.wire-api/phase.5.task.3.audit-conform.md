# Phase 5, Task 3 — users-module audit.Writer conforms to core interface

## Context
core-module's `audit.Writer` signature was chosen to match users-module's existing `audit.Writer`. If signatures are byte-identical, Go's structural typing lets users-module's writer be passed directly wherever core's interface is needed. Confirm or reconcile.

## Acceptance
- `users-module/api/internal/audit.Writer` methods signature:
  ```go
  Write(ctx context.Context, op, resource string, targetEntityID *int64, before, after any) error
  ```
- If there's any drift from `core-api/audit.Writer`, fix users-module to match.
- Compile-time assertion: add `var _ coreaudit.Writer = (*audit.Writer)(nil)` in a `_ = ...` line in `users-module/api/cmd/server/main.go` (or a dedicated `doc.go`) to catch future drift.

## How to verify
- `go build ./...` exits 0.
- The structural assertion compiles.

## Notes
- If users-module's writer has extra methods (e.g. flush), that's fine — Go interfaces only require a subset.
