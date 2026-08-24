# Plan Summary: entity-display-name

## What was planned and why

Expose mod-core's existing in-process `api/display.Registry` — the type-keyed `FieldRenderer`
dispatch that already resolves an entity's canonical display-name field — over HTTP, so a browser
GUI client can turn an entity UUID it holds into a human-readable name without mod-core knowing
anything about the downstream module that owns that entity type.

The registry mechanism, its per-type registration model, and its graceful
`ErrRendererNotRegistered` fallback all already exist and are documented as the intended pattern in
`docs/mf-standards/architecture/entity-typing.md` ("Display-rendering pattern"). Two gaps make it
unreachable in practice, and this plan closes both **within mod-core**:

1. **No HTTP endpoint anywhere in `api/httpapi` calls `Render`.** The registry is Go-in-process only.
2. **No production code constructs a `display.Registry` at all** — a repo-wide grep finds
   `display.NewRegistry` and `RegisterBuiltins` only in tests. mod-core has no standalone server
   binary (no `api/cmd`); its only composition path is the mfgen-generated app `main.go` driven by
   `moduleforge.module.yaml`, and that manifest declares no registry today.

### What must change

- `api/service` gains a display service: resolve the public UUID to an internal entity ID, authorize
  `"read"` on it (the same two-step gate every other single-entity read in mod-core uses), call
  `Registry.Render` for `display.FieldName`, and map `ErrRendererNotRegistered` to a non-error
  "unavailable" outcome. Plus a constructor that builds a registry with mod-core's builtins already
  registered.
- `api/httpapi` gains `GET /entities/{uuid}/display-name` inside the existing `/entities` route
  block, a nil-safe `Deps` field for the display service, and an additive three-argument `Deps`
  constructor.
- `moduleforge.module.yaml` declares the registry and display service so an mfgen-composed app
  constructs one shared registry and threads it into the core router.
- `api/openapi.fragment.yaml`, `AGENTS.md`, `README.md`, and a new mod-core-owned architecture doc
  describe the endpoint and the wiring a future composing app (and a future downstream renderer
  registration) follows.

### What must not change

- **`httpapi.NewDeps` (two-argument) and `service.New` keep their exact current signatures.** Peer
  repositories call both from their own servers; every addition here is additive.
- No behavioural change to any existing `/entities/*`, `/apps`, or `/field-crypto-keys` route. The
  new route lives inside the same `/entities` chi block as the existing entity routes precisely so
  the routing trie for the existing surface is untouched — it is deliberately **not** a second
  `register:`-onto-`/v1` entry alongside the router already mounted there.
- Nothing under `docs/mf-standards/` — a git submodule owned by the separate `docs-mf-standards`
  repository — is edited, staged, or committed. See
  [documentation ownership constraint](./notes/docs-ownership-constraint.md).
- No change to `model/`, `gui/`, or any migration.

### Success criteria

- `GET /v1/entities/{uuid}/display-name` returns `200` with
  `{"uuid": "...", "display_name": "<rendered>"}` for an entity of a type with a registered renderer.
- The same route returns `200` with `{"uuid": "...", "display_name": null}` — not an error — when
  the entity resolved and the caller is authorized to read it but no value can be rendered: no
  renderer is registered for that entity's type, or no registry is wired into the deployment at all.
  This is the expected steady state for an entity type owned by a module absent from a given
  deployment. The `200`/`null` shape is reserved for those two cases only.
- `403` when the caller is not authorized to read the target entity **or** the UUID names no entity
  — the two are deliberately indistinguishable, exactly as they already are for
  `GET /v1/entities/{uuid}`. Holding a UUID entitles a caller to nothing; this route requires the
  same real `"read"` authorization as every other single-entity read.
- `401` when the request carries no effective actor; `400` for a malformed UUID.
- mod-core's own tests demonstrate the full chain — real `display.Registry` + `RegisterBuiltins` +
  real service + real router — resolving `natural_person`, `corporation`, and `service_account`.
