# Add Display Service

## Purpose and scope

Add the service-layer half of the display-name HTTP surface: a constructor that builds a
`display.Registry` with mod-core's builtin renderers already registered, and a service type whose
one method turns a public entity UUID into a rendered display string — resolving the UUID,
authorizing `"read"` on the resulting entity exactly as every other single-entity read in mod-core
does, dispatching through the registry, and reporting "no renderer for this entity's type" as a
normal, non-error outcome rather than an error.

Scope is `api/` only: one new source file (`api/service/display.go`) and one new test file
(`api/service/display_test.go`). No HTTP wiring, no manifest change, no documentation beyond Go doc
comments — later tasks in this phase own those. No standard skill covers this; follow the procedure
below.

## Requirements

### 1. `NewDisplayRegistry`

Add to `api/service/display.go`:

```go
func NewDisplayRegistry(q coredb.Querier) *display.Registry
```

It constructs `display.NewRegistry(q)`, calls the existing `RegisterBuiltins(reg, q)` on it, and
returns it. This is the single call an mfgen-composed app makes to obtain a registry that already
knows mod-core's three concrete types; it is deliberately a manifest-declarable constructor with a
single arg-source (`queries:coredb`).

Do **not** modify `api/service/display_builtins.go`'s `RegisterBuiltins` — it stays exactly as it is
so a caller that already has its own registry can keep calling it directly (mod-users' own server
does).

### 2. `DisplayServicer` / `DisplayService`

An interface plus its implementation, following the shape of `EntityServicer`/`EntityService` in
`api/service/entity.go` (interface, struct, compile-time `var _ DisplayServicer = (*DisplayService)(nil)`
assertion):

```go
type DisplayServicer interface {
    RenderField(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID, fieldName string) (string, bool, error)
}
```

The `bool` is `available` — `true` when a renderer produced a value, `false` when the value could not
be produced for an expected, non-error reason. Constructor:

```go
func NewDisplayService(reg *display.Registry, az authz.Authorizer, entityResolver *entity.Resolver) *DisplayService
```

`az` is stored as a struct field named `az`, mirroring `EntityService`'s `az authz.Authorizer`
field. The parameter order puts the authorizer ahead of the resolver, matching `service.New`'s own
ordering (`... service:authorizer ... service:entityResolver ...`), so task 003's manifest args read
`service:displayRegistry`, `service:authorizer`, `service:entityResolver`.

`RenderField` behaviour, in order:

1. **Resolve the UUID.** Call `entityResolver.Resolve(ctx, q, entityUUID, "entity")`; on error
   return `("", false, err)` — **propagated verbatim, not wrapped, not special-cased, not collapsed
   into an "unavailable" result.** The resolver's default policy already masks a missing UUID as
   `apiresp.ErrForbidden`, and `api/entity/resolver.go`'s sentinels are aliased directly onto the
   `apiresp` sentinels, so `apiresp.WriteError` classifies it correctly with zero translation code
   at this call site. That is the settled precedent — see
   [`plan-summary-masked-lookup-403.md`](../plan-summary-masked-lookup-403.md), "Alias fix at the
   source, not a translation shim".
2. **Authorize the read.** Call `s.az.Authorize(ctx, "read", &internalID)`; on error return
   `("", false, err)` — again propagated as-is. This is the real authorization gate: this endpoint
   requires the same `"read"` authorization on the target entity as every other single-entity read
   in mod-core, and holding a UUID entitles a caller to nothing.

   Steps 1 and 2 together must be byte-for-byte the shape `EntityService.GetByUUID` and
   `EntityService.ResolveProfile` already use in `api/service/entity.go` — copy that pattern rather
   than inventing anything. Note there is deliberately **no** separate `authz.RequireAuthenticated`
   call: `Authorizer` implementations are contractually required to return
   `apiresp.ErrUnauthenticated` themselves when no effective actor is present (`api/authz/authz.go`,
   `Authorizer` doc comment), which is exactly why `GetByUUID` needs no such call either. 401-vs-403
   falls out of `Authorize` alone. `authz.RequireAuthenticated` remains an available but unused
   helper in `api/authz/authz.go` for some future genuinely authentication-only case — do not use
   it here, and do not remove it.
3. **Nil-registry tolerance.** *After* both checks above have passed: if the receiver's registry is
   `nil`, return `("", false, nil)`. A deployment that wires no registry is a valid state, not a
   failure. The nil check comes after authorization, not before, so the "unavailable" outcome is
   never reachable without a successful read authorization.
4. **Render.** Call `reg.Render(ctx, nil, internalID, fieldName)` — `nil` transaction; the registry
   and every builtin renderer already handle a nil tx by falling back to the base querier. When the
   error wraps `display.ErrRendererNotRegistered` (check with `errors.Is`) return `("", false, nil)`.
   Any other error is returned wrapped (`fmt.Errorf("display.RenderField: %w", err)`).
5. On success return `(value, true, nil)`. An empty-but-successful render (e.g. a
   `service_account` whose label is empty) returns `("", true, nil)` — `available` reflects whether a
   renderer ran, not whether the string is non-empty.

So `available == false` with a `nil` error now means exactly one thing: **the entity exists and the
caller may read it, but nothing can render it** (no renderer for its type, or no registry wired).
It no longer means "unknown or unauthorized entity" — that is a real, propagated error.

