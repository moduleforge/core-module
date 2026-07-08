# mod-core

`mod-core` is the foundational domain module for the ModuleForge platform. It owns the canonical entity-identity schema and the Go library packages that every other ModuleForge module builds on.

## What it provides

- **Entity schema** — Postgres tables for `entities`, `legal_entities`, `natural_persons`, `corporations`, and `service_accounts`, plus the `types` registry that drives the entity-typing system. UUID ownership and assignment live here.
- **Go API library** (`github.com/moduleforge/core-api`) — service layer (entity CRUD), HTTP handlers, observer/fan-out pattern, AES-256-GCM field cipher, authorization and operation-context contracts, and display-renderer registry.
- **Go model library** (`github.com/moduleforge/core-model`) — sqlc-generated query code for the entity schema, consumed by core-api and by peer modules that need direct DB access.
- **GUI component library** (`@moduleforge/core-gui`) — shared TypeScript/React components published via yalc for local development.

## Sub-projects

| Directory | Language | Package |
|-----------|----------|---------|
| `model/` | Go / SQL | `github.com/moduleforge/core-model` |
| `api/` | Go | `github.com/moduleforge/core-api` |
| `gui/` | TypeScript / React | `@moduleforge/core-gui` |

## Quick start

```sh
# Build all sub-projects
make build

# Run tests
make test
```

See [AGENTS.md](./AGENTS.md) for full environment setup, build commands, sqlc codegen workflow, and package descriptions. See [docs/mf-standards/manifest-spec.md](./docs/mf-standards/manifest-spec.md) for the module manifest specification.
