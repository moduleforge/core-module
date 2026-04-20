# Phase 7, Task 5 — sqlc idempotence

## Acceptance
- `cd users-module/model && make clean && make build && git status` → `users-module/model/db/` unchanged (no diff from committed state).
- `cd core-module/model && make clean && make build && git status` → same.

## How to verify
Run the commands.

## Notes
- A diff here means non-determinism in code generation or a config drift. Fix before sign-off.
