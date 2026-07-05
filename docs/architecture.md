# mod-core architecture

## Purpose and scope

These documents describe the **system design concepts, high-level specifications, and the reasoning behind them** for `mod-core` and the modules that build on it. The target audience is architects and engineers who need to understand the system as a whole, the top level concepts and components, and how everything fits together.

These documents do NOT cover specific APIs, usage, or operations.

## Framework overview

The unit of organisation is the **module**. A module is a self-contained code package built around a single domain or topic — user accounts, tagging, contact data, audit logging, and so on. A module bundles whatever data and behaviour that domain needs, expressed as some combination of three parts:

- a **model** part (database schema, migrations, generated query code),
- an **API** part (Go services and HTTP handlers), and
- a **GUI** part (React component library).

A module need not include all three.

Modules are not applications. Each module is a library that exposes its domain as a coherent surface — schema fragments, service constructors, HTTP route mounters, React components — without prescribing how it gets used. **Applications** are built by *composing* modules: a composition root constructs whatever modules the application needs, wires them with shared dependencies (a database pool, a single authorization policy, a single observer group, a configuration object), and produces a runnable artifact. `mod-users` contains a small example used for local debugging.

The composition root has three application-level wiring responsibilities that no individual module owns:

- **Authorization policy.** Construct one `Authorizer` implementation, then call `setup.ApplyFuncs(ctx, pool, generator, slugs)` from `mod-core/api/authz/setup` to install the SQL access functions for row-level scoping. Different apps may use different `AccessFuncGenerator` implementations (see [`authorization-design.md`](./architecture/authorization-design.md)).
- **Cross-module routing.** Routes that span multiple modules — e.g. dependent-data routes like `/natural-persons/{uuid}/contacts` (a contacts list under a parent legal entity) — are registered at the composition root, not by either module. The convention `/{parent-resource}/{uuid}/{dependent-resource}` is documentation, not a framework. Each peer module exposes service methods (e.g. `ContactService.ListByLegalEntity`); the app wires the URL.
- **Observer composition.** Build one `*observer.ObserverGroup` containing whichever cross-cutting observers the app composes (audit, outbox, cache invalidation, search-index sync); inject into every service.

**Module design rules:**

- **`mod-core` defines the shared contracts.** The interfaces and helpers that let modules be composed cleanly — entity identity, authorization, state-change observation, transaction lifecycle, request context — all live in `mod-core`. Other modules implement or use these contracts; they do not redefine them.
- **All other modules are final.** The `mod-core` is (almost always) used as the basis for other modules. However, there is no general "inheritance" mechanism like OOP style classes. A module may "build" off other modules internally, but that's all internal to the module.
- **Modules primarily model 'domains'.** In theory, you could have module that "sub class" other modules, extending or refining services and semantics. You could have modules that augment other modules by defining additional or more sophisticated GUI components. However, the current design focus is on "modules as domain implementations" which implement large, cohesive application chunks. It's natural for the module implementations to share many details, but those are implementation concerns not addressed here.
- **The module components may be tightly bound.** The model definition, API implementation, and GUI implementation may have interdependencies at the implementation level. E.g., the API may rely on denormalized view tables or other database implementation details for the sake of efficiency.

### Model

The model part is the persistent state for the domain.

**Scope:** SQL schema (Postgres), migrations (managed via `goose`), and the auto-generated Go query code (`sqlc`). Each module owns its own model package with its own `go.mod`, its own `migrations/` directory, and its own `db/` (sqlc output).

**Technical goals:**

- Canonical, transactional state in **Postgres**. The system of record is ACID.
- The primay data models are always fully normalised.
- Secondary de-normalized views and functions may be supported at the database layer for the sake of efficiency.
- **Keep to vanilla SQL unless Postgres-specific features offer signifcant advantage.** E.g., partial indexes and `JSONB` are noted architectural commitments.
- Generated query code (`sqlc`) so that Go callers see typed parameters and rows without the need for a runtime query layer.

**Design rules:**

