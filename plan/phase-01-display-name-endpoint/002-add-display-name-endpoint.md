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
   and return — mirroring `getEntity`'s first lines. This is the same pre-existing handler-layer
   convention every other entity handler follows, kept for consistency; it is a cheap short-circuit,
   **not** the authorization gate. The real gate is the service's
   `az.Authorize(ctx, "read", &internalID)` call (task 001), which also produces the 401 on its own
   when no effective actor is present.
2. Parse `chi.URLParam(r, "uuid")` with `uuid.Parse`; on error
   `apiresp.WriteError(w, r, apiresp.ErrInvalidInput)` (400) and return.
3. If `h.d.Display` is nil, write the unavailable body (below) with `200` and return — no panic, no
   500. A deployment with no display service wired is a valid state, and that response is constant
   across every UUID, so it discloses nothing about any entity.
4. Otherwise call
   `h.d.Display.RenderField(r.Context(), h.d.Services.Querier(), entityUUID, display.FieldName)`.
   On a non-nil error, `apiresp.WriteError(w, r, err)` and return — pass the service's error through
   untouched. It is already an `apiresp`-classifiable sentinel in the cases that matter (`403` for
   a masked miss or an authorization denial, `401` for no effective actor); do **not** inspect,
   translate, or downgrade it into a `200`/`null` body.
5. Write `200` via `apiresp.WriteJSON` with the body below.

Response body — a small named struct with explicit JSON tags, not a bare map (`display_name` must
serialize as JSON `null`, not be omitted, when unavailable; use `*string`):

```json
{ "uuid": "<the requested uuid>", "display_name": "Ada Lovelace" }
```

```json
{ "uuid": "<the requested uuid>", "display_name": null }
```

The unavailable body is returned for exactly two cases: no renderer is registered for the (resolved,
readable) entity's type, and no display service/registry is wired into this deployment. A UUID
naming no entity is **not** one of them — that is a `403`, produced by the service's resolve step
and indistinguishable from an authorization denial. Do **not** add `kind`, `fundamental_type_slug`,
or any other field: this endpoint answers one question, and a caller authorized to read the entity
can already get its type from `GET /entities/{uuid}`; omitting it also keeps the body shape
identical in both cases. Never include an internal entity ID (`AGENTS.md` Conventions).

Carry a doc comment on the handler stating that the endpoint requires real `"read"` authorization on
the target entity — the same gate as every other single-entity read, with the same masked `403` for
a nonexistent or unauthorized UUID — and that the `200`/`null` shape is the deliberate
graceful-fallback contract for a readable entity with no registered renderer, not a missing error
case.

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
- `403` when the fake service returns an error satisfying `errors.Is(err, apiresp.ErrForbidden)` —
  the handler passes it through `apiresp.WriteError` untouched, exactly like the `500` case, just
  with a different sentinel. This proves the handler never downgrades a real authorization/masked-miss
  error into the `200`/null shape.
- **End-to-end**: following `api/httpapi/masked_lookup_test.go`'s precedent of wiring real
  collaborators rather than fakes, build a real registry via `service.NewDisplayRegistry(stubQuerier)`,
  a real `service.NewDisplayService(reg, &stubAuthorizer{}, entity.NewResolver())` — reusing that
  file's package-local, always-allow `stubAuthorizer` — and a real router via `NewDepsWithDisplay`,
  then assert:
  - the rendered name for `natural_person`, `corporation`, and `service_account`;
  - the null body for an entity whose `FundamentalTypeSlug` is a type with no registered renderer
    (still using `stubAuthorizer`, so the request reaches the renderer-not-found path rather than
    being stopped by authorization);
  - **and**, mirroring `TestGetEntity_MaskedMiss_Returns403Forbidden` exactly, a `403` for a random/
    unseeded UUID against the same real chain — proving the resolver's masked-403 propagates through
    this endpoint too, not just through `getEntity`.

  Reuse or extend `masked_lookup_test.go`'s `stubQuerier` rather than writing a third one.

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

## Status

**Outcome:** succeeded — 2026-08-24.

Implemented exactly the scope described above, entirely within `api/httpapi`:

- `api/httpapi/router.go` — added `Deps.Display service.DisplayServicer` (nil-safe), left `NewDeps`
  byte-for-byte unchanged, added the additive `NewDepsWithDisplay(svcs, logger, dsp) Deps`
  constructor, and registered `r.Get("/{uuid}/display-name", h.getDisplayName)` inside the existing
  `/entities` route group next to `r.Get("/{uuid}", ...)`, with the load-bearing "why not a second
  `Register*Routes` entry" rationale captured in a code comment.
- `api/httpapi/display.go` (new) — `getDisplayName` handler mirroring `getEntity`'s thin-handler
  shape: 401 pre-flight actor check, UUID parse (400 on failure), nil-`Display` graceful-fallback
  (200/null), `RenderField` call with untouched error passthrough, `200` via `apiresp.WriteJSON` with
  a named `displayNameResponse{UUID uuid.UUID; DisplayName *string}` struct (never a bare map) so
  `display_name` serializes as JSON `null`, not omitted, when unavailable.
- `api/httpapi/mock_test.go` — added `fakeDisplayService` (implements `service.DisplayServicer`) and
  a sibling helper `buildTestDepsWithDisplay(entity, np, corp, sa, dsp) Deps` that wraps the
  pre-existing `buildTestDeps` and sets `.Display`; none of the ~34 pre-existing `buildTestDeps(...)`
  call sites needed to change.
- `api/httpapi/masked_lookup_test.go` — extended `stubQuerier` with optional seed maps
  (`entitiesByUUID`, `entitiesByID`, `naturalPersonsByEntityID`, `corporationsByEntityID`,
  `serviceAccountsByEntityID`); nil maps (the zero value every pre-existing test uses) preserve the
  original always-zero-row/nil-error behaviour byte-for-byte, so `TestGetEntity_MaskedMiss_Returns403Forbidden`
  is unaffected. A populated map now reports `pgx.ErrNoRows` for any key absent from it, which the
  new end-to-end test's masked-403 sub-test relies on.
- `api/httpapi/display_test.go` (new) — all tests listed in `## Requirements` §4, including the
  end-to-end sub-tests (real `service.NewDisplayRegistry`, real `service.NewDisplayService`, real
  `NewDepsWithDisplay`/`NewRouter`) for `natural_person`/`corporation`/`service_account`, the
  no-registered-renderer null case, and the masked-403 case for an unseeded UUID.

**Validation:** `cd api && go build ./...`, `cd api && make test` (all packages, including the full
new `display_test.go` suite), and `cd api && make lint` (`go vet` + `gofmt -l`) all pass. Confirmed
`grep -n "func NewDeps(" api/httpapi/router.go` still shows the original two-argument signature, and
`handlers_test.go`, `handlers_extra_test.go`, `apps_test.go` are untouched (only `masked_lookup_test.go`
and `mock_test.go` were extended, both explicitly anticipated by `## Requirements` §4, with no
`buildTestDeps` signature change). No response body in the new code carries an internal entity ID.

**Assumptions relied on:** none beyond what `## Requirements` and the referenced design note already
state; no `## Assumptions` section was present on this task doc.

Affected files (repo-relative to `api/`, all under `httpapi/` except `router.go`'s parent):
`api/httpapi/router.go`, `api/httpapi/display.go`, `api/httpapi/display_test.go`,
`api/httpapi/mock_test.go`, `api/httpapi/masked_lookup_test.go`.
