# Phase 7, Task 2 — dev.start smoke

## Acceptance
- `make dev.start` from `users-module/` brings up Postgres + api + gui.
- Local login succeeds (Authelia or local credentials — whatever phase 9 of users-module left working).
- Navigate to `/profile`, edit given_name, save, reload — value persists.
- Admin: create a user via `/admin/users` (if UI supports) or via POST /v1/users — user appears in list.
- Admin: edit a user — profile fields update.

## How to verify
Manual walkthrough; log results in `report.phase7.md`.

## Notes
- If a local volume was populated under the old schema, it's invalid now — `make dev.reset` (or manually `docker compose down -v`) to start fresh. Expected once, per Phase 3's notes.