- `cd api && make test` and `cd api && make lint` pass.

### Hard constraints

- The top-level `error.code` set is closed and owned by mod-core
  (`docs/mf-standards/architecture/api-response-design.md`, "Reserved core codes"); this plan
  introduces no new top-level code. The not-registered case is therefore a `200` shape, not an error.
- `AGENTS.md` conventions hold: internal IDs never appear in an HTTP response (`uuid` only), handlers
  stay thin, authorization is checked first in the service method.

### Deliberately deferred (out of scope, tracked elsewhere)

- Any change to **mod-users** (registering a renderer for its user-owning entity type).
- Any change to **mod-workflows**. The fix for its GUI showing a raw assignee UUID is
  **response enrichment inside mod-workflows' own read endpoints**, not a separate GUI lookup call:
  the node-read endpoint that already authorizes the read and already discloses the assignee
  relation should resolve the related entity's display name — using this display capability
  internally — and include it in its own response, at the point where the relation is disclosed to
  an already-authorized caller. A direct client call against a UUID the GUI merely holds is
  explicitly *not* the path, because this endpoint requires real `"read"` authorization on the
  target entity. That enrichment is a separate, not-yet-planned mod-workflows change; mod-workflows
  followup `98Bq` remains open and is **not** closed by this plan.
- Building or modifying any **composed application binary** wiring mod-core + mod-users +
  mod-workflows. That app does not exist yet and is a separate future planning session; this plan
  documents the wiring it will follow.
- A typed client helper in mod-core's `gui/` package for the new endpoint.
- A sibling `/description` route. The service method takes a field name, so adding one later is
  trivial, but no caller needs it now.

A single implementation phase, plus the standard documentation-updates phase. The four
implementation tasks are strictly sequential — each builds directly on the previous one's exported
surface — so there are no parallel-eligible groups.

### Phase 1 — Display Name Endpoint (`display-name-endpoint`)

1. **`001-add-display-service`** *(sonnet-high)* — `api/service/display.go`: a
   `NewDisplayRegistry(q)` constructor that builds a `display.Registry` and calls `RegisterBuiltins`
   on it, plus a `DisplayServicer` interface and `DisplayService` implementation whose one method
   resolves the UUID through the shared `entity.Resolver`, authorizes `"read"` on the resulting
   internal ID via the injected `authz.Authorizer` — propagating either error as-is, exactly as
   `EntityService.GetByUUID` does — then calls `Render` and reports `ErrRendererNotRegistered` as a
   non-error "unavailable" result. Unit tests cover resolved, not-registered, nil-registry,
   masked-miss (403 propagates), authz-denied, and genuine-render-error paths.

2. **`002-add-display-name-endpoint`** *(sonnet-high)* — `api/httpapi/display.go`: the
   `getDisplayName` handler and its `GET /{uuid}/display-name` route inside the existing
   `/entities` block; a nil-safe `Deps.Display` field; an additive three-argument `Deps` constructor
   leaving the existing two-argument `NewDeps` untouched. Handler tests plus one end-to-end test
   wiring a real registry with builtins through a real service and the real router for all three
   core concrete types.

3. **`003-wire-display-registry-in-manifest`** *(sonnet-med)* — `moduleforge.module.yaml`: the
   `displayRegistry` and display-service `provides.services` entries and the `coreDeps` constructor
   switch, so an mfgen-composed app constructs one shared registry and threads it into the core
   router. Includes a read-only check against the mfgen resolver that every new node is reachable
   from a route root and that the construction order is satisfiable.

4. **`004-document-display-name-surface`** *(sonnet-high)* — the `api/openapi.fragment.yaml` path
   entry, a new mod-core-owned architecture doc covering the endpoint contract and the
   composing-app / downstream-module wiring pattern, `README.md` and `AGENTS.md` links and row
   updates, and a **drafted but uncommitted** amendment to the docs-mf-standards
   `architecture/entity-typing.md` "Display-rendering pattern" section, surfaced as a followup.

