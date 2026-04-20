# core-model

Generic entity-identity foundation for the moduleforge platform. This module
holds the Atlas versioned migrations and sqlc-generated Go queries for the
base entity hierarchy: `entities`, `legal_entities`, `natural_persons`,
`corporations`, and `service_accounts`. It is extracted from users-module so
that any downstream module can depend on a stable, shared identity schema
without pulling in users-module application logic.

## Layout

- `migrations/` — Atlas versioned migration files (`.sql`)
- `queries/` — sqlc query files (`.sql`), one per concept
- `db/` — sqlc-generated Go code (do not edit)
- `atlas.hcl` — Atlas environment configuration
- `sqlc.yaml` — sqlc v2 configuration

## Prerequisites

- [Atlas CLI](https://atlasgo.io) — `curl -sSf https://atlasgo.sh | sh`
- [sqlc](https://docs.sqlc.dev) — `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0`
- Running Postgres instance (local: `docker compose up -d` from `deploy/local/`)

## Make targets

```
make build            # generate Go from sqlc queries (default)
make gen              # same as build
make verify           # atlas validate + sqlc compile
make migrate.new NAME=foo  # create a new migration file
make migrate.up       # apply pending migrations
make migrate.status   # show migration status
make migrate.hash     # recalculate migration integrity hash
make test.integration # apply migrations against DATABASE_URL
make lint             # atlas migrate lint (latest migration)
make clean            # remove generated Go code
```

All targets default `DATABASE_URL` to `postgresql://core:core@localhost:5432/core?sslmode=disable`.
