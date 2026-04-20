# Phase 3 — Wire model into users-module, drop duplicates

## Goal
Make users-module/api consume `github.com/moduleforge/core-model` for the 5 extracted tables; delete the duplicate migrations and queries from users-module/model; renumber the users-module-specific migrations to 0100+; wire the compose pipeline so atlas + sqlc still see a single flat migration dir.

## Preconditions
- Phase 2 complete; core-module/model builds clean and has its own atlas.sum.
- Top-level `go.work` (Phase 1) stitches core-module/model.

## Outputs
- `users-module/api/go.mod` requires `github.com/moduleforge/core-model`.
- `users-module/api/internal/...` call sites import coredb for core tables.
- `users-module/model/migrations/` no longer contains 0000–0005.
- `users-module/model/migrations/0100_users.sql` … `0109_drop_provider_enabled_json.sql` (renumbered).
- `users-module/model/queries/` no longer contains the 5 core query files.
- `users-module/model/Makefile` has a `compose` target; `build` and `migrate.*` depend on it.
- `users-module/model/atlas.hcl` + `sqlc.yaml` point at `./schema/migrations` (composed dir).
- `users-module/model/.gitignore` includes `/schema/`.
- `users-module/model/sqlc.yaml` sets `omit_unused_structs: true` so core-table types don't get re-emitted.
- `users-module/api/Dockerfile` updated to pull core migrations via `go mod download` and copy from `$GOMODCACHE`.
- Composed migration hash is idempotent (`make compose && make migrate.hash` stable).

## Hard rules
- **No prod DB exists** — renumbering is safe.
- Do NOT commit `users-module/model/schema/` (gitignored).
- Preserve exact SQL content during renumber — only filenames change.
- The sqlc output in `users-module/model/db/` must lose its core-table structs after regen.

## Tasks
- 3.1 go.mod require core-model
- 3.2 Switch call sites to coredb
- 3.3 Delete 0000–0005 in users-module/model
- 3.4 Renumber 0006–0015 → 0100–0109 (git mv)
- 3.5 Delete 5 core query files in users-module/model
- 3.6 Add compose Makefile target
- 3.7 Point atlas.hcl + sqlc.yaml at composed dir
- 3.8 Regenerate sqlc + set omit_unused_structs
- 3.9 Update Dockerfile
- 3.10 migrate.hash on composed dir
- 3.11 make test green in both modules

## How to verify
- `cd users-module/api && go build ./...` exits 0.
- `cd users-module/model && make compose && make build` exits 0.
- `ls users-module/model/schema/migrations/` shows 6 core files + 10 users-module files in lexical order (0000–0005 then 0100–0109).
- `atlas migrate validate` on composed dir exits 0.
- `grep -r "users-module/model/db" users-module/api/internal/` — only users-module-specific imports, no core-table types.
- `grep -r "natural_persons\|legal_entities\|entities " users-module/model/queries/` returns nothing.
- `make test` in users-module/api and users-module/model both exit 0.

## Notes
- After this phase, entity CRUD in users-module still goes through ad-hoc coredb calls from handlers — this is transitional. Phase 4 + 5 replace these with core-module service calls.
- The compose target should be shell-portable (no bash-only features) since it runs in CI.
