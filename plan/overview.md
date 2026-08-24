# Entity Display Name

## Purpose and scope

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

## Current status

Plan created; no tasks executed. Execution begins at Phase 1 (`display-name-endpoint`), task 001.

Pre-conditions, all verified during planning and all already true on the working branch:

- `api/display/registry.go` and `api/service/display_builtins.go` are present and unchanged.
- `api/service/entity.go`'s `GetByUUID` / `ResolveProfile` show the resolve-then-`Authorize("read")`
  pattern this plan's display service copies verbatim, propagating both errors as-is.
- `api/entity/resolver.go`'s sentinels are aliased to the `apiresp` sentinels, so a masked miss
  already classifies correctly through `apiresp.WriteError` with no per-call-site translation.
- `api/service/service.go`'s `service.New` already takes `az authz.Authorizer`, sourced in the
  manifest as `service:authorizer` — the same arg source the display service constructor will use.
- `api/httpapi/masked_lookup_test.go` establishes the pattern for wiring real services (rather than
  the package's `fake*Service` mocks) into an httpapi-layer test.
- The `docs/mf-standards` submodule is checked out and readable, reference-only.

## Overview

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
