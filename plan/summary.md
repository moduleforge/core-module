# core-module — Plan summary

## Purpose

Extract the generic identity foundation — schema, Go API (service + HTTP), and React UI components — out of `users-module/` into a standalone `core-module/` that users-module (and any future consumer) imports. The result is a reusable package that ships its own profile-editing API and UI, so consumers don't re-implement entity CRUD or wrap services just to expose profile endpoints.

This supersedes the prior schema-only plan. The `entities → legal_entities → (natural_persons | corporations) + service_accounts` hierarchy, its Go service + HTTP handlers, and the React `<ProfileEditor>` / entity-form components all move.

## What moves vs. stays

**Moves into core-module:**
- Model: migrations `0000–0005` + matching sqlc queries (entities, legal_entities, natural_persons, corporations, service_accounts).
- Go API: `service/` (tx-aware CRUD for each entity type) + `httpapi/` (mountable chi subrouter exposing `/entities/self`, admin entity CRUD routes).
- UI: `<ProfileEditor>`, `<NaturalPersonForm>`, `<CorporationForm>`, `<ServiceAccountForm>`, plus shadcn primitives the components depend on.

**Stays in users-module (for now):**
- `users`, `auth_local`, `email_codes`, `password_resets`, `apps`, `apps_users`, `audit_log`, `oidc_config`, `oidc_providers` tables and their handlers.
- All auth flows (login, register, reset, email-code, OIDC start/callback, assume-identity).
- `audit_log` table + `audit.Writer` implementation. core-module defines its own `audit.Writer` **interface** (identical signature) so users-module's writer satisfies it structurally.

## Locked decisions

1. **UI is a component library**, not a Next.js app. Built with tsup; emits ESM + types. Consumer owns routes. Re-exports bundled shadcn primitives so consumers don't need a matching shadcn setup.
2. **Go API exposes two layers**: (a) `service/` package with tx-accepting methods for cross-module atomic operations, (b) `httpapi/` package with `NewRouter(deps) chi.Router` for drop-in mount.
3. **Authentication stays in users-module.** core-module defines a `PrincipalExtractor` interface; users-module implements it against its own auth context. No auth middleware in core-module.
4. **Audit log stays in users-module.** core-module declares an `audit.Writer` interface; users-module's writer implements it.
5. **Local package simulation**: `go.work` for Go, `yalc` for Node. Top-level `user-components/go.work` stitches all Go modules; `make link-core` in the root drives yalc publish/add.
6. **Migration numbering**: core reserves `0000–0099`; users-module migrations `0006–0015` renumber to `0100–0109`. Fresh DB only — no prod exists.
7. **Compose-at-build-time**: users-module/model still runs Atlas/sqlc over a single flat dir. A `compose` Makefile target copies core's migrations + users-module's into `users-module/model/schema/migrations/` (gitignored) before build/migrate.

## Architecture

### Module layout

```
user-components/
├── go.work                      # stitches all Go modules for local dev
├── Makefile                     # link-core, build, test aggregators
├── core-module/
│   ├── model/      # github.com/moduleforge/core-model
│   │   ├── migrations/0000_helpers.sql … 0005_service_accounts.sql
│   │   ├── queries/{entities,legal_entities,natural_persons,corporations,service_accounts}.sql
│   │   └── db/      # sqlc output
│   ├── api/        # github.com/moduleforge/core-api
│   │   ├── audit/           # Writer interface
│   │   ├── service/         # Principal, PrincipalExtractor, entity services
│   │   └── httpapi/         # NewRouter(deps) chi.Router
│   └── gui/        # @moduleforge/core-gui (yalc)
│       ├── src/{ProfileEditor,NaturalPersonForm,CorporationForm,ServiceAccountForm,ui/*}.tsx
│       └── dist/            # tsup output
└── users-module/
    ├── model/      # composes core migrations + own 0100+ into schema/migrations/
    ├── api/        # imports core-api + core-model; mounts core router; owns users/auth/apps/audit
    └── gui/        # imports @moduleforge/core-gui via yalc
```

### API composition

users-module mounts the core router inside its existing `/v1` auth-protected group. core-module's `PrincipalExtractor` reads a context value populated by users-module's existing `RequireAuth` middleware.

For flows that span both modules (admin user-create writes users + entities + auth_local atomically), users-module opens a pgx tx, calls core's service layer with the tx, writes its own rows, and commits.

### UI composition

core-module/gui exports presentational components. users-module/gui's `/profile` page reduces to: load current user via users-module auth context → pass to `<ProfileEditor onSave={api.self.update} />`. The admin user-edit page does the same inside its existing layout.

## Phases

1. **Bootstrap core-module skeleton** — go.work, three module scaffolds, root Makefile link targets.
2. **Extract model** — migrations 0000–0005 + queries → core-module/model; sqlc build; atlas hash.
3. **Wire model into users-module, drop duplicates** — require core-model; renumber users-module migrations to 0100+; compose pipeline; Dockerfile update.
4. **Build core-module/api** — audit interface; Principal + PrincipalExtractor; entity services; chi subrouter; tests; OpenAPI fragment.
5. **Wire users-module/api to consume core-module/api** — Principal adapter; mount core router; rework users.go to use core service in tx; delete self.go.
6. **Extract UI components into core-module/gui** — ProfileEditor + entity forms + shadcn primitives; tsup build; yalc; consumer Tailwind glob.
7. **Verification + cleanup** — full make test; dev.start smoke; atlas status; grep sanity; audit trail; docs.

Each phase runs on its own git worktree/branch. Task branches are carved off only for large, parallelizable tasks.

## Architecture

- [Entity typing](../docs/architecture/entity-typing.md) — type registry, rigid-designator semantics, append-only enforcement, display-rendering pattern.
- Full index: [`docs/architecture.md`](../docs/architecture.md).

## Supporting documents

- `TODO.md` — live phase/task checklist.
- `phase.<N>.<title>/phase.<N>.<title>.md` — phase summary + acceptance.
- `phase.<N>.<title>/phase.<N>.task.<M>.<title>.md` — agent-ready task instructions.
- Approved top-level design: `/Users/zane/.claude/plans/q1-a-definitely-q2-wondrous-donut.md`.
- Coupling analysis: see §"Coupling findings" in the approved plan.
