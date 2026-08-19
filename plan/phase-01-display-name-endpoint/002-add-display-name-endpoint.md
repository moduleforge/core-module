# Add Display Name Endpoint

## Purpose and scope

Expose the display service from task 001 as `GET /entities/{uuid}/display-name` — the HTTP surface a
browser GUI calls to turn an entity UUID into a human-readable name. Adds the handler, the route, a
nil-safe dependency field, and an additive `Deps` constructor, then proves the whole chain works
end-to-end for mod-core's three builtin concrete types.

Scope is `api/httpapi` only: `api/httpapi/display.go` (new), `api/httpapi/router.go` (edit),
`api/httpapi/display_test.go` (new), and whatever minimal edit `api/httpapi/mock_test.go`'s
`buildTestDeps` helper needs. No manifest change and no prose documentation — tasks 003 and 004 own
those. No standard skill covers this; follow the procedure below.

## Requirements

### 1. `Deps` gains a nil-safe display field

In `api/httpapi/router.go`:

- Add an exported field to `Deps`, typed as the **interface** from task 001
  (`Display service.DisplayServicer`), not the concrete struct, so tests can substitute a fake.
- **Leave `NewDeps(svcs, logger)` byte-for-byte unchanged.** It is declared in
  `moduleforge.module.yaml` and called by peer repositories' own servers; changing its signature is
  a cross-repo break. A `Deps` built by it simply has a nil `Display`.
- Add an additive constructor alongside it — `NewDepsWithDisplay(svcs *service.Services,
  logger *slog.Logger, dsp service.DisplayServicer) Deps` — documented as the constructor a
  composing app uses when it wants the display-name route to resolve. Its doc comment states that
  `NewDeps` remains valid and yields the graceful unavailable behaviour.

### 2. The route

In `NewRouter`'s existing `r.Route("/entities", ...)` block, add:

```go
r.Get("/{uuid}/display-name", h.getDisplayName)
```

Place it with the generic entity routes (near `r.Get("/{uuid}", ...)`) and extend the existing
"Generic entity routes — must come after typed sub-paths" comment to cover it.

Do **not** add a second `Register*Routes`-style entry mounted onto `/v1` for this route. The reason
is load-bearing and must be captured in a short code comment: `apps.go` and `field_crypto_keys.go`
use that pattern because they own *new top-level* prefixes; registering `/entities/...` a second
time into the same chi group that already `Mount`s this router at `/` would introduce a competing
`entities` trie node alongside the catch-all and can change routing precedence for the existing
`/entities/*` surface.

### 3. `getDisplayName` handler

New file `api/httpapi/display.go`, following `api/httpapi/entities.go`'s `getEntity` shape exactly
(thin handler: parse, one service call, shape response):

1. If `opctx.ActorEntityID(r.Context())` reports no actor, `apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)`
   and return — mirroring `getEntity`'s first lines. (The service re-checks via
   `authz.RequireAuthenticated`; both layers checking matches the existing pattern.)
2. Parse `chi.URLParam(r, "uuid")` with `uuid.Parse`; on error
   `apiresp.WriteError(w, r, apiresp.ErrInvalidInput)` (400) and return.
3. If `h.d.Display` is nil, write the unavailable body (below) with `200` and return — no panic, no
   500. A deployment with no registry wired is a valid state.
4. Otherwise call
   `h.d.Display.RenderField(r.Context(), h.d.Services.Querier(), entityUUID, display.FieldName)`.
   On a non-nil error, `apiresp.WriteError(w, r, err)` and return.
5. Write `200` via `apiresp.WriteJSON` with the body below.

Response body — a small named struct with explicit JSON tags, not a bare map (`display_name` must
serialize as JSON `null`, not be omitted, when unavailable; use `*string`):

```json
{ "uuid": "<the requested uuid>", "display_name": "Ada Lovelace" }
```

