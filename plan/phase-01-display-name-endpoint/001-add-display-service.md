# Add Display Service

## Purpose and scope

Add the service-layer half of the display-name HTTP surface: a constructor that builds a
`display.Registry` with mod-core's builtin renderers already registered, and a service type whose
one method turns a public entity UUID into a rendered display string — authenticating first,
resolving the UUID, dispatching through the registry, and reporting "no renderer for this entity's
type" as a normal, non-error outcome rather than an error.

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
func NewDisplayService(reg *display.Registry, entityResolver *entity.Resolver) *DisplayService
```

`RenderField` behaviour, in order:

1. **Authenticate first.** Call `authz.RequireAuthenticated(ctx)`; on error return
   `("", false, err)`. This is the *only* authorization gate — deliberately **not**
   `az.Authorize(ctx, "read", &id)`. Carry a code comment explaining why, in the terms of
   [the design note](../notes/display-http-surface-design.md#authorization-decision): the caller is
   resolving an entity UUID it already holds as a reference in another module's data and generally
   cannot read that entity's profile; `authz.RequireAuthenticated` was added to `api/authz/authz.go`
   for exactly this authentication-only case and this is its first production call site. Keep the
   gate to a single, clearly-marked statement so tightening it later is a one-line change.
2. **Nil-registry tolerance.** If the receiver's registry is `nil`, return `("", false, nil)`. A
   deployment that wires no registry is a valid state, not a failure.
3. **Resolve the UUID.** Call `entityResolver.Resolve(ctx, q, entityUUID, "entity")`. On
   `apiresp.ErrForbidden` or `apiresp.ErrNotFound` (checked with `errors.Is`) return
   `("", false, nil)` — an unknown UUID is reported as unavailable, not as an error, so the endpoint
   cannot be used as an existence oracle. Any other error is returned as-is (wrapped with context).
4. **Render.** Call `reg.Render(ctx, nil, internalID, fieldName)` — `nil` transaction; the registry
   and every builtin renderer already handle a nil tx by falling back to the base querier. When the
   error wraps `display.ErrRendererNotRegistered` (check with `errors.Is`) return `("", false, nil)`.
   Any other error is returned wrapped (`fmt.Errorf("display.RenderField: %w", err)`).
5. On success return `(value, true, nil)`. An empty-but-successful render (e.g. a
   `service_account` whose label is empty) returns `("", true, nil)` — `available` reflects whether a
   renderer ran, not whether the string is non-empty.

`RenderField` takes `fieldName` rather than hard-coding `display.FieldName` so a future
`/description` route needs no service change. It must not import anything from `api/httpapi`.

### 3. Tests

`api/service/display_test.go`, in the existing `package service` test convention for this directory
(check `api/service/entity_test.go` and reuse `api/service/mock_test.go`'s existing querier/resolver
fakes rather than adding parallel ones where they fit):

- Resolved value for a `natural_person` through a real `NewDisplayRegistry` over a fake `Querier` —
  asserts `("Ada Lovelace", true, nil)`-shaped output and proves `RegisterBuiltins` really ran.
- The same for `corporation` and `service_account`, so all three builtins are covered here at the
  service layer.
- Not-registered: an entity whose `FundamentalTypeSlug` is a type no renderer is wired for (e.g.
  `"user_account"`) → `("", false, nil)`, **and** assert the returned error is nil rather than merely
  non-fatal.
- Unknown UUID (resolver returns `pgx.ErrNoRows` from the fake querier) → `("", false, nil)`.
- Unauthenticated context (no actor, no sudo actor) → error satisfying
  `errors.Is(err, apiresp.ErrUnauthenticated)`, and `available` false.
- Nil registry → `("", false, nil)` with no panic.
- Genuine render failure (a renderer registered on a hand-built registry that returns an error) →
  non-nil error that does **not** satisfy `errors.Is(err, display.ErrRendererNotRegistered)`.

## Validation

- `cd api && go build ./...` succeeds.
- `cd api && make test` passes (`go test ./...`), including the new `api/service/display_test.go`.
- `cd api && make lint` passes (`go vet ./...` plus the gofmt check).
- `api/service/display_builtins.go` is unmodified: `git diff --stat` names only
  `api/service/display.go` and `api/service/display_test.go`.
- `service.New`'s signature is unchanged — grep confirms no edit to `api/service/service.go`.
- `grep -rn "httpapi" api/service/display.go` returns nothing (no layering inversion).
- The authentication-only gate is a single statement carrying the explanatory comment required
  above; confirm by reading the finished method.

## Metadata

architectural_impact: true

## References

- [Display HTTP surface design](../notes/display-http-surface-design.md) — response semantics, the
  closed error-code set, and the full authorization rationale this task implements.
- `api/display/registry.go` — `Registry`, `FieldRenderer`, `FieldName`, `ErrRendererNotRegistered`.
- `api/service/display_builtins.go` — `RegisterBuiltins`, unchanged by this task.
- `api/service/entity.go` — the interface + struct + compile-time-assertion shape to follow.
- `api/entity/resolver.go` — `Resolve` and its `apiresp`-aliased sentinels.
- `api/authz/authz.go` — `RequireAuthenticated` and its stated purpose.
- `AGENTS.md` "Conventions" — authorization checked first in every service method; internal IDs never
  leave the service layer.

## Checkpoint hints

- After `NewDisplayRegistry` plus the `DisplayServicer`/`DisplayService` skeleton compiles.
- After `RenderField`'s full behaviour is implemented.
- After the test file passes.