- **`Entities` represent independent objects/data.** The `entities` table in `mod-core/model` is the abstract root for all classes that model indpendent things with their own existence. `Entities` may reference each other, but are never a fundamental/existential "part" or directly dependent on another `Entity` for their existence at the model level.[^bussinesdependencies]
- **Tables whose primary ID does not transitively FK to `entities` is considered "dependent" objcets/data.** A dependent object/data is just "part" of an `Entity`'s data. Therefore, a single `Entity` model (such as a `NaturalPerson` or `Company`) may be captured by by multiple tables.
- **Otherwises dependent object classes may be promoted to `Entity` status if they are involved in authorization resulotion.** The cannonical example here is the `UserAccount` types in `mod-users`. A `UserAccount` is logically dependent on a `User`, however it is the `UserAccount` and not the `User` themselves to which authorizations are attached and since the subject and target of an authorization must be an `Entity`, `UserAccount` is effectively promoted.
- **Internal IDs are integers; external IDs are UUIDs.** Internal IDs are used only for joins and FKs and are never sent in HTTP responses.
- **Tables are named in the plural; columns in `snake_case`.**
- **Sub-tables inherit primary keys from parent tables (CTI — class-table inheritance).** A `natural_persons` row's `entity_id` is both the primary key and the FK to `entities`.
- **Data encryption is an API level concern.** Sensitive data like SSN, EIN, and similar are stored as ciphertext bytes; the database never sees plaintext. This means this data cannot be easily searched and additional fields such as `last_four_ssn` may be added as needed. This non-encrypted, searchable side-car data must be carefully designed so that the full unencrypted data cannot be recreated nor narrowed to an unnaceptable small number of options.
- **Cross-module schema dependencies are resolved by composition, not coupling.** When a peer module needs core's `entities` table for FK resolution, the application composes core migrations + module migrations into a single ordered set (`make compose`), rather than the peer module embedding core schema.

[^bussinesdependencies]: An `Entity` may depend on another `Entity` according to business rules. For instance, a `Post` `created_by` a `User` may be deleted because of an account deletion request. In general, though, at the DB level, cascade deletes and changes should not cross `Entity` boundaries.

For details:
- [Entity typing](./architecture/entity-typing.md) — the type registry, fundamental-type concept, abstract vs. concrete types, immutability rules.
- [Database considerations](./architecture/db-considerations.md) — why Postgres, why `goose` for migrations, why `sqlc` for query code, what was rejected.

### API

The API part is the behaviour for the domain — what callers can do and what happens when they do it.

**Scope:** Go service types in `api/service/` (the business-logic layer) and HTTP handlers in `api/httpapi/` (the transport-binding layer).

**Technical goals:**

- **Thin handlers, business logic in services.** Handlers parse and validate inputs, call a single service method, and shape the response. No business decisions in handlers.
- **REST-ful HTTP surface.** Resources are nouns, verbs are HTTP methods (`POST` to create, `PUT` to update, `GET` to retrieve, `DELETE` to delete). Specialised operations are `/{resource}/{id}/{action}`.
- **All requests except `login` carry a bearer token.** Authentication is consistent across modules.
- **Non-business logic services are decoupled and pluggable, not baked in.** E.g., authorization and audit functions are provided by other modules or libraries composed and configured at the application level as part of the application definition.

**Design rules:**

- **Authorization is the first question.** A single `Authorizer` per application gates every operation, including reads. The authorization decision may consider context beyond 'subject', 'operation', and 'target' such as whether sensitive fields are being requested. Service methods trust that authorization has been checked.
- **State changes flow through observers.** Audit, cache invalidation, outbox dispatch, search-index sync, etc. are all `MutationObserver` implementations composed by the application. Modules emit observation events at known points; they do not know which observers (if any) are listening.
- **Transactions are owned by `txhelper.Run`.** Service code does not open its own transaction; the helper begins, commits, rolls back, and dispatches post-commit observation. 
- **This yields a standard service-method shape.** Every mutating service method follows `Authorize → txhelper.Run(fetch before → mutate → Observe) → ObserveAfterCommit`. Every read method follows `Authorize → fetch & process`. There are no variations. The agent-facing one-pager is at [`skill.cross-cutting.md`](../../skill.cross-cutting.md) at the project root.
- **Actor identity flows on `context.Context` (`opctx`).** Action and target are explicit method parameters; the authenticated user is ambient request context.
- **`pgx` for the Postgres driver, `chi` for HTTP routing.** These are stable choices; replacing either is an architectural decision.

