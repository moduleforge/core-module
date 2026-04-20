# Phase 1, Task 2 — Scaffold core-module/model

## Context
`core-module/model` will hold the generic entity schema, atlas migrations, sqlc queries, and generated `db/` package. This task sets up the skeleton; migrations/queries land in Phase 2.

## Acceptance
- `core-module/model/go.mod` — module `github.com/moduleforge/core-model`, same Go version as users-module/api.
- `core-module/model/atlas.hcl` — mirror `users-module/model/atlas.hcl` but with `migration { dir = "file://migrations" }` pointing to `./migrations`.
- `core-module/model/sqlc.yaml` — v2 config: engine postgresql, schema `./migrations`, queries `./queries`, output `./db`, package `db`, with `emit_interface: true`, `emit_json_tags: true`, `emit_prepared_queries: false`. Package import path in overrides: `github.com/moduleforge/core-model/db`.
- `core-module/model/Makefile` — canonical targets per existing conventions (`build`, `test`, `lint`, `lint-fix`, `migrate.new`, `migrate.up`, `migrate.status`, `migrate.hash`). Model up `users-module/model/Makefile` for the target layout.
- `core-module/model/.gitignore` — ignore binary artifacts, `db/` if we regenerate on demand (prefer committing `db/` per users-module's convention — match whatever users-module/model does).
- `core-module/model/migrations/.keep` and `core-module/model/queries/.keep` — empty dirs present in git.
- `core-module/model/README.md` — one-paragraph description + install/build/migrate commands.

## How to verify
- `cd core-module/model && make build` exits 0 (sqlc produces an empty `db/db.go` since no queries).
- `atlas migrate validate --dir file://migrations` runs (empty dir is acceptable).

## Reference
- Existing model scaffold: `users-module/model/` — use as template.
- Make convention memory: project root memory `feedback_make_conventions.md`.
