# Phase 3, Task 11 — make test green in both modules

## Acceptance
- `cd users-module/model && make test` → pass.
- `cd users-module/api && make test` → pass.
- `cd core-module/model && make test` → pass (no tests).
- No flaky failures from renumbered migrations vs. existing fixtures.

## How to verify
Run the three make test invocations; all exit 0.

## Notes
- If fixture SQL files reference `0006_` migration names, update them.
- If a test sets up DB by applying `users-module/model/migrations/*.sql` directly, update it to use composed dir.
