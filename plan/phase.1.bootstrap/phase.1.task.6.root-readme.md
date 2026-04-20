# Phase 1, Task 6 — Root README

## Context
A short onboarding README at the repo root explaining the bundle and local dev setup.

## Acceptance
`user-components/README.md` covering:

1. **What this is** — 2 sentences. "Bundle of user-management components split into a generic core-module (entity identity foundation) and users-module (auth, users, multi-tenancy). See CLAUDE.md for the overall plan."
2. **Requirements** — Go 1.23+, Node 20+, Docker, GNU make 4+, yalc (`npm i -g yalc`).
3. **First-time setup**:
   ```sh
   go work sync
   make link-core     # builds core-module/gui and yalc-links into users-module/gui
   cd users-module && make dev.start
   ```
4. **Module map** — a tree showing `core-module/{model,api,gui}` and `users-module/{model,api,gui,deploy}`, one-liner each.
5. **When you change core-module** — re-run `make link-core`; Go changes are picked up automatically via go.work.
6. **Links** — pointers to `core-module/plan/summary.md`, `users-module/plan/summary.md`, and `CLAUDE.md`.

## How to verify
- README renders cleanly (spot-check markdown).
- Commands in "First-time setup" actually work on a fresh clone.

## Notes
- Keep it short (under 150 lines). Deep docs live in the plan folders.