For details:
- [Authorization design](./architecture/authorization-design.md) — the `Authorizer` interface, `opctx`, the Entity contract, where the Authorize call goes.
- [State-management design](./architecture/state-management-design.md) — `MutationObserver`, `ObserverGroup`, `txhelper.Run`, the standard service-method shape, app-side composition, and the cross-cutting use cases (audit, cache invalidation, outbox, search-index sync, webhooks).
- [Cross-cutting design rationale](./architecture/cross-cutting-design-rationale.md) — why this pattern (typed interfaces, explicit calls) over decorators, hook bus, repo-level hooks, or HTTP middleware.

### GUI

The GUI part is the user-facing surface for the domain — the React components and front-end primitives that an application uses to present the module's data and operations.

**Scope:** A standalone React component library per module (e.g. `mod-tags/gui`, `mod-contacts/gui`). TypeScript, npm-published. No application code; no module-specific routing or state stores beyond the components themselves.

**Technical goals:**

- **Library-first.** Each module's GUI is a set of reusable components an application composes; it is not a self-contained UI.
- **Built with a library toolchain, not an app framework.** Each GUI package builds its `dist/` (ESM + CJS + `.d.ts`) with `tsup`; Ladle, the component workbench, runs on Vite. The applications that consume these libraries use a full app framework (Next.js, in this ecosystem — see the aggregate's `docs/apps-architecture.md`), because they solve a different problem: a library must produce an importable package reusable across every consuming app, while an app framework produces one deployable, routable artifact for a single running application. A module GUI built with an app framework instead of a library toolchain cannot be cleanly exported for reuse by a second app — this is why `mod-users`'s GUI was rewritten from Next.js to tsup.
- **Components stand alone.** A tag editor or a contact form should be droppable into any application without dragging in module-specific app shell, navigation, or auth flows.
- **TypeScript with public type contracts.** Component props are the API surface; consumers should not need to read implementation files.
- **Component workbench for component-level development and review.** The workbench renders components in isolation with mocked-or-real data. Ladle is the standard, though individual implementations are free to use Storybook or other frameworks for this purpose.

**System-wide assumptions:**

- **Authentication and routing are application concerns**, not module concerns. A module's components accept whatever data they need as props; they do not call `/login` or assume a router.[^authbymodule]
- **A modules GUI components may factor in authorization** in order to determine what options to display or not. Ideally, these kinds of questions should be answered by results from the API. E.g., fetch results could indicate whether the data can be editted or not. However, it is also possible that the GUI component ask "can edit" or "can access" to determine what options to present. In general, impossible options should not be displayed.
- **Cross-module data flow is the application's responsibility.** A `<TagEditor>` does not reach into `mod-users`; the application provides whatever lookups the editor needs.
- **No module's GUI imports another module's GUI.** Same peer-module independence rule as the model and API parts. However, modules may take parameters which are then displayed. For instance, a "detail" widget may take an `additionalOptions` input which would be an array of labels + widgets like 'Tags' + tag widget, and then display the additional data options as secondary options.

[^authbymodule]: Though authentication is logically an "application level concern", the `mod-users` does provide an authentication implementation. The point here is that, in general, modules do not concern themselves with authentication.

The GUI surface is less mature than the model and API surfaces; current modules use it where there is end-user-visible functionality (tags, contacts, core entity forms), but there is no formal design doc yet.

## Topic guidelines

- [Entity typing](./architecture/entity-typing.md)
- [Database considerations](./architecture/db-considerations.md)
- [Authorization design](./architecture/authorization-design.md) — `Authorizer` interface, `opctx`, Entity contract.
- [State-management design](./architecture/state-management-design.md) — `MutationObserver`, `ObserverGroup`, `txhelper.Run`, standard service-method shape, audit / cache / outbox use cases.
- [Cross-cutting design rationale](./architecture/cross-cutting-design-rationale.md) — why this pattern was chosen over decorators, hook bus, repo hooks, and HTTP middleware.
