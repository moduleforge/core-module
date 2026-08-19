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

Unavailable (no renderer registered for this entity's type, no registry wired into this deployment,
or no such entity):

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
- Collapsing "unknown entity" into the same shape avoids turning the endpoint into an existence
  oracle, which matters given the authorization rule below.

`kind` / `fundamental_type_slug` is deliberately **not** included: it would leak type information on
an entity the caller may not be able to read, and it cannot be produced for the unknown-UUID case,
which would break the uniform shape.

Error responses (via `apiresp.WriteError`):

| Condition | Sentinel | Status |
|---|---|---|
| No effective actor on the request context | `apiresp.ErrUnauthenticated` | 401 |
| `{uuid}` is not a parseable UUID | `apiresp.ErrInvalidInput` | 400 |
| Any genuine DB/render failure | unmapped → `internal_error` | 500 |

## Authorization decision

**Rule: authentication-only.** The service method gates on `authz.RequireAuthenticated(ctx)` rather
than `az.Authorize(ctx, "read", &internalID)`.

Precedent and rationale:

- `authz.RequireAuthenticated` (`api/authz/authz.go`) was added deliberately as an
  authentication-only free function — "it answers 'is there an effective actor on this context at
  all?', not 'may this actor perform this operation?'". It has **no production call site in mod-core
  today**; this endpoint is the case it was built for.
- The driving use case is a peer module's GUI resolving an entity UUID it already holds as a foreign
  key in its own data (mod-workflows' assignee UUID). Gating on `"read"` against the *entity* would
  403 the overwhelmingly common case, since an actor authorized to see a workflow is generally not
  authorized to read the assignee's entity profile — which would make the endpoint useless for its
  stated purpose.
- The capability boundary is the UUID itself: a v4 UUID is unguessable, and a caller holding one
  obtained it from data it was already authorized to read. The uniform `display_name: null` response
  for unknown UUIDs keeps the endpoint from confirming existence for a UUID the caller guessed.
- The disclosure surface is bounded to a single rendered string per entity — the same string every
  downstream GUI already renders in list and detail views.

**This is the one security-relevant judgment call in the plan and it is flagged for the manager.**
It must be implemented as a single, clearly commented line in one service method so that tightening
it to `Authorize(ctx, "read", &id)` is a one-line change, and it must be stated explicitly in both
the endpoint doc and the OpenAPI fragment description.

## Layering

Per `AGENTS.md` Conventions — "Handlers are thin — parse input, call one service method" and
"Authorization is checked first in every service method" — the authenticated check, UUID→internal-ID
resolution, `Render` call, and `ErrRendererNotRegistered` mapping all live in a service method. The
handler parses the UUID, calls the one method, and shapes the response.

The handler must also tolerate an unwired display service (`nil`) without panicking, returning the
same `display_name: null` body. mod-core ships today with no composed app wiring a registry at all,
and `httpapi.NewDeps` (2-arg, manifest-declared and called by peer repos' own servers) must keep
compiling and working unchanged.
