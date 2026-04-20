# Phase 2, Task 3 — atlas migrate hash

## Context
Seal core-module/model's migration directory with `atlas.sum`.

## Acceptance
- `cd core-module/model && make migrate.hash` (or `atlas migrate hash --dir file://migrations`) produces `migrations/atlas.sum`.
- File is committed.

## How to verify
- `atlas migrate validate --dir file://migrations` exits 0 in core-module/model.
- `atlas migrate status --dir file://migrations --url "$DB_URL"` against a fresh DB lists 6 pending migrations.

## Notes
- If atlas complains about checksum drift, ensure no file was edited after the previous hash attempt.
