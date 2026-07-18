# core-model

Generic entity-identity foundation for the moduleforge platform. This module
holds the goose versioned migrations and sqlc-generated Go queries for the
base entity hierarchy: `entities`, `legal_entities`, `natural_persons`,
`corporations`, `service_accounts`, and `apps`. It is extracted from mod-users so
that any downstream module can depend on a stable, shared identity schema
without pulling in mod-users application logic.

See [../docs/mf-standards/architecture/db-considerations.md](../docs/mf-standards/architecture/db-considerations.md)
for the rationale behind the Postgres + goose choices.

## Layout

- `migrations/` — goose versioned migration files (`.sql`)
- `queries/` — sqlc query files (`.sql`), one per concept
- `db/` — sqlc-generated Go code (do not edit)
- `scripts/shadow-db-lint.sh` — ephemeral-Postgres lint runner
- `sqlc.yaml` — sqlc v2 configuration

## Prerequisites

- [goose](https://github.com/pressly/goose) — `go install github.com/pressly/goose/v3/cmd/goose@latest`
- [sqlc](https://docs.sqlc.dev) — `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0`
- Docker (for `make lint`'s ephemeral shadow Postgres)
- Running Postgres instance (local: `docker compose up -d` from `deploy/local/`)

## Make targets

```
make build            # generate Go from sqlc queries (default)
make gen              # same as build
make verify           # goose file-format validate + sqlc compile
make migrate.new NAME=foo  # create a new migration file
make migrate.up       # apply pending migrations
make migrate.status   # show migration status
make test.integration # apply migrations against DATABASE_URL
make lint             # apply all migrations to an ephemeral Postgres container
make clean            # remove generated Go code
```

All targets default `DATABASE_URL` to `postgresql://core:core@localhost:5432/core?sslmode=disable`.

## Migration file format

Every migration file uses the goose annotation format:

```sql
-- +goose Up

CREATE TABLE example (...);

-- +goose StatementBegin
CREATE FUNCTION example_fn() RETURNS TRIGGER AS $$
BEGIN
  -- multi-statement bodies need StatementBegin/End so goose
  -- treats the function as one statement.
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
```

Migrations are forward-only; no `-- +goose Down` sections are provided.
