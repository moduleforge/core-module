# AGENTS.md — mod-core

This file is the canonical reference for contributors and AI agents working on this codebase. It covers purpose, package layout, build and test commands, code-generation workflow, and conventions. Claude Code-specific guidance is in [`.claude/settings.local.json`](./.claude/settings.local.json).

## Project overview

`mod-core` is the foundational domain model for the ModuleForge platform. It defines the `entities` and `legal_entities` tables, owns UUID generation and assignment for every entity in the system, and exposes the service-layer abstractions (Entity interface, entity types, observers, cipher) that peer modules depend on.

The module ships three sub-projects:

- `model/` — Postgres schema (goose migrations) and sqlc-generated Go query code (`github.com/moduleforge/core-model`)
- `api/` — Go HTTP handlers, service layer, and all cross-cutting library packages (`github.com/moduleforge/core-api`)
- `gui/` — TypeScript/React component library (`@moduleforge/core-gui`), published via yalc for local development

See [docs/mf-standards/manifest-spec.md](./docs/mf-standards/manifest-spec.md) for the authoritative manifest specification that describes how mod-core integrates with mfgen and the application composition layer.

## Prerequisites

| Tool | Min version | Purpose |
|---|---|---|
| Go | 1.21+ | `model/` and `api/` sub-projects |
| Bun | 1.x | `gui/` component library and root aggregator |
| GNU make | 4.x | Build orchestration (use `gmake` on macOS if needed) |
| sqlc | v1.28.0 | Go query code generation (`model/`) |
| goose | latest | Database migrations (`model/`) |
| Docker | any recent | Ephemeral shadow Postgres for `make lint` in `model/` |

> macOS ships BSD make. Install GNU make with `brew install make` and invoke as `gmake`, or ensure `/usr/local/bin` is before `/usr/bin` in `PATH`.

## Build commands

```sh
make build           # build model (sqlc generate), api (go build ./...), and gui
make test            # unit-test api; typecheck gui (model has no unit tests)
make clean           # remove build artifacts from all sub-projects
```

Sub-projects can be built directly:

```sh
cd model && make build       # sqlc generate (idempotent if sqlc not installed and db/ exists)
cd api   && make build       # go build ./...
cd gui   && bun run build    # Vite/tsup bundle
```

## Test commands

```sh
make test                    # unit tests (api) + gui typecheck
cd api && make test          # go test ./...
cd api && make lint          # go vet ./... + gofmt check
cd api && make lint-fix      # gofmt -w .
cd model && make verify      # goose validate + sqlc compile
cd model && make lint        # apply migrations to ephemeral Postgres (requires Docker)
```

## Database migrations

Migrations live in `model/migrations/` and are managed with goose. They run automatically on host-application startup. To run or inspect manually:

```sh
cd model
make migrate.up                       # apply pending migrations to DATABASE_URL
make migrate.status                   # show current migration status
make migrate.new NAME=add_foo_column  # create a new numbered migration file
```

The default `DATABASE_URL` is `postgresql://core:core@localhost:5432/core?sslmode=disable`. Override by setting the environment variable before invoking make.

