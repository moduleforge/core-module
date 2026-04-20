# Phase 7 — Verification + cleanup

## Goal
Final gate: confirm everything builds, tests pass, runs, and docs are up to date. Mark the schema-only plan superseded.

## Outputs
- Full test suite green across all six packages.
- `make dev.start` smoke log showing a login + profile edit round-trip.
- Updated root + sub-module docs.

## Tasks
- 7.1 Full make test
- 7.2 dev.start smoke
- 7.3 atlas migrate status
- 7.4 grep sanity checks
- 7.5 sqlc diff check
- 7.6 audit trail verification
- 7.7 CLAUDE.md / summary.md updates
- 7.8 Archive prior schema-only plan content

## How to verify
All per-task acceptance in this phase dir.

## Notes
- If anything in 7.x fails, fix in place or open a task-branch for the fix — don't claim the phase done while tests are red.
