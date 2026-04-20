# Phase 7, Task 1 — Full make test

## Acceptance
- `cd core-module/model && make test` → pass.
- `cd core-module/api && make test` → pass.
- `cd core-module/gui && npm run typecheck && npm test` → pass (npm test may be a no-op if no tests exist).
- `cd users-module/model && make test` → pass.
- `cd users-module/api && make test` → pass.
- `cd users-module/gui && npm run build && npm run lint` → pass.
- `make test` at repo root aggregates — same result.

## How to verify
Run the commands. Any failure is a blocker.

## Notes
- Keep a brief `report.phase7.md` listing what was run and the outcome.