`RenderField` takes `fieldName` rather than hard-coding `display.FieldName` so a future
`/description` route needs no service change. It must not import anything from `api/httpapi`.

### 3. Tests

`api/service/display_test.go`, in the existing `package service` test convention for this directory
(check `api/service/entity_test.go` and reuse `api/service/mock_test.go`'s existing querier/resolver
fakes — `allowAllAuthz{}`, `denyAllAuthz{err: ...}`, `testEntityResolver()`, `mockQuerier` — rather
than adding parallel ones where they fit):

- Resolved value for a `natural_person` through a real `NewDisplayRegistry` over a fake `Querier`,
  with `allowAllAuthz{}` — asserts `("Ada Lovelace", true, nil)`-shaped output and proves
  `RegisterBuiltins` really ran.
- The same for `corporation` and `service_account`, so all three builtins are covered here at the
  service layer.
- Not-registered: an entity whose `FundamentalTypeSlug` is a type no renderer is wired for (e.g.
  `"user_account"`) → `("", false, nil)`, **and** assert the returned error is nil rather than merely
  non-fatal.
- **Masked miss propagates.** An unseeded/random UUID (resolver hits `pgx.ErrNoRows` from the fake
  querier) → a non-nil error, asserted the same way `TestEntityService_GetByUUID_NotFound` asserts
  it in `api/service/entity_test.go`: `errors.Is(err, entity.ErrForbidden)` **and**
  `errors.Is(err, ErrForbidden)` (so the error is `apiresp`-classifiable as 403). It must **not**
  return `("", false, nil)` — an unknown UUID is an error now, not an "unavailable" result.
- **Authorization denial propagates.** A service built with `denyAllAuthz{err: authzErr}` over a
  seeded, resolvable entity → `errors.Is(err, authzErr)`, mirroring
  `TestEntityService_GetByUUID_AuthzDenied`. Assert `available` is false and the returned string is
  empty, but the error — not the `available` flag — is what carries the refusal.
- Nil registry (with an allow-all authorizer and a resolvable entity) → `("", false, nil)` with no
  panic.
- Genuine render failure (a renderer registered on a hand-built registry that returns an error) →
  non-nil error that does **not** satisfy `errors.Is(err, display.ErrRendererNotRegistered)`.

There is deliberately no "unauthenticated context" case at this layer: the 401 comes from the
injected `Authorizer` implementation, which is a test fake here, so unauthenticated behaviour is
covered by the authorization-denial case above plus the handler-layer 401 test in task 002.

## Validation

- `cd api && go build ./...` succeeds.
- `cd api && make test` passes (`go test ./...`), including the new `api/service/display_test.go`.
- `cd api && make lint` passes (`go vet ./...` plus the gofmt check).
- `api/service/display_builtins.go` is unmodified: `git diff --stat` names only
  `api/service/display.go` and `api/service/display_test.go`.
- `service.New`'s signature is unchanged — grep confirms no edit to `api/service/service.go`.
- `grep -rn "httpapi" api/service/display.go` returns nothing (no layering inversion).
- `grep -n "RequireAuthenticated" api/service/display.go` returns nothing — this service does not
  use it; `api/authz/authz.go` still defines it, unchanged.
- `RenderField`'s first two statements are the resolve + `s.az.Authorize(ctx, "read", &internalID)`
  pair, each returning the error unmodified; confirm by reading the finished method side by side
  with `EntityService.GetByUUID` in `api/service/entity.go`. Neither error is wrapped, translated,
  or converted into an `available == false` result.

## Metadata

architectural_impact: true

## References

- [Display HTTP surface design](../notes/display-http-surface-design.md) — response semantics, the
  closed error-code set, and the full authorization rationale this task implements.
- `api/display/registry.go` — `Registry`, `FieldRenderer`, `FieldName`, `ErrRendererNotRegistered`.
- `api/service/display_builtins.go` — `RegisterBuiltins`, unchanged by this task.
- `api/service/entity.go` — the interface + struct + compile-time-assertion shape to follow, and
  `GetByUUID` / `ResolveProfile`: the exact resolve-then-`Authorize("read")` sequence to copy.
- `api/service/entity_test.go` — `TestEntityService_GetByUUID_NotFound` and
  `TestEntityService_GetByUUID_AuthzDenied`: the assertion style the new propagation tests mirror.
- `api/entity/resolver.go` — `Resolve` and its `apiresp`-aliased sentinels.
- [`plan-summary-masked-lookup-403.md`](../plan-summary-masked-lookup-403.md) — why resolver errors
  are propagated as-is rather than translated at the call site.
- `api/authz/authz.go` — the `Authorizer` interface contract (implementations return a
  distinguishable `apiresp.ErrUnauthenticated` for "no actor", which is why no separate
  authentication call is needed). `RequireAuthenticated` lives here too but is **not** used by this
  task.
- `AGENTS.md` "Conventions" — authorization checked first in every service method; internal IDs never
  leave the service layer.

## Checkpoint hints

- After `NewDisplayRegistry` plus the `DisplayServicer`/`DisplayService` skeleton compiles.
- After `RenderField`'s full behaviour is implemented.
- After the test file passes.
