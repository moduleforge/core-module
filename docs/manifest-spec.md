# ModuleForge Manifest Format Specification

## Version and scope

This document is the canonical contract for the ModuleForge manifest format. It is the single source of truth for:

- Module authors writing a new `moduleforge.module.yaml`
- Application authors writing a `moduleforge.app.yaml`
- Compiler implementors building `mfgen`

This spec covers everything needed to write a valid manifest from scratch without reading any other document. All design decisions documented here were resolved in the planning session that preceded phase 1.

**YAML library note:** `mfgen` uses `gopkg.in/yaml.v3`. Despite the library's name, yaml.v3 still follows YAML 1.1 integer parsing: integers with a leading `0` followed by octal digits (`0–7`) are parsed as **octal** (e.g. `0400` → 256, `0500` → 320). Write all integer fields — including `migrations.range.first` and `migrations.range.last` — as **plain decimal values without leading zeros** (e.g. `first: 400`, not `first: 0400`). The zero-padded format is a filesystem convention for migration filenames only.

---

## Table of contents

1. [Manifest types overview](#1-manifest-types-overview)
2. [moduleforge.module.yaml — field reference](#2-moduleforgemodule-yaml--field-reference)
3. [moduleforge.app.yaml — field reference](#3-moduleforgeappyaml--field-reference)
4. [Arg-source vocabulary](#4-arg-source-vocabulary)
5. [Migration range rules](#5-migration-range-rules)
6. [Routing scope matching](#6-routing-scope-matching)
7. [Observer composition](#7-observer-composition)
8. [Validation rules](#8-validation-rules)
9. [Generation pipeline](#9-generation-pipeline)
10. [Worked examples](#10-worked-examples)

- [Appendix A: core-module special case — `mountFromModule`](#appendix-a-core-module-special-case--mountfrommodule)
- [Appendix B: reserved service names](#appendix-b-reserved-service-names)
- [Appendix C: generated file header](#appendix-c-generated-file-header)

---

## 1. Manifest types overview

The system uses two manifest types. They live at the root of their respective repos.

| File | Placed in | Purpose |
|---|---|---|
| `moduleforge.module.yaml` | each module repo root | Declares what the module provides and what it requires |
| `moduleforge.app.yaml` | the aggregator repo root | Selects modules and wires the app-level config and infra |

The compiler (`mfgen`) reads one `moduleforge.app.yaml` and follows it to each module's `moduleforge.module.yaml`. It validates the combined set and generates `main.go` and `go.mod` into the directory named by `outputDir`.

---

## 2. moduleforge.module.yaml — field reference

### Top-level shape

```yaml
module: <string>           # required
goModule: <string>         # required (== go.api.modulePath)
go:                        # required
  api:
    modulePath: <string>
    dir: <string>
  model:
    modulePath: <string>
    dir: <string>
migrations:
  range:
    first: <int>           # required
    last: <int>            # required
provides:
  routes: [...]            # optional
  services: [...]          # optional
  observers: [...]         # optional
  middleware: [...]        # optional
requires:
  services: [...]          # optional
  infra: [...]             # optional
```

### Top-level fields

| Field | Type | Required | Purpose |
|---|---|---|---|
| `module` | string | yes | Human-readable module identifier used in error messages and as the key in `app.yaml` `modules:` list |
| `goModule` | string | yes | Go module path prefix for this module's API package (e.g. `github.com/moduleforge/audit-api`). The compiler uses this to build import paths. Must be a valid Go module path (see V8). |

### `migrations.range`

| Field | Type | Required | Purpose |
|---|---|---|---|
| `migrations.range.first` | int | yes | First migration file number this module owns (inclusive) |
| `migrations.range.last` | int | yes | Last migration file number this module owns (inclusive) |

The compiler validates that no two selected modules have overlapping ranges. See [Migration range rules](#5-migration-range-rules).

### `go`

The top-level `go:` section declares the Go sub-module paths for the module's component packages.

```yaml
go:
  api:
    modulePath: <string>   # required — Go module path of the api sub-module
    dir: <string>          # optional — path to the api sub-module root, relative to the module repo root (default ./api)
  model:
    modulePath: <string>   # required — Go module path of the model sub-module
    dir: <string>          # optional — path to the model sub-module root, relative to the module repo root (default ./model)
```

| Field | Type | Required | Purpose |
|---|---|---|---|
| `go.api.modulePath` | string | yes | Go module path of the module's API package (e.g. `github.com/moduleforge/core-api`). Used by the compiler to build import paths for the module's handlers, services, and middleware, and as the target of the generated `replace`/`require` directive. Must equal the `module:` value used to select this module in `moduleforge.app.yaml`. Must be a valid Go module path (see V8). |
| `go.api.dir` | string | no | Directory of the API sub-module relative to the module repo root. Defaults to `./api`. Appended to `localPath` to form the `replace` target. |
| `go.model.modulePath` | string | yes | Go module path of the module's model/db package (e.g. `github.com/moduleforge/core-model`). Used for the `queries:`/`txQueryFactory:` arg-sources and the model `replace`/`require` directive. Must be a valid Go module path. |
| `go.model.dir` | string | no | Directory of the model sub-module relative to the module repo root. Defaults to `./model`. |

**Note:** The top-level `goModule:` field remains and must equal `go.api.modulePath`. The `go:` section additionally declares the `model` sub-module path, which `goModule:` alone cannot express. New manifests should populate both for consistency.

### `provides.routes[]`

Each entry in `provides.routes` describes one mountable HTTP route group.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `prefix` | string | yes | URL path prefix under which this route group is mounted (e.g. `/v1/audit`) |
| `handler` | string | yes | Name of the handler variable constructed in the generated code (e.g. `auditHandler`). Omitted when `mountFromModule:` is set (the router is constructed directly from the `mountFromModule` expression without a named handler variable). |
| `constructor` | string | yes | Constructor call that produces the handler (e.g. `audithttpapi.NewAuditHandler`) |
| `args` | list of arg-source | yes | Ordered arguments to `constructor`. Each entry is a string in one of the 9 arg-source forms (see [§4](#4-arg-source-vocabulary)). |
| `register` | string | optional | If set, the name of the `RegisterRoutes`-style function to call instead of mounting the handler directly. Set when the module exposes a `func(chi.Router, handler)` registration function rather than returning a `chi.Router`. |
| `mountFromModule` | string | optional | When a module exposes `NewRouter` that returns a full `chi.Router` (not a registration function), set this to the fully-qualified `NewRouter` call. The compiler emits `r.Mount(prefix, <mountFromModule>(deps))`. Used by core-module, contacts-module, and tags-module. Mutually exclusive with `register`. |
| `scope` | string | optional | Authentication scope gate applied before this route group. Values: `public` (no auth required), `authenticated` (valid bearer token required), `verified` (authenticated + email verified). Default: `authenticated`. |
| `middleware` | list of string | optional | Named middleware from `provides.middleware` to apply to this route group, in order. |
| `innerMount` | bool | optional | When `true`, the module router is emitted inside a `r.Route("<prefix>", ...)` block owned by another module rather than as a standalone top-level `r.Mount` call. Use this when another module already registers the same prefix at the top level so that chi does not panic on duplicate mounts. Default: `false` (top-level `r.Mount`). Only meaningful when `mountFromModule` is also set. |

**`register` vs `mountFromModule`:** Some modules expose routes through a `RegisterRoutes(r chi.Router, handler)` function (audit-module, authz-module). Others expose `NewRouter(deps) chi.Router` that returns a full mountable router (core-module, contacts-module, tags-module). Use `register` for the former and `mountFromModule` for the latter.

**`constructor` and `register` — two patterns:** There are two valid patterns for `register:` entries, and whether `constructor:` is required depends on the pattern:

- **Pattern A — pre-built handler struct:** `constructor:` is required. The `constructor` call builds a handler struct (e.g. `audithttpapi.NewAuditHandler`), and `register` is a function that takes that struct as its first argument (e.g. `audithttpapi.RegisterRoutes(r, handler)`). The compiler first constructs the handler using `constructor` + `args`, then calls `register(r, handler)` inside a `r.Route(prefix, ...)` block. audit-module uses this pattern.
- **Pattern B — service-aggregate registration:** `constructor:` is omitted. The `register` function takes service dependencies directly (e.g. the services aggregate or individual services) and constructs handlers internally. No separate pre-built handler struct is needed. authz-module uses this pattern.

### `provides.services[]`

Each entry in `provides.services` declares a named service value that other modules may consume via `requires.services`.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `name` | string | yes | Logical name for this service (e.g. `auditObserver`). Must be unique across all services provided by all selected modules. Used as the variable name in generated code and as the key for `requires.services` lookups. |
| `type` | string | yes | The Go type of the provided value as it appears in the generated code (e.g. `*auditservice.Observer`, `authz.Authorizer`). Used for documentation and for verifying the consumer's expected type. |
| `constructor` | string | yes | Go expression that constructs the service value (e.g. `auditservice.New`, `localAuthz.New`). |
| `args` | list of arg-source | yes | Ordered arguments passed to `constructor`. Each entry is a string in one of the 9 arg-source forms. |

### `provides.observers[]`

Each entry declares an observer that the module contributes to the app's `ObserverGroup`.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `name` | string | yes | Identifier for this observer. Used in error messages and in `ObserverGroup` construction order. |
| `service` | string | yes | Name of the service (from `provides.services`) that implements `core-api/observer.MutationObserver`. |
| `policy` | string | yes | Observer error policy. Values: `propagate` (errors abort the operation), `swallow` (errors are logged, operation continues), `default` (uses the ObserverGroup's configured default policy). Controls which `ObserverGroup` variant the compiler uses at call sites (`MustObserve` for propagate, `MayObserve` for swallow, `Observe` for default). |

### `provides.middleware[]`

Each entry declares a middleware function the module contributes for use by routes in the same module.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `name` | string | yes | Identifier for this middleware. Referenced in `provides.routes[].middleware`. |
| `constructor` | string | yes | Go expression that constructs the middleware (e.g. `auth.RequireAuth`). |
| `args` | list of arg-source | yes | Ordered arguments to `constructor`. |

### `requires.services[]`

Each entry declares a service the module consumes from another module.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `name` | string | yes | The logical name of a service declared in another module's `provides.services`. The compiler verifies that exactly one selected module provides a service with this name. |
| `as` | string | optional | Local alias for the service in the generated code. If omitted, the compiler uses `name` as the variable reference. |

### `requires.infra[]`

Each entry declares a shared infrastructure dependency (database pool, etc.) the module consumes.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `name` | string | yes | The name of an infra block declared in the app's `infra:` section (e.g. `pool`). |
| `as` | string | optional | Local alias for the infra value in this module's context. If omitted, `name` is used. |

---

## 3. moduleforge.app.yaml — field reference

### Top-level shape

```yaml
app: <string>              # required
goModule: <string>         # required
outputDir: <string>        # required
modules:
  - module: <string>       # required
    localPath: <string>    # optional
config:
  <key>: <value>           # arbitrary key-value pairs
infra:
  <name>:
    type: <string>         # required
    constructor: <string>  # required
    args: [...]            # required
```

### Top-level fields

| Field | Type | Required | Purpose |
|---|---|---|---|
| `app` | string | yes | Human-readable application name. Used in generated file headers and error messages. |
| `goModule` | string | yes | Go module path for the generated application (e.g. `github.com/myorg/myapp`). Written into the generated `go.mod`. Must be a valid Go module path (see V8). |
| `outputDir` | string | yes | Directory path (relative to the repo root where `mfgen` is run) where the compiler writes `main.go` and `go.mod`. The compiler errors if this directory contains any non-generated file (detected by the absence of the `// Code generated by mfgen. DO NOT EDIT.` header comment on `main.go`). The path must not escape the project root: values containing `..` path components or absolute paths are rejected at compile time (see V6). |

### `modules[]`

Each entry selects one module to include in the generated application.

```yaml
modules:
  - module: <string>         # required
    localPath: <string>      # optional
```

| Field | Type | Required | Purpose |
|---|---|---|---|
| `module` | string | yes | The Go module path of the module's `api` sub-module (e.g. `github.com/moduleforge/core-api`). Must match the `go.api.modulePath` declared in the selected module's `moduleforge.module.yaml`. The compiler uses this as the identity of the module and as the import-path root for its API packages. Must be a valid Go module path (see V8). |
| `localPath` | string | no | Filesystem path to the module repo root, relative to the directory containing `moduleforge.app.yaml`, used for local development (e.g. `../core-module`). When present and the directory exists on disk, the compiler reads the module manifest from `<localPath>/moduleforge.module.yaml` and emits `replace` directives in the generated `go.mod` pointing at the local source. When absent, the module is treated as a published dependency resolved at a pinned version. |

#### Module reference resolution

- **`localPath` present and directory exists** → local mode. The compiler:
  - reads `<localPath>/moduleforge.module.yaml`,
  - derives import paths from that manifest's `go:` section,
  - emits `replace <go.api.modulePath> => <abs-localPath>/<go.api.dir>` and `replace <go.model.modulePath> => <abs-localPath>/<go.model.dir>` in the generated `go.mod`.
- **`localPath` absent (or present but directory missing)** → published mode. The module is expected to be available as a published Go module at a pinned version, and the compiler emits a `require <module> <version>` directive with no `replace`.

> **Note:** Published-mode resolution (download at pinned version) is reserved; the current compiler requires every selected module to declare a `localPath` that exists on disk and errors otherwise.

### `config`

A flat or nested key-value map. Values from this block are reachable via the `config:` arg-source (see [§4](#4-arg-source-vocabulary)). Keys are navigated with dot notation: `config:auth.adminRole` resolves `config.auth.adminRole` in the YAML.

There are no fixed field names in `config:` — all entries are application-specific. The compiler validates that any `config:` arg-source reference resolves to a key that exists in this block.

| Field | Type | Required | Purpose |
|---|---|---|---|
| (any key) | any scalar or map | per-module | Application configuration values consumed by module constructors |

### `infra`

A map of named infrastructure singletons. Each entry is constructed once at startup and shared with any module that lists its name in `requires.infra`.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `<name>` | object | — | Defines one named infra singleton |
| `<name>.type` | string | yes | Go type of the value (e.g. `*pgxpool.Pool`). Used in generated code variable declarations. |
| `<name>.constructor` | string | yes | Go expression that constructs the value (e.g. `localdb.New`). |
| `<name>.args` | list of arg-source | yes | Ordered arguments to `constructor`. |

---

## 4. Arg-source vocabulary

Every `args:` list in the manifest is a list of arg-source strings. Each string specifies where the compiler should obtain one constructor argument.

The compiler resolves each arg-source to a Go expression at code generation time.

| Form | Syntax | Resolved type | Example |
|---|---|---|---|
| `infra:` | `infra:<name>` | The Go type declared in `app.yaml infra.<name>.type` | `infra:pool` → `pool` (a `*pgxpool.Pool`) |
| `service:` | `service:<name>` | The Go type declared in the providing module's `provides.services.<name>.type` | `service:authorizer` → `az` (an `authz.Authorizer`) |
| `queries:` | `queries:<import-alias>` | `*<package>.Queries` backed by the pool | `queries:auditdb` → `auditdb.New(pool)` |
| `config:` | `config:<dot.path>` | The scalar value at `dot.path` in the app's `config:` block | `config:auth.adminRole` → `cfg.Auth.AdminRole` |
| `symbol:` | `symbol:<pkg>.<Func>` | The function itself (not a call) | `symbol:auth.HashPassword` → `auth.HashPassword` |
| `method:` | `method:<varname>.<Method>` | Bound method expression on a named variable | `method:resolver.SetObserverGroup` → `resolver.SetObserverGroup` |
| `field:` | `field:<varname>.<Field>` | Struct field access on a named variable | `field:cfg.LocalAuth.JWTSecret` → `cfg.LocalAuth.JWTSecret` |
| `txQueryFactory:` | `txQueryFactory:<import-alias>` | `func(pgx.Tx)*<package>.Queries` closure | `txQueryFactory:auditdb` → `func(tx pgx.Tx) *auditdb.Queries { return auditdb.New(tx) }` |
| `context` | `context` | `context.Context` | `context` → `ctx` (the boot context) |

### Arg-source resolution rules

- **`infra:<name>`** — `name` must match a key in the app's `infra:` block. The compiler uses the variable name the infra singleton was assigned when constructed.
- **`service:<name>`** — `name` must match a `provides.services[].name` in exactly one selected module. Circular dependencies among services are a compile-time error (see [§8](#8-validation-rules)).
- **`queries:<import-alias>`** — the compiler emits `<import-alias>.New(pool)`. The `import-alias` must correspond to a Go import in the generated `main.go`. The compiler derives it from the module's `goModule` path plus the standard `db` suffix.
- **`config:<dot.path>`** — the path is dot-delimited and navigates the app's `config:` YAML map. The compiler validates at generation time that the path exists and holds a scalar value.
- **`symbol:<pkg>.<Func>`** — the compiler emits the bare identifier `<pkg>.<Func>`. No parentheses. Used when a constructor expects a function value (e.g. a hash function or a factory). The value after `symbol:` must be a valid Go qualified identifier (see V7).
- **`method:<varname>.<Method>`** — the compiler emits `<varname>.<Method>` as a bound method reference. The variable `varname` must have been assigned before this point in the generated `main.go`. The value after `method:` must be a valid Go qualified identifier (see V7).
- **`field:<varname>.<Field>`** — the compiler emits `<varname>.<Field>`. The variable must be assigned earlier in the generated code. The path may contain multiple dots for nested struct access (e.g. `field:cfg.LocalAuth.JWTSecret`). The value after `field:` must be a valid Go qualified identifier (see V7).
- **`txQueryFactory:<import-alias>`** — the compiler emits an inline closure: `func(tx pgx.Tx) *<import-alias>.Queries { return <import-alias>.New(tx) }`. This is the standard pattern for passing a tx-scoped query factory to observers that must write inside the operation's transaction (e.g. the audit-module observer).
- **`context`** — the compiler emits `ctx`, the `context.Context` passed to the boot sequence. Used when a constructor needs a context for startup-time I/O (e.g. loading keys, running migrations).

---

## 5. Migration range rules

### Range assignment

Each module declares a numeric range of migration file numbers it owns. Ranges are assigned at module creation time and do not change.

**Plain decimal integers required:** Write range values as plain decimal integers without leading zeros (e.g. `first: 400`, `last: 499`). Leading zeros cause yaml.v3 to misparse the value as octal (see §1 YAML library note). Migration filenames on disk use zero-padded strings (e.g. `0400_create_audit_log.sql`), but the compiler matches them by numeric value after stripping the numeric prefix, so the formats are independent.

Current assignments:

| Module | Range |
|---|---|
| core-module | 1–99 |
| users-module | 100–199 |
| tags-module | 200–299 |
| contacts-module | 300–399 |
| audit-module | 400–499 |
| authz-module | 500–599 |

### Enforcement rules

1. **No overlap.** The compiler errors if any two selected modules have overlapping migration ranges. Overlap is defined as: for modules A and B, A.first ≤ B.last AND B.first ≤ A.last.
2. **Monotonic within a module.** Within a single module's migration directory, each migration file number must be strictly greater than the preceding one. The compiler validates this by reading the `migrations/` directory in the module path.
3. **Range bounds respected.** Every migration file number in a module's `migrations/` directory must fall within `[first, last]` inclusive. Files outside the declared range are a compiler error.
4. **Gap policy.** Gaps within a module's range (e.g. 0401, then 0403, skipping 0402) are permitted. The compiler does not require contiguous numbering; it only requires monotonic ordering and that numbers stay within the declared range.

### Error messages

- Overlapping ranges: `migration range conflict: module "users" (100–250) overlaps with module "contacts" (200–399)`
- Out-of-range file: `migration file 0600_foo.sql in module "authz" is outside declared range 500–599`
- Non-monotonic file: `migration files in "users" are not monotonically increasing at 0100_apps.sql → 0100_users.sql`

---

## 6. Routing scope matching

The compiler generates the router mount block in `main.go`. When multiple route entries could match the same incoming request, the compiler applies the following precedence rules to determine which handler is mounted at which level.

### Precedence rules (highest to lowest)

1. **Exact path beats prefix path.** A path entry `/v1/auth/login` beats a prefix entry `/v1/auth` when matching the URL `/v1/auth/login`. The compiler orders exact mounts before prefix mounts within the same parent group.
2. **Method-prefixed path beats path-only at the same specificity.** An entry `GET /v1/audit` beats `/v1/audit` when the method is GET. Method-prefixed entries are mounted using `r.Get(...)`, `r.Post(...)`, etc. Path-only entries are mounted using `r.Route(...)`.
3. **Most-specific path wins.** Among paths with equal method specificity, the longer (more specific) path wins. `/v1/authz/actor-groups/{uuid}/members` beats `/v1/authz/actor-groups/{uuid}` for a request to `/v1/authz/actor-groups/abc/members`.
4. **Alphabetical tiebreak.** When two paths are equal in all other respects, the compiler uses alphabetical order of the module name to break ties, and emits a warning.

### Router generation behavior

- The compiler collects all `provides.routes` entries from all selected modules.
- It groups entries by their common path prefix (as determined by the chi router's nesting model).
- Within each group, it emits mounts in most-specific-first order.
- Middleware specified in `provides.routes[].middleware` is applied with `r.Use(...)` at the innermost scope containing only the route entries that reference it.

---

## 7. Observer composition

### The MutationObserver interface

The `core-api/observer` package defines two interfaces that observers implement:

```
MutationObserver
  Observe(ctx, tx, op, resource, targetEntityID, before, after) error
  ObserveAfterCommit(ctx, op, resource, targetEntityID, after)
```

An implementation may implement only one of the two methods as a no-op if it cares about only one phase.

### In-tx observers

Observers that implement `Observe` run **inside the operation's transaction**. The observer receives the live `pgx.Tx` and must write its side effects (e.g. an audit log row, an outbox row) in the same transaction. If the operation rolls back, the observer's writes roll back with it. This is the correct pattern for:

- Audit rows (must be transactionally consistent with the data change)
- Outbox rows (the event must commit atomically with the data or the delivery guarantee is broken)

### Post-commit observers

Observers that implement `ObserveAfterCommit` run **after the transaction commits**. They do not receive a `pgx.Tx`. Errors from post-commit observers are logged but cannot abort the already-committed operation. This is the correct pattern for:

- Cache invalidation (invalidating before commit creates a re-cache window on stale data)
- External system notifications and webhook delivery (should not fire for rolled-back operations)

### ObserverGroup policy variants

The compiler generates an `ObserverGroup` that wraps all observers from all selected modules. The `policy:` field on each observer entry in `provides.observers` controls which call variant is used at service call sites:

| `policy:` value | Generated call variant | Behavior |
|---|---|---|
| `propagate` | `MustObserve(...)` | Always propagates the first observer error; aborts the operation and rolls back the transaction |
| `swallow` | `MayObserve(...)` | Always logs and swallows observer errors; the operation proceeds regardless |
| `default` | `Observe(...)` | Uses the ObserverGroup's configured default policy (PolicyPropagate unless overridden at composition time) |

The `ObserverGroup` dispatches all in-tx observers in parallel via `errgroup`. No inter-observer dependencies are supported — observers must not depend on each other's execution.

### Design rationale

The design rationale for the two-phase observer model (in-tx vs post-commit) and the three policy variants is documented in `core-module/docs/architecture/cross-cutting-design-rationale.md`. That document explains why Pattern A (typed interface, constructor-injected) was chosen over decorator chains, hook buses, repo hooks, and HTTP middleware.

---

## 8. Validation rules

The compiler enforces these rules at generation time. All errors cause the compiler to exit non-zero before writing any output file.

### V1 — Missing required provider

**Condition:** A `requires.services` entry in any selected module has a `name` that does not match any `provides.services[].name` in any other selected module.

**Error:** `unsatisfied service dependency: module "contacts" requires service "authorizer" but no selected module provides it`

### V2 — Circular dependency in construction graph

**Condition:** The service dependency graph (where a directed edge A → B means "A requires a service provided by B") contains a cycle.

**Error:** `circular service dependency: users → authz → users`

The compiler performs a topological sort on the construction dependency graph. If the sort fails (cycle detected), it reports the cycle and exits.

### V3 — Overlapping migration ranges

**Condition:** Two or more selected modules have migration ranges that overlap (see [§5](#5-migration-range-rules)).

**Error:** `migration range conflict: module "users" (100–250) overlaps with module "contacts" (200–399)`

### V4 — Unknown arg-source prefix

**Condition:** An arg-source string in any `args:` list uses a prefix that is not one of the 9 defined forms (see [§4](#4-arg-source-vocabulary)).

**Error:** `unknown arg-source prefix "database:" in module "audit" service "auditObserver" arg 0`

### V5 — Malformed YAML

**Condition:** Any manifest file fails to parse as valid YAML, or fails to unmarshal into the manifest schema (unexpected field types, missing required fields).

**Error:** `failed to parse moduleforge.module.yaml in "audit-module": yaml: line 4: mapping values are not allowed in this context`

### V6 — Non-generated file in outputDir

**Condition:** `outputDir` exists and contains a `main.go` file that does not begin with the generated file header comment `// Code generated by mfgen. DO NOT EDIT.`. Additionally, the compiler resolves `outputDir` to an absolute path and verifies it is a descendant of the directory where `mfgen` is invoked. A path that escapes the project root (via `..` or an absolute path) is a compile error.

**Error (non-generated file):** `outputDir "cmd/app" contains a non-generated main.go; refusing to overwrite. Remove the file or add the generated header to indicate it is safe to overwrite.`

**Error (path traversal):** `outputDir "<value>" escapes the project root; only paths within the project tree are permitted.`

The compiler never overwrites non-generated files. The presence of any non-generated file in `outputDir` is a hard error. An empty `outputDir` (or one that does not yet exist) is fine — the compiler creates it.

### V7 — Invalid identifier in constructor or arg-source

**Condition:** The `constructor` field in any `provides.services`, `provides.routes`, `provides.middleware`, `provides.observers`, or `infra:` entry, or the value portion of any `symbol:`, `method:`, or `field:` arg-source, does not match the Go qualified-identifier pattern: one or more dot-separated segments, each matching `[a-zA-Z_][a-zA-Z0-9_]*`. Values containing newlines, spaces, shell metacharacters, or any character outside this grammar are a compile error.

**Error:** `invalid identifier "<value>" in module "<name>": must be a Go qualified identifier (e.g. pkg.Constructor)`

### V8 — Invalid module path in goModule

**Condition:** The `goModule` field in `moduleforge.module.yaml` or `moduleforge.app.yaml` contains characters outside the set permitted by the Go module path grammar: lowercase or mixed-case letters, digits, dots, hyphens, underscores, and forward slashes. Whitespace, newlines, or template metacharacters (`{`, `}`, `%`) are not permitted.

**Error:** `invalid goModule path "<value>" in "<manifest-file>": not a valid Go module path`

---

## 9. Generation pipeline

The compiler runs five sequential stages. Each stage must complete successfully before the next begins. A failure in any stage causes the compiler to exit with a non-zero status and print a human-readable error; no output files are written.

### Stage 1: Parse

**Input:** The `moduleforge.app.yaml` file in the current working directory.

**Actions:**
1. Load and unmarshal `moduleforge.app.yaml` into the app manifest struct.
2. For each entry in `modules:`, resolve `localPath` relative to the app manifest's directory; if it exists on disk, load and unmarshal `<localPath>/moduleforge.module.yaml` into a module manifest struct. If `localPath` is absent or missing on disk, the compiler errors (published-mode resolution is reserved).
3. Collect all manifests into an in-memory set.

**Output:** A populated set of manifest structs: one `AppManifest` and N `ModuleManifest` values.

**Failures:** V5 (malformed YAML), unreadable file.

### Stage 2: Validate

**Input:** The populated manifest structs from stage 1.

**Actions:**
1. Run all validation rules (V1–V5, V7, V8) against the combined manifest set.
2. Collect all validation errors.
3. Report all errors at once (fail-all, not fail-fast on first error) so the author can fix everything in one pass.

**Output:** If all rules pass, validation OK. If any rule fails, the compiler prints all errors and exits.

**Failures:** V1–V5, V7, V8.

### Stage 3: Resolve

**Input:** Validated manifest structs.

**Actions:**
1. Build the service dependency graph: for each `requires.services` entry, create a directed edge from the requiring module to the providing module.
2. Collect the infra dependencies: for each `requires.infra` entry, record the dependency on the named infra singleton.
3. Resolve all arg-sources: for each `args:` entry in every `provides.services`, `provides.routes`, `provides.observers`, `provides.middleware`, and `infra:` block, resolve the arg-source to a concrete Go expression.
4. Validate that all referenced variables (from `infra:`, `service:`, `field:`, `method:`, `symbol:`, `txQueryFactory:`) exist in the resolved dependency graph.

**Output:** A fully resolved dependency graph: each node is a named value (infra singleton, service, handler, observer) annotated with its Go expression and the ordered list of resolved argument expressions.

**Failures:** V1 (unresolved service reference), V4 (unknown arg-source prefix), missing variable reference.

### Stage 4: Sort

**Input:** The fully resolved dependency graph from stage 3.

**Actions:**
1. Perform a topological sort on the construction dependency graph.
2. Produce a total ordering of all named values such that each value is constructed before any value that depends on it.

**Output:** An ordered list of construction steps: `[(varName, goExpression, [argExpression, ...])]` in dependency order.

**Failures:** V2 (circular dependency detected during sort).

### Stage 5: Render

**Input:** The ordered construction list from stage 4, plus the validated manifests.

**Actions:**
1. Validate `outputDir` for non-generated files (V6 check).
2. Create `outputDir` if it does not exist.
3. Execute the `main.go` Go template, substituting:
   - Package imports derived from all `goModule` paths referenced in arg-sources and constructors
   - The ordered construction steps as variable assignments
   - The router mount block, built using the routing scope matching rules from [§6](#6-routing-scope-matching)
   - The `ObserverGroup` construction, wrapping all observer services in declaration order
4. Execute the `go.mod` template, substituting the app's `goModule` path and the Go version.
5. Write `main.go` and `go.mod` to `outputDir`.
6. Prepend the generated file header `// Code generated by mfgen. DO NOT EDIT.` to `main.go`.

**Output:** A complete, compilable `main.go` and `go.mod` in `outputDir`.

**Failures:** V6 (non-generated file present), template execution error, filesystem write error.

---

## 10. Worked examples

### 10.1 audit-module — moduleforge.module.yaml

Audit-module is the simple case: one service, one observer, one route group, no cross-module service dependencies. Its migration range is 400–499.

```yaml
module: audit
goModule: github.com/moduleforge/audit-api

go:
  api:
    modulePath: github.com/moduleforge/audit-api
    dir: ./api
  model:
    modulePath: github.com/moduleforge/audit-model
    dir: ./model

migrations:
  range:
    first: 400
    last: 499

provides:
  services:
    # auditObserver is the MutationObserver that writes audit_log rows
    # inside the operation's transaction. It is constructed with a
    # txQueryFactory because it needs a tx-scoped *auditdb.Queries at
    # observe time, not a pool-backed one.
    - name: auditObserver
      type: "*auditservice.Observer"
      constructor: auditservice.New
      args:
        - txQueryFactory:auditdb   # func(pgx.Tx)*auditdb.Queries — passed to New

    # auditReadService provides the AuditServicer used by auditHandler.
    # It is listed separately from auditObserver because reading and
    # observing are distinct constructor shapes.
    - name: auditReadService
      type: "audit.AuditServicer"
      constructor: auditservice.NewReadService
      args:
        - queries:auditdb   # auditdb.New(pool) — pool-backed read queries

  # auditObserver implements MutationObserver and is registered in the
  # app's ObserverGroup. policy: propagate means audit failures abort
  # the operation (MustObserve call variant). Audit rows must be
  # transactionally consistent with the data change.
  observers:
    - name: auditLog
      service: auditObserver
      policy: propagate

  # auditHandler serves GET /v1/audit, /by-actor/{uuid}, /by-entity/{entity_uuid}.
  # RegisterRoutes is used because audit-module exposes a registration
  # function, not a NewRouter that returns a chi.Router.
  routes:
    - prefix: /v1/audit
      handler: auditHandler
      constructor: audithttpapi.NewAuditHandler
      args:
        - service:auditReadService   # AuditServicer — the read service, separate from the observer
      register: audithttpapi.RegisterRoutes
      scope: authenticated

requires:
  services:
    - name: authorizer       # authz.Authorizer from authz-module
    - name: coreQuerier      # coredb.Querier from core-module
  infra:
    - name: pool             # *pgxpool.Pool
```

**Annotations:**

- `txQueryFactory:auditdb` is the standard pattern for observers that must write inside the operation transaction. The compiler generates `func(tx pgx.Tx) *auditdb.Queries { return auditdb.New(tx) }` and passes it to `auditservice.New`.
- `policy: propagate` means the audit observer uses `MustObserve(...)` at all service call sites — an audit failure aborts and rolls back the operation.
- `register: audithttpapi.RegisterRoutes` tells the compiler to call `audithttpapi.RegisterRoutes(r, auditHandler)` inside a `r.Route("/v1/audit", ...)` block rather than using `r.Mount`.

---

### 10.2 users-module — moduleforge.module.yaml

Users-module is the complex case: multiple services, multiple route groups, cross-module dependencies on core, audit, and authz, the `txQueryFactory:` arg-source for the audit observer wiring, and the first-user observer hook.

After the phase-3 refactors, users-module exposes route groups via `RegisterRoutes`-style methods. This example reflects the post-refactor shape that the compiler targets.

```yaml
module: users
goModule: github.com/moduleforge/users-module

go:
  api:
    modulePath: github.com/moduleforge/users-module/api
    dir: ./api
  model:
    modulePath: github.com/moduleforge/users-module/model
    dir: ./model

migrations:
  range:
    first: 100
    last: 199

provides:
  services:
    # The local Authorizer — implements authz.Authorizer using the grants table.
    - name: authorizer
      type: "authz.Authorizer"
      constructor: localAuthz.New
      args:
        - queries:authzdb          # authzdb.New(pool)
        - service:operationRegistry  # *authzapi.OperationRegistry from authz-module
        - infra:pool               # *pgxpool.Pool (needed for wildcard-grant check)

    # The UserAccountService — manages user account CRUD.
    - name: userAccountService
      type: "*usersservice.UserAccountService"
      constructor: usersservice.NewUserAccountService
      args:
        - infra:pool
        - queries:usersdb
        - queries:coredb
        - service:authorizer
        - service:observerGroup
        - service:naturalPersonService   # NaturalPersonServicer from core-module
        - service:typeResolver
        - symbol:auth.HashPassword       # bare function reference

    # The firstUserHook function — invoked after the first user account is
    # created to bootstrap the wildcard manage grant. Declared as a service
    # so it can be injected into auth handlers.
    - name: firstUserHook
      type: "func(context.Context, int64) error"
      constructor: users.NewFirstUserHook
      args:
        - infra:pool
        - service:grantService           # GrantServicer from authz-module

    # The oauthOrchestrator manages OIDC provider onboarding flow.
    # Provided by users-module itself; consumed by the /v1/oidc-config handler.
    - name: oauthOrchestrator
      type: "*oauth.Orchestrator"
      constructor: oauth.NewOrchestrator
      args:
        - queries:usersdb
        - field:cfg.Providers

  routes:
    # Health endpoints — no authentication required.
    - prefix: /healthz
      handler: healthHandler
      constructor: handlers.Live
      args: []
      scope: public

    # Local auth endpoints — gated by OIDC confirmed state but not bearer auth.
    - prefix: /v1/auth
      handler: authHandler
      constructor: authhandlers.New
      args:
        - infra:pool
        - queries:usersdb
        - queries:coredb
        - field:cfg.LocalAuth.JWTSecret
        - field:cfg.LocalAuth.LocalIssuer
        - service:emailSender
        - field:cfg.Server.GUIBaseURL
      register: authhandlers.RegisterRoutes
      scope: public

    # Authenticated user-account management routes.
    - prefix: /v1/user-accounts
      handler: usersHandler
      constructor: handlers.NewUserAccountsHandler
      args:
        - service:userAccountService
        - service:grantAdminFn
        - service:revokeAdminFn
      register: handlers.RegisterUserAccountRoutes
      scope: verified

    # OIDC config / onboarding routes. Gated separately — must be reachable
    # even when state is unconfirmed.
    - prefix: /v1/oidc-config
      handler: onboardingHandler
      constructor: handlers.NewOIDCConfigHandler
      args:
        - queries:usersdb
        - service:oauthOrchestrator
        - field:cfg.Providers
        - field:cfg.Onboarding.TokenDisplay
      register: handlers.RegisterOIDCConfigRoutes
      scope: public

    # Self endpoints — GET requires auth but not verification; PUT requires verification.
    - prefix: /v1/self
      handler: selfHandler
      constructor: handlers.NewSelfHandler
      args:
        - queries:usersdb
        - queries:coredb
        - service:coreServices
      register: handlers.RegisterSelfRoutes
      scope: authenticated

requires:
  services:
    - name: observerGroup         # *observer.ObserverGroup (assembled by compiler)
    - name: coreServices          # *coreservice.Services from core-module
    - name: naturalPersonService  # NaturalPersonServicer from core-module
    - name: typeResolver          # *types.Resolver from core-module
    - name: operationRegistry     # *authzapi.OperationRegistry from authz-module
    - name: grantService          # GrantServicer from authz-module
    - name: emailSender           # EmailSender — email delivery service (external provider)
    - name: grantAdminFn          # func(ctx, userID) error — provided by authz-module
    - name: revokeAdminFn         # func(ctx, userID) error — provided by authz-module
  infra:
    - name: pool
    - name: cfg                   # *config.Config
```

**Key annotations:**

- `symbol:auth.HashPassword` passes the hash function as a value, not a call. The compiler emits `auth.HashPassword` (no parens). Used where a constructor takes a `func(string) (string, error)`.
- `field:cfg.LocalAuth.JWTSecret` navigates the config struct: `cfg.LocalAuth.JWTSecret`. The compiler derives the `cfg` variable from `infra:cfg`.
- `service:observerGroup` references the `ObserverGroup` that the compiler assembles from all modules' `provides.observers`. This service name is reserved and generated automatically by the compiler.
- `txQueryFactory:auditdb` (seen in audit-module's worked example) generates the closure `func(tx pgx.Tx) *auditdb.Queries { return auditdb.New(tx) }` and passes it to the observer constructor.
- The `firstUserHook` service is injected into auth handlers so they can bootstrap the first-user wildcard grant after account creation. It is declared as a service with a function type.

---

### 10.3 App deployment — moduleforge.app.yaml

This example selects all six existing modules for a full deployment.

```yaml
app: moduleforge-app
goModule: github.com/myorg/moduleforge-app

outputDir: cmd/app

modules:
  - module: github.com/moduleforge/core-api
    localPath: ./core-module
  - module: github.com/moduleforge/users-module/api
    localPath: ./users-module
  - module: github.com/moduleforge/audit-api
    localPath: ./audit-module
  - module: github.com/moduleforge/authz-api
    localPath: ./authz-module
  - module: github.com/moduleforge/contacts-api
    localPath: ./contacts-module
  - module: github.com/moduleforge/tags-api
    localPath: ./tags-module

config:
  auth:
    adminRole: admin
    requireStepUp: false
    oauthRedirectBaseURL: https://api.example.com
    frontendReturnURL: https://app.example.com/auth/oidc/return
    jwtSecret: ${JWT_SECRET}          # placeholder; resolved from env at runtime
    localIssuer: https://api.example.com
  server:
    addr: :8080
    shutdownTimeout: 30s
    corsOrigins: https://app.example.com
    guiBaseURL: https://app.example.com
  smtp:
    host: smtp.example.com
    port: 587
    from: noreply@example.com
  onboarding:
    tokenDisplay: both
  deployMode: k8s

infra:
  pool:
    type: "*pgxpool.Pool"
    constructor: localdb.New
    args:
      - context
      - field:cfg.DB
  cfg:
    type: "*config.Config"
    constructor: config.Load
    args: []
```

**Key annotations:**

- Each `module:` value is the Go module path of the selected module's `api` sub-module. It must match `go.api.modulePath` in the corresponding `moduleforge.module.yaml`.
- Each `localPath:` value is resolved relative to the directory containing `moduleforge.app.yaml`. When the directory exists on disk, the compiler reads the module manifest from `<localPath>/moduleforge.module.yaml` and emits `replace` directives in the generated `go.mod` for both the `api` and `model` sub-modules declared in the manifest's `go:` section.
- `infra:pool` is the shared `*pgxpool.Pool`. Every module that lists `infra:pool` in `requires.infra` gets this singleton. The compiler constructs it once.
- `infra:cfg` makes the loaded config available as a named infra singleton. Modules reference specific fields via `field:cfg.LocalAuth.JWTSecret`, `field:cfg.Auth.AdminRole`, etc.
- `config:auth.adminRole` resolves to `cfg.Auth.AdminRole` in the generated code.
- The `outputDir: cmd/app` means the compiler writes `cmd/app/main.go` and `cmd/app/go.mod`. If `cmd/app/main.go` already exists and does not contain the generated header, the compiler exits with V6.

---

## Appendix A: core-module special case — `mountFromModule`

Core-module exposes `NewRouter(deps) chi.Router` rather than a `RegisterRoutes` function. This is because core-module's handler set is large and its dependencies are expressed via a `Deps` struct rather than individual arguments.

In `moduleforge.module.yaml` for core-module, the route entry uses `mountFromModule:` instead of `register:`:

```yaml
provides:
  routes:
    - prefix: /v1
      mountFromModule: corehttpapi.NewRouter
      args:
        - service:coreDeps    # corehttpapi.Deps struct
      scope: verified
```

The compiler emits:

```go
r.Mount("/v1", corehttpapi.NewRouter(coreDeps))
```

This pattern applies to any module that returns a full `chi.Router` from a `New*` constructor. The contacts-module and tags-module use the same pattern. The distinction from `register:` is:

- `mountFromModule:` — the constructor returns a `chi.Router` that is mounted at `prefix`
- `register:` — the constructor is a `RegisterRoutes(r chi.Router, handler)` function that mounts routes onto an existing router

---

## Appendix B: reserved service names

The compiler generates the following services automatically. These names must not be used by any module's `provides.services`:

| Name | Type | Generated from |
|---|---|---|
| `observerGroup` | `*observer.ObserverGroup` | All `provides.observers` entries across all selected modules |
| `ctx` | `context.Context` | The boot context passed to `main` |
| `pool` | matches `infra.pool.type` | The `infra:pool` singleton (if declared in app.yaml) |

---

## Appendix C: generated file header

All files written by `mfgen` begin with:

```
// Code generated by mfgen. DO NOT EDIT.
```

The presence of this line on the first line of `main.go` in `outputDir` is the compiler's signal that the file is safe to overwrite. Any `main.go` that does not begin with this exact line is treated as non-generated (validation rule V6).

---

## Known gaps

The following patterns are not yet representable in the manifest format. Each is a confirmed requirement discovered during the authoring of real module and application manifests. They are deferred to the phase-4 compiler-design work and must not be inferred from example files until a spec revision documents the final form.

### 1. Post-construction / deferred method wiring

**What it is.** Some services require a two-phase initialization: the object is constructed normally, but one or more collaborators are injected *after construction* by calling a setter method on the already-constructed instance. The current arg-source vocabulary (§4) covers only constructor *inputs*; there is no form for "after constructing X, call X.SetY(z)."

**Where it appears.** `users-module` and `core-module` both carry `TODO(phase-4/compiler-design)` markers on services that need this pattern. The canonical example is `userResolver`: it is constructed with `nil` as its observer argument, then `SetObserverGroup` is called on it after the observer group is assembled. A `firstUserGrant` hook service similarly needs to register itself onto `authHandler` and `userResolver` via setter calls after both are constructed.

**Proposed sketch (not yet valid format).**

```yaml
services:
  userResolver:
    constructor: resolver.NewUserResolver
    args:
      - source: service
        name: naturalPersonService
      - source: nil          # deferred; set via SetObserverGroup after construction
    postConstruct:
      - method: SetObserverGroup
        args:
          - source: generated
            name: observerGroup
```

The `nil` arg-source and `postConstruct` block are placeholders for the eventual design. The compiler must topologically sort post-construction calls alongside regular dependency resolution.

### 2. Composition-root inline closures

**What it is.** Some wiring points require a small inline function (closure) that captures one or more already-constructed services and adapts their interface for a consumer. These closures have no module-level identity — they exist only at the composition root — and cannot be expressed as named services or `symbol:` references because they are anonymous function literals.

**Where it appears.** `users-module` requires two such closures: `grantAdminFn` and `revokeAdminFn`, each adapting a method of `authzClient` into a `func(ctx, userID)` signature expected by `userResolver`. Both are flagged `TODO(phase-4/compiler-design): inline closure; not representable as a module service or symbol today`.

**Proposed sketch (not yet valid format).**

```yaml
# In moduleforge.app.yaml, under a future `closures:` or `adapters:` section:
closures:
  grantAdminFn:
    signature: "func(context.Context, string) error"
    body: "authzClient.GrantAdmin"   # method reference with automatic ctx/arg threading
  revokeAdminFn:
    signature: "func(context.Context, string) error"
    body: "authzClient.RevokeAdmin"
```

The final form may be a method-reference shorthand, a lambda expression, or a generated adapter type. This requires a phase-4 compiler-design decision on how much Go expression power the manifest should expose.

### 3. Conditional route mounting (app-level config predicate)

**What it is.** Some routes should be mounted only when a specific app-level configuration value satisfies a predicate. The current spec §6 route model derives all routes unconditionally from `provides.routes[]`. There is no mechanism for a module to say "mount this route only if `cfg.X != Y`," because modules do not have access to app-level config keys at manifest-resolution time — the full config tree is only available at the composition root.

**Why it must be app-level.** A per-module `provides.routes[]` entry cannot express a condition that references app-level config, because the module manifest is authored and resolved before the app config shape is known. The predicate must live in the app manifest, where the full config tree is in scope.

**Where it appears.** The `users-module` example app required conditional mounting of a token-display route: show `GET /v1/self/token` only when `cfg.Onboarding.TokenDisplay != config.TokenDisplayNone`. This condition references an app-level config key that no individual module manifest can observe.

**Proposed sketch (not yet valid format).**

```yaml
# In moduleforge.app.yaml, under a future top-level `routing:` section:
# Mount GET /v1/self/token only when onboarding token display is enabled.
routing:
  - module: users
    handler: accountHandler
    routes:
      - method: GET
        path: /v1/self/token
        condition: cfg.Onboarding.TokenDisplay != config.TokenDisplayNone
```

The `condition:` field holds a Go boolean expression evaluated at composition-root generation time. If the condition is a compile-time constant (i.e., the config value is a literal in `moduleforge.app.yaml`'s `config:` block), the compiler can resolve it statically and either include or omit the route from the generated output. If the config value is a runtime variable, the generated code wraps the `router.Handle(...)` call in an `if` guard.

**Open questions for phase-4 design.**
- Should `condition:` be a restricted predicate language (config-key comparisons only) or allow arbitrary Go boolean expressions? The former is safer and statically analyzable; the latter is more powerful but harder to validate.
- Should conditionally-mounted routes still appear in generated OpenAPI specs (as optional/tagged) or be omitted entirely?
- Can `condition:` be expressed at the module level by passing config values through as `provided` flags, or is app-level always required?

### 4. `go:` section — Go sub-module path declarations

Resolved. See [§2 `go` field reference](#go). The `go:` section is now load-bearing and consumed by the compiler for import-path derivation and `replace`/`require` directive generation.

### 5. `gui:` section — GUI component package declarations (phase-4 deferred)

**What it is.** A top-level `gui:` section in `moduleforge.module.yaml` that declares the module's GUI component package: its yalc/npm package name and version. Intended to let the compiler (or a companion tool) assemble the frontend dependency set the same way it assembles the backend service graph.

**Where it appears.** Not present in any current module manifest. Reserved for phase-4 frontend-integration work.

**Proposed sketch (not yet valid format).**

```yaml
gui:
  package: "@moduleforge/audit-ui"
  version: "1.2.0"
```

**Status.** Phase-4 deferred. No compiler support exists. The final field names and whether versioning is managed here or via a lockfile are open design questions.

### 6. `openapi:` section — OpenAPI spec file declarations (phase-4 deferred)

**What it is.** A top-level `openapi:` section in `moduleforge.module.yaml` that declares one or more OpenAPI spec files contributed by the module. Intended to let a companion documentation-generation tool merge per-module specs into a single app-level OpenAPI document.

**Where it appears.** Not present in any current module manifest. Reserved for phase-4 documentation-generation work.

**Proposed sketch (not yet valid format).**

```yaml
openapi:
  - path: api/openapi/audit.yaml
    prefix: /v1/audit
```

**Status.** Phase-4 deferred. No tooling support exists. Open questions include: whether the compiler validates spec files against the route entries declared in `provides.routes`, and whether path prefixes are derived from `provides.routes[].prefix` automatically or must be restated here.
