# core-module architecture

## Scope and purpose

These documents describe the **system design concepts, high-level specifications, and the reasoning behind them** for `core-module` and the modules that build on it. The target audience is architects and engineers who need to understand the system as a whole, the top level concepts and components, and how everything fits together.

These documents do NOT cover specific APIs, usage, or operations.

## Framework overview

The unit of organisation is the **module**. A module is a self-contained code package built around a single domain or topic — user accounts, tagging, contact data, audit logging, and so on. A module bundles whatever data and behaviour that domain needs, expressed as some combination of three parts:

- a **model** part (database schema, migrations, generated query code),
- an **API** part (Go services and HTTP handlers), and
- a **GUI** part (React component library).

A module need not include all three. `core-module` ships model, API, and (a small) GUI; `audit-module` is model + API; the GUI parts of other modules are independent libraries published separately.

Modules are not applications. Each module is a library that exposes its domain as a coherent surface — schema fragments, service constructors, HTTP route mounters, React components — without prescribing how it gets used. **Applications** are built by *composing* modules: a composition root constructs whatever modules the application needs, wires them with shared dependencies (a database pool, a single authorization policy, a single observer group, a configuration object), and produces a runnable artifact. Today the only such composition root is `users-module/cmd/server`; a future dedicated `app/` project will take this role over.

Two system-wide rules hold across every module:

- **Peer-module independence.** A peer module depends only on `core-module`. It never imports another peer module's packages, schema, or types. Cross-module relationships exist only at the composition root.
- **`core-module` defines the shared contracts.** The interfaces and helpers that let modules be composed cleanly — entity identity, authorization, state-change observation, transaction lifecycle, request context — all live in `core-module`. Peer modules implement or use these contracts; they do not redefine them.

The three parts of a module are described below.

### Model

The model part is the persistent state for the domain.

**Scope:** SQL schema (Postgres), migrations (managed via `goose`), and the auto-generated Go query code (`sqlc`). Each module owns its own model package with its own `go.mod`, its own `migrations/` directory, and its own `db/` (sqlc output).

**Technical goals:**

- Canonical, transactional state in **Postgres**. The system of record is ACID; caches sit above it, never below.
- Fully normalised. Caching, denormalisation, or read-model projections are application concerns above the data layer, not in it.
- **Vanilla SQL where reasonable, Postgres-specific features where they pay for themselves.** Partial indexes and `JSONB` are used deliberately and noted as architectural commitments.
- Generated query code (`sqlc`) so that Go callers see typed parameters and rows without a hand-rolled query layer.

**System-wide assumptions:**

- **Every domain object is an `Entity`.** The `entities` table in `core-module/model` is the abstract root; module-specific tables either FK to `entities(id)` (the typical pattern) or model dependent data attached to a parent entity. The entity-typing registry decouples type extension from core schema changes.
- **Internal IDs are integers; external IDs are UUIDs.** Internal IDs are used only for joins and FKs and are never sent in HTTP responses.
- **Tables are named in the plural; columns in `snake_case`.**
- **Sub-tables inherit primary keys from parent tables (CTI — class-table inheritance).** A `natural_persons` row's `entity_id` is both the primary key and the FK to `entities`.
- **Sensitive blobs are encrypted at the application layer.** SSN, EIN, and similar are stored as ciphertext bytes; the database never sees plaintext.
- **Cross-module schema dependencies are resolved by composition, not coupling.** When a peer module needs core's `entities` table for FK resolution, the application composes core migrations + module migrations into a single ordered set (`make compose`), rather than the peer module embedding core schema.

For details:
- [Entity typing](./architecture/entity-typing.md) — the type registry, fundamental-type concept, abstract vs. concrete types, immutability rules.
- [Database considerations](./architecture/db-considerations.md) — why Postgres, why `goose` for migrations, why `sqlc` for query code, what was rejected.

### API

The API part is the behaviour for the domain — what callers can do, who is allowed to do it, and what happens when they do.

**Scope:** Go service types in `api/service/` (the business-logic layer) and HTTP handlers in `api/httpapi/` (the transport-binding layer).

**Technical goals:**