```json
{ "uuid": "<the requested uuid>", "display_name": null }
```

The unavailable body is returned identically for: no renderer registered for the entity's type, no
registry wired into this deployment, and a UUID naming no entity. Do **not** add `kind`,
`fundamental_type_slug`, or any other field — it would leak type or existence information about an
entity the caller may not be able to read, and it could not be produced for the unknown-UUID case.
Never include an internal entity ID (`AGENTS.md` Conventions).

Carry a doc comment on the handler stating the authentication-only authorization rule and that the
`200`/`null` shape is the deliberate graceful-fallback contract, not a missing error case.

### 4. Tests

Update `buildTestDeps` in `api/httpapi/mock_test.go` to accept/populate the new field in whatever way
disturbs its existing call sites least (a variadic or a sibling helper is preferable to editing every
existing call). Add a fake `DisplayServicer` alongside the existing `fake*Service` types.

`api/httpapi/display_test.go` covers:

- `200` with a rendered `display_name` (fake service returns `("Ada Lovelace", true, nil)`).
- `200` with a JSON-`null` `display_name` when the fake returns `("", false, nil)` — assert on the
  decoded JSON that the key is **present and null**, not absent.
- `200` with null when `Deps.Display` is nil (built via the unchanged two-argument `NewDeps`).
- `401` when the request carries no actor.
- `400` for a non-UUID path segment.
- `500` when the fake service returns an unmapped error, with the body carrying
  `error.code == "internal_error"` and no raw error text.
- **End-to-end**: following `api/httpapi/masked_lookup_test.go`'s precedent of wiring real
  collaborators rather than fakes, build a real registry via `service.NewDisplayRegistry(stubQuerier)`,
  a real `service.NewDisplayService(reg, entity.NewResolver())`, a real router via
  `NewDepsWithDisplay`, and assert the rendered name for `natural_person`, `corporation`, and
  `service_account`, plus the null body for an entity whose `FundamentalTypeSlug` is a type with no
  registered renderer. Reuse or extend that file's `stubQuerier` rather than writing a third one.

## Validation

- `cd api && go build ./...` succeeds.
- `cd api && make test` passes, including every new test above.
- `cd api && make lint` passes.
- `grep -n "func NewDeps(" api/httpapi/router.go` shows the original two-argument signature
  unchanged, and all pre-existing `NewDeps(` call sites (including in peer test files) still compile.
- The route responds correctly through the real router: the end-to-end test exercises
  `/entities/{uuid}/display-name` via `httptest` against `NewRouter(...)`, not the handler function
  directly.
- Existing `/entities/*` behaviour is unregressed: the pre-existing `api/httpapi` tests
  (`handlers_test.go`, `handlers_extra_test.go`, `masked_lookup_test.go`, `apps_test.go`) all still
  pass unmodified except for any mechanical `buildTestDeps` signature adjustment.
- No response body anywhere in the new code contains an internal entity ID.

## Metadata

architectural_impact: true

## References

- [Display HTTP surface design](../notes/display-http-surface-design.md) — endpoint placement
  rationale, response shape, error mapping, authorization rule.
- `api/httpapi/entities.go` — `getEntity`, the handler shape to mirror.
- `api/httpapi/router.go` — `Deps`, `NewDeps`, `NewRouter`'s `/entities` block.
- `api/httpapi/masked_lookup_test.go` — the real-collaborators httpapi test precedent and its
  `stubQuerier`.
- `api/apiresp/writer.go` and `errors.go` — `WriteJSON`, `WriteError`, the closed sentinel set.
- `AGENTS.md` "Conventions" — thin handlers, no internal IDs over HTTP.
- Task `001-add-display-service.md` — the `DisplayServicer` interface this task consumes.

## Checkpoint hints

- After `Deps`/`NewDepsWithDisplay` and the route compile.
- After the handler is implemented.
- After the fake-service handler tests pass.
- After the end-to-end real-registry test passes.
