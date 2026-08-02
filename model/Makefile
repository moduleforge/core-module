# model/ sub-project Makefile — SQL migrations (goose) + Go query generation (sqlc).

DATABASE_URL ?= postgresql://core:core@localhost:5432/core?sslmode=disable

.DEFAULT_GOAL := build

# ---------------------------------------------------------------------------
# Preflight — verify tools, set vars
# ---------------------------------------------------------------------------

.PHONY: preflight
preflight: ## Check required tools for model
	@printf "  model preflight: "
	@if command -v sqlc >/dev/null 2>&1; then \
		echo "sqlc=$$(sqlc version) ✓"; \
	elif [ -d db ]; then \
		echo "sqlc not found (using committed generated code) ✓"; \
	else \
		echo "FAIL"; \
		echo "ERROR: sqlc is not installed and no generated code exists in db/."; \
		echo "  Install: go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0"; \
		exit 1; \
	fi
	@if ! command -v goose >/dev/null 2>&1; then \
		echo "  model preflight: goose not found"; \
		echo "  Install: go install github.com/pressly/goose/v3/cmd/goose@latest"; \
	fi

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

.PHONY: build
build: preflight ## Generate Go code from queries
	@if ! ls queries/*.sql >/dev/null 2>&1; then \
		echo "model: no queries yet; skipping sqlc"; \
	elif command -v sqlc >/dev/null 2>&1; then \
		sqlc generate; \
	else \
		echo "model: using existing generated code in db/"; \
	fi

# ---------------------------------------------------------------------------
# Migration targets
# ---------------------------------------------------------------------------

.PHONY: migrate.new
migrate.new: ## Create a new migration file (usage: make migrate.new NAME=foo)
	@test -n "$(NAME)" || (echo "ERROR: NAME is required (e.g. make migrate.new NAME=create_entities)" && exit 1)
	goose -dir migrations -s create $(NAME) sql

.PHONY: migrate.up
migrate.up: ## Apply pending migrations to DATABASE_URL
	goose -dir migrations postgres "$(DATABASE_URL)" up

.PHONY: migrate.status
migrate.status: ## Show migration status
	goose -dir migrations postgres "$(DATABASE_URL)" status

# ---------------------------------------------------------------------------
# Code generation
# ---------------------------------------------------------------------------

.PHONY: gen
gen: ## Generate Go packages from sqlc queries
	sqlc generate

# ---------------------------------------------------------------------------
# Verification
# ---------------------------------------------------------------------------

.PHONY: verify
verify: ## Validate migration file format + compile-check queries (sqlc)
	goose -dir migrations validate
	sqlc compile

# ---------------------------------------------------------------------------
# Canonical targets (delegated from root Makefile)
# ---------------------------------------------------------------------------

.PHONY: test
test: test.unit ## Alias for test.unit

.PHONY: test.unit
test.unit: ## Run unit tests
	@echo "model: no unit tests (generated code)"

.PHONY: test.integration
test.integration: ## Run integration tests (migration apply against live DB)
	goose -dir migrations postgres "$(DATABASE_URL)" up

.PHONY: lint
lint: ## Apply migrations to an ephemeral shadow Postgres (replaces atlas migrate lint)
	@./scripts/shadow-db-lint.sh migrations

.PHONY: lint-fix
lint-fix: ## Lint-fix (no-op for model)
	@echo "model: no auto-fix available"

.PHONY: dev.start
dev.start: ## No runtime for model
	@echo "model: no runtime; nothing to start"

.PHONY: dev.stop
dev.stop: ## No runtime for model
	@echo "model: no runtime; nothing to stop"

.PHONY: clean
clean: ## Remove generated Go code (preserves migrations and queries)
	rm -rf db

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} \
		/^[a-zA-Z0-9_.\-]+:.*?## / { printf "  %-20s %s\n", $$1, $$2 }' \
		$(MAKEFILE_LIST) | sort
