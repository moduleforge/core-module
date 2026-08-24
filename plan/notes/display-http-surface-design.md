# Display HTTP surface design

## Purpose and scope

Records the concrete endpoint, response, error-mapping, and authorization decisions for exposing
`api/display.Registry` over HTTP, derived from the conventions already in `api/httpapi`,
`api/apiresp`, and `docs/mf-standards/architecture/api-response-design.md`. This note is the
reference the phase's task documents point at rather than re-deriving.

## What already exists

- `api/display/registry.go` — `Registry.Render(ctx, tx, entityID, fieldName) (string, error)`,
  keyed on `(fundamental_type_slug, fieldName)`; returns an error wrapping
  `display.ErrRendererNotRegistered` when nothing is wired for that pair. Field constants:
  `display.FieldName` (`"name"`), `display.FieldDescription` (`"description"`).
- `api/service/display_builtins.go` — `RegisterBuiltins(reg, q)` wires `natural_person`,
  `corporation`, and `service_account` for both fields.
- **No production code anywhere constructs a `display.Registry`.** A repository-wide grep for
  `display.NewRegistry` / `RegisterBuiltins` finds only `api/display/registry_test.go` and the
  definition itself. The mechanism is live-tested but never wired.
- **No HTTP route reaches `Render`.** `api/httpapi/router.go`'s `/entities` block has
  `POST/GET/PUT` per subtype plus generic `GET /{uuid}` and `DELETE /{uuid}`.
- mod-core has **no** `api/cmd` or any other standalone server binary. Its only composition path is
  the mfgen-generated app `main.go` driven by `moduleforge.module.yaml`.

## Endpoint shape

`GET /entities/{uuid}/display-name`, registered **inside** `httpapi.NewRouter`'s existing
`r.Route("/entities", ...)` block (mounted by the composing app at `/v1`, giving
`GET /v1/entities/{uuid}/display-name`).

**Why inside `NewRouter` rather than a separate `Register*Routes` entry.** `apps.go` and
`field_crypto_keys.go` use the standalone-handler + `register:`-onto-`/v1` pattern because they own
*new top-level* prefixes (`/apps`, `/field-crypto-keys`) that do not overlap the router mounted at
`/` inside the same `/v1` chi group. A second registration of `/entities/{uuid}/display-name` into
that same group would put an explicit `entities` trie node alongside the `Mount("/")` catch-all that
already serves every `/entities/*` route, risking a chi routing-precedence change (or panic) for the
*existing* `/entities/*` surface. Registering the route where every other `/entities/*` route already
lives has no such interaction.

**Why `display-name` specifically and not a generic field endpoint.** `api/httpapi` uses explicit,
named routes throughout; there is no query-parameter-dispatched endpoint anywhere in the package. A
generic `?field=` surface would also let a client name arbitrary registry keys. The service method
underneath still takes a `fieldName`, so a sibling `/description` route is a two-line addition later.

## Response shape

Success (renderer resolved a value):

```json
{ "uuid": "0f5e...", "display_name": "Ada Lovelace" }
```

Unavailable — **only** when the entity resolved and the caller is authorized to read it, but no
value can be rendered: no renderer is registered for this entity's type, or no registry is wired
into this deployment at all:

```json
{ "uuid": "0f5e...", "display_name": null }
```

Both are `200 OK` written through `apiresp.WriteJSON`. Rationale:

- The **top-level `error.code` set is closed** (`api/apiresp/errors.go`;
  `docs/mf-standards/architecture/api-response-design.md` "Reserved core codes"): a module MUST NOT
  introduce a new top-level code. "No renderer registered" is not any of `unauthenticated` /
  `forbidden` / `not_found` / `invalid_input` / `conflict`, and it is explicitly **not** an error —
  `ErrRendererNotRegistered` is the registry's documented graceful-fallback contract for an entity
  type belonging to a module that is not present in this deployment.
- A `null` `display_name` is the exact signal a GUI needs to fall back to rendering the raw UUID.

An unknown UUID is **not** part of this shape: it is a real error (`403`, masked) produced by the
resolve/authorize chain below, exactly as it is for `GET /entities/{uuid}`.

`kind` / `fundamental_type_slug` is deliberately **not** included: it is not what this endpoint is
for, and a caller authorized to read the entity can already obtain it from `GET /entities/{uuid}`.
Omitting it also keeps the body byte-shape identical between the available and unavailable cases,
so a client parses one struct.

Error responses (via `apiresp.WriteError`):

