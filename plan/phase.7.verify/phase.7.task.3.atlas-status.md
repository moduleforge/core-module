# Phase 7, Task 3 — atlas migrate status

## Acceptance
After `make dev.start` has applied migrations:
- `atlas migrate status --dir file://schema/migrations --url "$DB_URL"` in `users-module/model/` shows all migrations applied.
- Listed order starts with `0000_helpers`, then 0001–0005, then 0100+.
- No pending migrations.

## How to verify
Run the command. Capture output into `report.phase7.md`.

## Notes
- If atlas reports order issues, check the compose step is producing the expected flat dir.
