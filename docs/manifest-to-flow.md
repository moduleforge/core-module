# Manifest → Runtime Flow

How `moduleforge.app.yaml` and `moduleforge.module.yaml` sections map into the
generated composition root and the per-request HTTP pipeline.

```mermaid
flowchart LR
    %% ────────────────────────────────────────────────────────────────
    %% LEFT — Manifest specifications
    %% ────────────────────────────────────────────────────────────────
    subgraph APP["moduleforge.app.yaml"]
        direction TB
        a_mods["modules[ ]\nmodule: · localPath:"]
        a_cfg["config:\nkey-value pairs"]
        a_infra["infra:\nname · type · constructor · args"]
        a_close["closures:\nparams · returnType · args · body"]
    end

    subgraph MOD["moduleforge.module.yaml  ×N"]
        direction TB
        m_mig["migrations.range\nfirst · last"]
        m_svc["provides.services[ ]\nname · type · constructor · args"]
        m_obs["provides.observers[ ]\nname · service · policy"]
        m_mw["provides.middleware[ ]\nname · constructor · args"]
        m_rt["provides.routes[ ]\nprefix · scope\nmiddleware[ ] · innerMount\nregister / mountFromModule"]
        m_rsvc["requires.services[ ]"]
        m_rinf["requires.infra[ ]"]
    end

    %% ────────────────────────────────────────────────────────────────
    %% MIDDLE — Startup / Composition root  (mfgen-generated main.go)
    %% ────────────────────────────────────────────────────────────────
    subgraph STARTUP["Startup — generated composition root"]
        direction TB
        s1["① Load config\ninfra: cfg singleton"]
        s2["② Open infra singletons\npool · smtp · opReg …"]
        s3["③ Run migrations\nordered by range.first"]
        s4["④ Construct services\ntopological sort of\nprovides.services + requires.services"]
        s5["⑤ Assemble ObserverGroup\nwraps all provides.observers[ ]"]
        s6["⑥ Emit closures\nclosure: variables"]
        s7["⑦ Build chi.Router\nper-route: scope gate · r.Use(mw) · mount"]

        s1 --> s2 --> s3 --> s4 --> s5 --> s6 --> s7
    end

    %% ────────────────────────────────────────────────────────────────
    %% RIGHT — Per-request flow
    %% ────────────────────────────────────────────────────────────────
    subgraph REQUEST["Per-request flow"]
        direction TB
        r_in(["HTTP Request"])
        r_srv["HTTP Server\nnet/http.ListenAndServe"]
        r_rtr["chi.Router\nprefix matching"]
        r_scope{"Scope gate\npublic · authenticated · verified"}
        r_mw["Route middleware\nr.Use(mw, …)"]
        r_hdl["Handler\nRegisterRoutes / r.Mount"]
        r_svc["Service layer"]
        r_og["ObserverGroup"]
        r_intx["In-tx observers\nObserve(ctx, tx, …)\npropagate → MustObserve ✗ abort\nswallow  → MayObserve  ✓ continue\ndefault  → Observe"]
        r_post["Post-commit observers\nObserveAfterCommit(ctx, …)\nerrors logged, never abort"]
        r_db[("PostgreSQL")]

        r_in --> r_srv --> r_rtr --> r_scope --> r_mw --> r_hdl --> r_svc
        r_svc -- "mutation\n(pgx.Tx)" --> r_og
        r_og --> r_intx
        r_og -.-> r_post
        r_intx -- "writes inside tx" --> r_db
        r_post -.-> r_db
        r_svc -- "reads / writes" --> r_db
    end

    %% ────────────────────────────────────────────────────────────────
    %% App manifest → Startup
    %% ────────────────────────────────────────────────────────────────
    a_infra -- "cfg" --> s1
    a_infra -- "pool · smtp · …" --> s2
    a_mods  -- "module selection\ngo.mod replace directives" --> s3
    a_cfg   -- "field: / config:\narg-source values" --> s4
    a_close -- "closure: arg-source" --> s6

    %% ────────────────────────────────────────────────────────────────
    %% Module manifest → Startup
    %% ────────────────────────────────────────────────────────────────
    m_mig  -- "range.first\n→ run order" --> s3
    m_svc  -- "constructor + args" --> s4
    m_rsvc -- "cross-module\nservice wiring" --> s4
    m_rinf -- "infra binding" --> s2
    m_obs  -- "service + policy\n→ ObserverGroup" --> s5
    m_mw   -- "constructor + args\n→ r.Use(mw)" --> s7
    m_rt   -- "prefix → mount\nscope → gate\nmiddleware → r.Use\nregister / mountFromModule" --> s7

    %% ────────────────────────────────────────────────────────────────
    %% Startup → Request flow
    %% ────────────────────────────────────────────────────────────────
    s7 -- "mounted router" --> r_rtr
    s4 -- "services injected\ninto handlers" --> r_svc
    s5 -- "ObserverGroup\ninjected into services" --> r_og
    s6 -- "closure fn vars\ninjected into handlers" --> r_hdl
```

## Key field-to-flow mappings at a glance

| Manifest field | Where it appears at runtime |
|---|---|
| `infra.pool` | Shared `*pgxpool.Pool` — passed to every service and handler that lists `infra:pool` in `requires.infra` |
| `infra.cfg` | Shared `*config.Config` — fields accessed via `field:cfg.*` arg-sources |
| `closures.*` | Inline `func` variables emitted at the composition root; injected into handlers via `closure:<name>` arg-source |
| `migrations.range` | Determines run-order for migration files at startup (ordered by `range.first`, non-overlapping across modules) |
| `provides.services[]` | Service variables constructed once in topological dependency order; injected via `service:<name>` arg-source |
| `requires.services[]` | Declares cross-module wiring; the compiler validates exactly one provider exists and edges the dependency graph |
| `provides.observers[].service` | The service that implements `MutationObserver`; wrapped into the `ObserverGroup` |
| `provides.observers[].policy` | `propagate` → `MustObserve` (errors abort tx); `swallow` → `MayObserve` (errors logged); `default` → `Observe` |
| `provides.middleware[]` | Middleware constructors; referenced by name in `provides.routes[].middleware` and emitted as `r.Use(mw)` calls |
| `provides.routes[].prefix` | chi mount point — `r.Mount(prefix, ...)` or `r.Route(prefix, ...)` |
| `provides.routes[].scope` | `public` — no gate; `authenticated` — `RequireAuth` middleware; `verified` — `RequireAuth` + `RequireVerifiedEmail` |
| `provides.routes[].middleware[]` | Named middleware applied via `r.Use(...)` inside the route group, innermost scope only |
| `provides.routes[].register` | Pattern A: pre-built handler struct passed to `RegisterRoutes(r, handler)` |
| `provides.routes[].mountFromModule` | Pattern B: `NewRouter(deps)` returns a full `chi.Router`; emitted as `r.Mount(prefix, NewRouter(deps))` |
| `provides.routes[].innerMount` | `true` → emitted inside a parent module's `r.Route(prefix, ...)` block to avoid chi duplicate-mount panic |