| Condition | Sentinel | Status |
|---|---|---|
| No effective actor on the request context | `apiresp.ErrUnauthenticated` | 401 |
| `{uuid}` is not a parseable UUID | `apiresp.ErrInvalidInput` | 400 |
| UUID names no entity, **or** names one the caller may not read | `apiresp.ErrForbidden` | 403 |
| Any genuine DB/render failure | unmapped → `internal_error` | 500 |

The single `403` row covers both the nonexistent and the unauthorized case deliberately: that
masking is the pre-existing, module-wide behaviour of `entity.Resolver.Resolve`'s default policy
plus `az.Authorize`, not something this endpoint invents.

## Authorization decision

**Rule: real `"read"` authorization on the target entity — the same gate as every other
single-entity read in mod-core.** Merely holding a UUID entitles a caller to nothing. The service
method runs the identical two-step sequence `EntityService.GetByUUID` and
`EntityService.ResolveProfile` already run (`api/service/entity.go`):

```go
internalID, err := s.entityResolver.Resolve(ctx, q, entityUUID, "entity")
if err != nil {
    return "", false, err // propagated as-is
}
if err := s.az.Authorize(ctx, "read", &internalID); err != nil {
    return "", false, err // propagated as-is
}
```

Precedent and rationale:

- **No bespoke rule, no bespoke code.** Both errors propagate untouched to `apiresp.WriteError`,
  which classifies them: `403` for a resolve miss (the resolver's default masked-404-as-403 policy)
  and `403` for an authorize denial, so a nonexistent UUID and an unauthorized one are
  indistinguishable. This is the exact behaviour the `masked-lookup-403` fix put in place — see
  [plan-summary-masked-lookup-403.md](../plan-summary-masked-lookup-403.md), whose key decision was
  that `api/entity/resolver.go`'s sentinels are aliased directly to the `apiresp` sentinels so
  `errors.Is`-based classification "just works" with **no per-call-site translation code**. Follow
  that precedent: propagate, do not translate, do not collapse.
- **401 vs 403 needs no extra call.** `Authorizer` implementations are contractually required to
  "return distinguishable error values so HTTP handlers can map 'not authenticated' to 401 and
  'authenticated but not allowed' to 403" (`api/authz/authz.go`, `Authorizer` doc comment), and
  return `apiresp.ErrUnauthenticated` themselves when no effective actor is present. That is why
  `EntityService.GetByUUID` carries no separate authentication call, and why this service needs
  none either. `authz.RequireAuthenticated` is therefore **not used by this plan**; it remains an
  available, still-unused general-purpose helper in `api/authz/authz.go` for some future genuinely
  authentication-only case.
- **Consistency is the security property.** An endpoint that disclosed a display name for any UUID
  a caller happens to hold would be a read side-channel around the entity read authorization every
  other route enforces. Whatever the disclosure is bounded to, the caller has to be entitled to it.

### What this means for the original motivating problem (and why it is out of scope)

The problem that prompted this plan is a peer module's GUI rendering a raw UUID where a name
belongs — mod-workflows showing a node's assignee UUID. The fix for that is **not** a direct
display-name lookup from the GUI against a UUID it holds but may not be authorized to read. It is
**response enrichment by the module that already has the authorized read**: mod-workflows' own
node-read endpoint, which is already authorizing and already disclosing the assignee relation,
resolves the related entity's display name and includes it in its own response. Authorization is
established once, by the containing read, at the point where the relation is disclosed.

That enrichment is future work in mod-workflows, separately tracked and not planned here. It does
not change this plan's deliverables — mod-core still ships exactly the same display service,
endpoint, manifest wiring, and documentation, and the display capability is what mod-workflows
would consume.

## Layering

Per `AGENTS.md` Conventions — "Handlers are thin — parse input, call one service method" and
"Authorization is checked first in every service method" — the UUID→internal-ID resolution, the
`az.Authorize(ctx, "read", &id)` gate, the `Render` call, and the `ErrRendererNotRegistered`
mapping all live in a service method. The handler parses the UUID, calls the one method, and shapes
the response.

The handler's own pre-flight `opctx.ActorEntityID` 401 check is the pre-existing convention
`getEntity` already follows and is retained for consistency; it is a cheap short-circuit, not the
authorization gate. The gate is the service's `az.Authorize` call.

The handler must also tolerate an unwired display service (`nil`) without panicking, returning the
same `display_name: null` body. That response is constant across every UUID and so discloses
nothing about any entity. mod-core ships today with no composed app wiring a registry at all, and
`httpapi.NewDeps` (2-arg, manifest-declared and called by peer repos' own servers) must keep
compiling and working unchanged.
