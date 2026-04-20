# Phase 3, Task 10 — migrate.hash on composed dir

## Context
The composed `schema/migrations/` dir needs its own `atlas.sum`. Generated at build time, not committed.

## Acceptance
- `make compose` in users-module/model produces a fresh `schema/migrations/atlas.sum`.
- `atlas migrate validate --dir file://schema/migrations` exits 0.
- `schema/atlas.sum` is covered by the `/schema/` gitignore — not committed.

## How to verify
- After `make compose`, `ls schema/migrations/atlas.sum` exists.
- `git status` does not show `schema/` as tracked.

## Notes
- CI should run `make compose && make migrate.hash` and assert no stderr noise.