### Phase 2 — Documentation Updates (`doc-updates`)

5. **`001-update-architecture-docs`** *(sonnet-high)* — the standard architecture-conformance pass
   over the docs this plan's changes touch, run after the implementation tasks have landed.

### Supporting research

- [Display HTTP surface design](./notes/display-http-surface-design.md) — endpoint and response
  shape, error mapping, and the read-authorization decision with its precedent — plus why the
  mod-workflows fix is response enrichment rather than a direct lookup call.
- [mfgen composition wiring](./notes/mfgen-composition-wiring.md) — the manifest entries, mfgen's
  reachability pruning, and the downstream-module registration pattern.
- [Documentation ownership constraint](./notes/docs-ownership-constraint.md) — why the
  `docs/mf-standards` submodule is not edited and what happens instead.

## What shipped

### Phase 01 — Display Name Endpoint

1. **Add Display Service** (`001-add-display-service.md`, tier `sonnet-high`) — Implemented api/service/display.go: NewDisplayRegistry, DisplayServicer/DisplayService, RenderField mirroring EntityService.GetByUUID resolve-then-authorize sequence, nil-registry tolerance and ErrRendererNotRegistered handling after the authz gate. Added 8 tests covering all cases. Full validation green.
   Commit `2cf384d`, merged at `6dfac45394af341bc885ebf5e7632e64fb0d86b3`.

2. **Add Display Name Endpoint** (`002-add-display-name-endpoint.md`, tier `sonnet-high`) — Implemented GET /entities/{uuid}/display-name in api/httpapi. Deps gained nil-safe Display field and additive NewDepsWithDisplay constructor; NewDeps unchanged. getDisplayName handler mirrors getEntity's thin shape (401/400/nil-Display 200-null/RenderField passthrough). Tests cover all cases incl. end-to-end real-router wiring for three builtin types, no-renderer-null case, and masked-403 case. go build/make test/make lint all green.
   Commit `df223e8`, merged at `bdf6ef869b166f90348657519831358a4e29a662`.

3. **Wire Display Registry In Manifest** (`003-wire-display-registry-in-manifest.md`, tier `sonnet-med`) — Declared displayRegistry and displayService as new provides.services entries in moduleforge.module.yaml matching task 001 constructor signatures, switched coreDeps to three-arg NewDepsWithDisplay. Verified reachability/topological order read-only against mfgen. Updated AGENTS.md display/service/httpapi rows. make test/make lint green, no Go source touched.
   Commit `9293b83`, merged at `bd03c8b539ef5a66b85cf77ef3e52e24a0142a76`.

4. **Document Display Name Surface** (`004-document-display-name-surface.md`, tier `sonnet-high`) — Documented the display-name HTTP surface: added GET /entities/{uuid}/display-name to api/openapi.fragment.yaml, wrote docs/architecture/display-name-http.md covering endpoint contract, mfgen wiring chain, downstream renderer-registration pattern, and peer-module response-enrichment note. Linked from README.md and AGENTS.md. Drafted but did not apply a docs-mf-standards entity-typing.md amendment per submodule ownership constraint (qxLX precedent). make test/make lint green; docs/mf-standards untouched.
   Commit `7f71bbb`, merged at `085fc32f9d3e3f9481a4ef2c69b0cd69b25d76b5`.

### Phase 02 — Documentation Updates

1. **Update Architecture Docs** (`001-update-architecture-docs.md`, tier `sonnet-high`) — Pure reconciliation/verification pass with no code or doc changes required: AGENTS.md, docs/architecture/display-name-http.md, README.md, api/openapi.fragment.yaml, and moduleforge.module.yaml already accurately describe the landed Phase 1 work (task 004 wrote docs directly against shipped code). Verified every claim by grep/read against actual source. Cross-checked conformance against read-only docs/mf-standards submodule without touching it. No further cross-repo amendment needed beyond task 004's existing draft. make test/make lint pass.
   Commit `1f5a4fa`, merged at `7e9efa41978c0f6fc24671458111225c2330544c`.