- **Thin handlers, business logic in services.** Handlers parse and validate inputs, call a single service method, and shape the response. No business decisions in handlers.
- **REST-ful HTTP surface.** Resources are nouns, verbs are HTTP methods (`POST` to create, `PUT` to update, `GET` to retrieve, `DELETE` to delete). Specialised operations are `/{resource}/{id}/{action}`.
- **All requests except `login` carry a bearer token.** Authentication is consistent across modules.
- **Cross-cutting concerns are pluggable, not baked in.** Authorization and state-change observation flow through interfaces defined in `core-module`; no module hard-codes a particular authz implementation or audit writer.

**System-wide assumptions:**

- **Standard service-method shape.** Every mutating service method follows `Authorize → txhelper.Run(fetch before → mutate → Observe) → ObserveAfterCommit`. Every read method begins with `Authorize`. There are no variations. The agent-facing one-pager is at [`skill.cross-cutting.md`](../../skill.cross-cutting.md) at the project root.
- **Authorization is the first line.** A single `Authorizer` per application gates every operation, including reads. Service methods do not duplicate authorization in handlers.
- **State changes flow through observers.** Audit, cache invalidation, outbox dispatch, search-index sync, etc. are all `MutationObserver` implementations composed by the application. Modules emit observation events at known points; they do not know which observers (if any) are listening.
- **Transactions are owned by `txhelper.Run`.** Service code does not open its own transaction; the helper begins, commits, rolls back, and dispatches post-commit observation.
- **Actor identity flows on `context.Context` (`opctx`).** Action and target are explicit method parameters; the authenticated user is ambient request context.
- **`pgx` for the Postgres driver, `chi` for HTTP routing.** These are stable choices; replacing either is an architectural decision.

For details:
- [Authorization design](./architecture/authorization-design.md) — the `Authorizer` interface, `opctx`, the Entity contract, where the Authorize call goes.
- [State-management design](./architecture/state-management-design.md) — `MutationObserver`, `ObserverGroup`, `txhelper.Run`, the standard service-method shape, app-side composition, and the cross-cutting use cases (audit, cache invalidation, outbox, search-index sync, webhooks).
- [Cross-cutting design rationale](./architecture/cross-cutting-design-rationale.md) — why this pattern (typed interfaces, explicit calls) over decorators, hook bus, repo-level hooks, or HTTP middleware.

### GUI

The GUI part is the user-facing surface for the domain — the React components and front-end primitives that an application uses to present the module's data and operations.

**Scope:** A standalone React component library per module (e.g. `tags-module/gui`, `contacts-module/gui`). TypeScript, npm-published. No application code; no module-specific routing or state stores beyond the components themselves.

**Technical goals:**

- **Library-first, not app-first.** Each module's GUI is a set of reusable components an application composes; it is not a self-contained UI.
- **Components stand alone.** A tag editor or a contact form should be droppable into any application without dragging in module-specific app shell, navigation, or auth flows.
- **TypeScript with public type contracts.** Component props are the API surface; consumers should not need to read implementation files.
- **Component workbench (Ladle today; possibly Storybook later) for component-level development and review.** The workbench renders components in isolation with mocked-or-real data.

**System-wide assumptions:**

- **Authentication and routing are application concerns**, not module concerns. A module's components accept whatever data they need as props; they do not call `/login` or assume a router.
- **Cross-module data flow is the application's responsibility.** A `<TagEditor>` does not reach into `users-module`; the application provides whatever lookups the editor needs.
- **No module's GUI imports another module's GUI.** Same peer-module independence rule as the model and API parts.

The GUI surface is less mature than the model and API surfaces; current modules use it where there is end-user-visible functionality (tags, contacts, core entity forms), but there is no formal design doc yet. Notes on outstanding work are in each module's `next-steps.md`.

## Topic guidelines

- [Entity typing](./architecture/entity-typing.md)
- [Database considerations](./architecture/db-considerations.md)
- [Authorization design](./architecture/authorization-design.md) — `Authorizer` interface, `opctx`, Entity contract.
- [State-management design](./architecture/state-management-design.md) — `MutationObserver`, `ObserverGroup`, `txhelper.Run`, standard service-method shape, audit / cache / outbox use cases.
- [Cross-cutting design rationale](./architecture/cross-cutting-design-rationale.md) — why this pattern was chosen over decorators, hook bus, repo hooks, and HTTP middleware.