**No cross-module range coordination.** Each module — including mod-core — numbers its own migrations independently starting from 1, tracked in its own dedicated `goose_db_version_<module>` table (mod-core's is `goose_db_version_core`), so no two modules can collide regardless of numbering. Cross-module ordering, where required, is declared via the module manifest's `migrations.after` edge; mod-core is the star-topology root and needs no `after:` edge. Migrations are applied automatically at host-application startup via `model/migrations.Migrate`. See [`docs/mf-standards/manifest-spec.md`](./docs/mf-standards/manifest-spec.md) §5 for the full convention.

## Code generation (sqlc)

`model/db/` is generated from SQL in `model/queries/`. After editing a query file:

```sh
cd model && sqlc generate
# or equivalently:
cd model && make gen
```

The generated files are committed to the repo. `make clean` in `model/` removes `model/db/` — restore with `git checkout HEAD -- model/db/` if needed.

## Key types and packages

All packages below live under `api/` (`github.com/moduleforge/core-api`).

| Package | Purpose |
|---|---|
| `entity/` | `Entity` interface + concrete service-layer types (`LegalEntity`, `NaturalPerson`, `Corporation`, `ServiceAccount`). These are not DB row structs; they carry the resource slug, internal entity ID, and public UUID. |
| `observer/` | `MutationObserver` interface and `ObserverGroup` concrete type. `ObserverGroup` fans out to N observers in parallel with configurable error policy (`PolicyPropagate` / `PolicySwallow`). Provides `Observe`, `MustObserve`, `MayObserve` per-call-site policy overrides, and `ObserveAfterCommit` for post-tx hooks. |
| `fieldcrypto/` | Public façade for the AES-256-GCM field cipher. Reads `CORE_FIELD_KEY_HEX` from the environment. Implementation lives in `internal/fieldcrypto/`; this package re-exports only what callers outside core need. |
| `service/` | `Services` aggregate wrapping `EntityService`, `NaturalPersonService`, `CorporationService`, and `ServiceAccountService`. Constructed via `service.New(...)` and passed to `httpapi.NewRouter`. Its sentinel errors (`ErrNotFound`, `ErrForbidden`, `ErrInvalidInput`, `ErrConflict`) alias the canonical `apiresp` sentinels below. |
| `authz/` | `Authorizer` and `OpResolver` interfaces. Implementations are consumer-supplied (e.g. mod-authz); this package defines only the contracts. |
| `types/` | `Resolver` that maps `fundamental_type_slug` strings to internal type IDs. Populated once at startup from the `types` table; safe to cache for process lifetime. |
| `display/` | Per-`(typeSlug, fieldName)` renderer registry. Modules register renderers for their own entity kinds; the registry dispatches at render time. |
| `opctx/` | Typed context accessors for `ActorEntityID`, `SudoActorEntityID`, and `RequestID`. Set by HTTP middleware; consumed by service methods and the `Authorizer`. |
| `txhelper/` | Thin transaction-helper utilities used by service methods to open and commit pgx transactions. |
| `apiresp/` | Canonical shared response/error contract: the sentinel set (`ErrUnauthenticated`, `ErrForbidden`, `ErrNotFound`, `ErrInvalidInput`, `ErrConflict`), `WriteJSON` (bare success encoder), `WriteError` (sentinel→status/code mapper and nested `{error:{code,message,details?}}` envelope encoder; logs 5xx via `slog` with request context and never leaks raw error text in the response body), and `InvalidInput(...)` (field-error builder producing the `FieldError`-carrying error `WriteError` surfaces as `details`). Every ModuleForge module is expected to import this in place of a copy-pasted response trio; mod-core's own `httpapi` package dogfoods it. |
| `httpapi/` | Chi router wiring and HTTP handlers for `/entities/*` routes. `httpapi.NewRouter(deps)` returns a mountable `chi.Router`. Success and error responses are written via `apiresp.WriteJSON`/`apiresp.WriteError`. |

Model packages (`github.com/moduleforge/core-model`):

| Package | Purpose |
|---|---|
| `model/db/` | sqlc-generated Go query code. Do not edit by hand; regenerate with `make gen`. |
| `model/migrations/` | goose migration files for the full entity hierarchy (`entities`, `legal_entities`, `natural_persons`, `corporations`, `service_accounts`). |
| `model/queries/` | SQL query files consumed by sqlc, one file per entity kind. |

### gui/ error and toast toolkit

`@moduleforge/core-gui` (`gui/`) also ships a shared error/toast toolkit consumed by other modules' GUI packages:

- **`gui/src/lib/api-client.ts`** — wire types (`FieldError`, `ApiError`, `ApiErrorResponse`), the `ApiRequestError` class (`code`/`status`/`details`), and a shared typed `request()` fetch wrapper. `request()` synthesizes `network_error`/status 0 on transport failure, redirects on `401` (`unauthenticated`) unless `skipAuthRedirect` is set, and never redirects on `403` (`forbidden`). Token storage and the redirect action are injectable via `configureApiClient({ getToken, onUnauthenticated })`, with an SSR-safe browser default.
- **`<FieldError>`** (`gui/src/FieldError.tsx`) and **`<ErrorBanner>`** (`gui/src/ErrorBanner.tsx`) — presentational widgets binding a `FieldError`/`ApiError` to inline field-level and banner-level rendering; `<ErrorBanner>` wraps the existing `Alert` `destructive` variant.
- **`ToastProvider`/`useToast`** (`gui/src/lib/toast-context.tsx`, primitives in `gui/src/ui/toast.tsx`) — the transient/global toast surface, built on the existing `radix-ui` Toast primitive (no new dependency).
- **`useApiError(error, { fields? })`** (`gui/src/lib/use-api-error.ts`) — the single place the field/banner/toast routing rule lives: field-bound `details[]` entries go to `fieldErrors`; top-level `forbidden`/`not_found`/`conflict`/`invalid_input` (and unmatched `details[]` entries) go to `bannerError`; `network_error`/`internal_error`/rollback errors are dispatched to the toast provider; `unauthenticated` is never surfaced inline.

All of the above are exported from `gui/src/index.ts`.

## mountFromModule special case

mod-core's route block in `moduleforge.module.yaml` uses `innerMount: true`:

```yaml
routes:
  - prefix: /v1
    mountFromModule: corehttpapi.NewRouter
    innerMount: true
    args:
      - service:coreDeps
```

With `innerMount: true`, mfgen emits:

```go
r.Route("/v1", func(r chi.Router) {
    r.Mount("/", corehttpapi.NewRouter(coreDeps))
})
```

instead of the default:

```go
r.Mount("/v1", corehttpapi.NewRouter(coreDeps))
```

This distinction matters because other modules (e.g. mod-users) also register routes under `/v1`. A top-level `r.Mount("/v1", ...)` would create a chi prefix group that conflicts with subsequent `r.Route("/v1", ...)` calls in the same router, causing a duplicate-prefix panic. The `innerMount` flag makes the `/v1` group open (additive) rather than terminal (captured), so other modules can nest their own routes inside the same group.

**mod-core's `/v1` route group is the entry point for the outer route group that other modules nest inside.** When wiring the composition root, mod-core must be mounted before peer modules that share `/v1`.

## Manifest spec

The manifest specification — which defines the structure of `moduleforge.module.yaml` and the mfgen composition contract — lives at:

[`docs/mf-standards/manifest-spec.md`](./docs/mf-standards/manifest-spec.md)

This is the authoritative reference for `provides`, `requires`, `routes`, `observers`, and `migrations.range` fields.

## Conventions

- **Internal IDs are never exposed in HTTP responses** — always use the `uuid` field. The `Entity.EntityID()` value is for DB joins only.
- **Handlers are thin** — parse input, call one service method, shape response. No business logic in handlers.
- **Authorization is checked first** in every service method, before any data access.
- **Generated code (`model/db/`) is committed** and must not be edited by hand. Re-run `make gen` after any query change.
- **Migration numbers are module-local.** mod-core's `model/migrations/` numbers independently starting from 1, isolated in the `goose_db_version_core` tracking table; there is no cross-module range to collide with (see [Database migrations](#database-migrations) above).
- **Observer policy**: pass `observer.NewObserverGroup()` (no-op) when you don't need observation. Use `MustObserve` when an observer failure should abort the operation; use `MayObserve` when best-effort is acceptable.