## Key decisions

_No `## Why this shape` section is recorded in `plan/overview.md`, so this plan's cross-task rationale was never written down. Per-task outcomes are under "What shipped" above._

## Follow-up items

- **`yoUt`** — **Stale cipher-key comment in module.yaml** — moduleforge.module.yaml's `cipher` service comment describes the old "CORE_FIELD_KEY_HEX set explicitly always wins" behavior, which AGENTS.md documents as replaced by fail-loudly-on-mismatch (CORE_FIELD_KEY_HEX is now a first-boot-only bootstrap seed; a later mismatch fails construction rather than silently taking precedence). Flagged incidentally by the entity-display-name planning pass; not touched by that plan (out of scope). Worth a small doc-comment fix to reconcile the manifest comment with current behavior.

- **`JQT8`** — **Pending cross-repo doc amendment (route to do** — Pending cross-repo doc amendment (route to docs-mf-standards), followup type documentation, qxLX-precedent shape: Cross-repo doc amendment awaiting explicit manager/user authorization before being applied to the docs-mf-standards repository (architecture/entity-typing.md, Display-rendering pattern section) — NOT committed anywhere by this task. Verbatim drafted addition: 'HTTP exposure. The display-rendering pattern described above is reachable over HTTP via mod-core's GET /v1/entities/{uuid}/display-name endpoint. The endpoint requires the same real read authorization on the target entity as any other single-entity read in mod-core's API. A readable entity whose type has no renderer registered in the current deployment — the expected steady state for an entity type owned by a module not composed into that deployment — renders as display_name: null with a 200, its documented graceful-fallback contract, never as an error. See mod-core's own docs/architecture/display-name-http.md for the full endpoint contract, how a composed application wires the shared display registry, and how a downstream module registers a renderer for its own entity type.'

- **`mSR8`** — **Informational: task doc 004's scope descripti** — Informational: task doc 004's scope description of docs/architecture/ was stale (described it as pre-existing with only a CLAUDE.md; it did not exist in this worktree). Not a blocker — directory and doc created as required. Phase 1 is now complete so no other task doc is affected.

- **`SO31`** — **Informational: no docs/architecture.md parent** — Informational: no docs/architecture.md parent index exists; new topic doc is reachable via direct links from README.md/AGENTS.md so this is not a compliance gap, just a note that a future task could add the fuller structure.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — Display Name Endpoint

- [x] [001-add-display-service.md](./phase-01-display-name-endpoint/001-add-display-service.md) — tier `sonnet-high` · branch `plan/entity-display-name-01-001` · commit `2cf384d` · merge `6dfac45394af341bc885ebf5e7632e64fb0d86b3`
- [x] [002-add-display-name-endpoint.md](./phase-01-display-name-endpoint/002-add-display-name-endpoint.md) — tier `sonnet-high` · branch `plan/entity-display-name-01-002` · commit `df223e8` · merge `bdf6ef869b166f90348657519831358a4e29a662`
- [x] [003-wire-display-registry-in-manifest.md](./phase-01-display-name-endpoint/003-wire-display-registry-in-manifest.md) — tier `sonnet-med` · branch `plan/entity-display-name-01-003` · commit `9293b83` · merge `bd03c8b539ef5a66b85cf77ef3e52e24a0142a76`
- [x] [004-document-display-name-surface.md](./phase-01-display-name-endpoint/004-document-display-name-surface.md) — tier `sonnet-high` · branch `plan/entity-display-name-01-004` · commit `7f71bbb` · merge `085fc32f9d3e3f9481a4ef2c69b0cd69b25d76b5`

### Phase 02 — Documentation Updates

- [x] [001-update-architecture-docs.md](./phase-02-doc-updates/001-update-architecture-docs.md) — tier `sonnet-high` · branch `plan/entity-display-name-02-001` · commit `1f5a4fa` · merge `7e9efa41978c0f6fc24671458111225c2330544c`
