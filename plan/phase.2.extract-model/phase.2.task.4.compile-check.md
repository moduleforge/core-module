# Phase 2, Task 4 — Compile-check

## Context
End-of-phase gate: core-module/model builds standalone with no dependency on users-module.

## Acceptance
- `cd core-module/model && go build ./...` exits 0.
- `go vet ./...` exits 0.
- `make test` exits 0 (no tests yet — passes empty).
- `grep -r "users-module" core-module/model/` returns nothing (no stale imports).

## How to verify
All acceptance commands pass in sequence.

## Notes
- If the Go version mismatch shows up here, reconcile go.mod versions between modules.
