# Phase 3, Task 6 — Add compose Makefile target

## Context
Atlas and sqlc each want a single flat schema dir. The `compose` target assembles core + users-module migrations into `users-module/model/schema/migrations/` (gitignored).

## Acceptance
Add to `users-module/model/Makefile`:

```makefile
CORE_DIR := $(shell go list -m -f '{{.Dir}}' github.com/moduleforge/core-model 2>/dev/null || echo ../../core-module/model)

.PHONY: compose
compose: ## Assemble composed migrations dir
	@rm -rf schema/migrations
	@mkdir -p schema/migrations
	@cp $(CORE_DIR)/migrations/*.sql schema/migrations/
	@cp migrations/*.sql schema/migrations/
	@atlas migrate hash --dir file://schema/migrations > /dev/null

build: compose
migrate.up: compose
migrate.status: compose
migrate.hash: compose
```

Also add `/schema/` to `users-module/model/.gitignore`.

## How to verify
- `cd users-module/model && make compose && ls schema/migrations/` shows 6 core + 10 users-module files.
- Re-running `make compose` is idempotent (output identical).
- `schema/` is untracked by git.

## Notes
- The `go list -m` fallback path handles the case where go.mod resolution fails (e.g. during a fresh clone before `go work sync`).
- If atlas fails on the composed dir, check for hash drift and re-run `make compose`.
