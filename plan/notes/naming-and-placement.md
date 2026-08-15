# Naming and placement research

## Purpose and scope

Records the evidence gathered from the existing `mod-core/api` source and from
`docs/mf-standards/architecture/authorization-design.md` that fixes the name,
signature, package, and doc-comment shape of the two new exported primitives.
Written during planning so the implementing task agents do not have to re-derive
any of it.

## Existing state

### `api/opctx/opctx.go`

Package doc opens with "Three values are defined:" and enumerates
`ActorEntityID`, `SudoActorEntityID`, and `RequestID`. Each accessor is a
free function with a two-to-three line doc comment stating what it returns and
what the zero/absent case is:

```go
// ActorEntityID returns the authenticated user's entity ID from ctx.
// Returns 0, false if not set.
func ActorEntityID(ctx context.Context) (int64, bool)

// SudoActorEntityID returns the entity ID being impersonated by an admin,
// or 0, false if no impersonation is active.
func SudoActorEntityID(ctx context.Context) (int64, bool)
```

The package imports only `context`. It has no dependency on any other
`core-api` package.

### `api/authz/authz.go`

Declares two interfaces (`Authorizer`, `OpResolver`) and nothing else. The
package doc says "Authorizer is intentionally narrow: one method" and "The
implementation is consumer-supplied; this package defines only the contract."
It imports only `context`.

### `api/apiresp/errors.go`

`ErrUnauthenticated` is declared at line 21 as part of the canonical sentinel
block. `apiresp` already imports `opctx` (in `writer.go`) and does **not**
import `authz`, so an `authz` → `apiresp` import edge introduces no cycle.
Verified by grepping every `core-api/authz` importer: `httpapi/`, `service/`,
`authz/setup/`, and test files — never `apiresp/`.

### The duplicated policy in mod-users

`mod-users/api/internal/authz/authz.go:188-196`:

```go
// effectiveActor returns the entity ID that should be used for policy checks.
// If a sudo actor is set (admin assuming another user's identity), that
// entity ID is returned, since the admin is acting as the sudo user.
func effectiveActor(ctx context.Context) (int64, bool) {
	if id, ok := opctx.SudoActorEntityID(ctx); ok {
		return id, true
	}
	return opctx.ActorEntityID(ctx)
}
```

This is the exact behavior the new `opctx.EffectiveActorEntityID` must
reproduce, including the `(int64, bool)` return shape. mod-users' own
delegation to it is a sibling phase in this federated plan, in mod-users' own
plan worktree, and is out of scope here.

## What `authorization-design.md` fixes

`docs/mf-standards/architecture/authorization-design.md` is canonical for this
interface family. The load-bearing statements for this plan:

- The `Authorizer` interface is stated verbatim as the single-method contract
  (lines 13-17). Anything added here must not touch it.
- "HTTP handlers map known sentinel errors (`ErrUnauthenticated`,
  `ErrForbidden`) to 401/403 status codes." The new check must therefore return
  the existing `apiresp.ErrUnauthenticated`, never a new sentinel.
- The Operation context section states the sudo-first rule in prose: "When an
  admin assumes another user's identity, the authorizer reads the **assumed**
  actor for policy purposes ... Both are available on `ctx` simultaneously."
  `EffectiveActorEntityID` is the executable form of exactly this sentence,
  which is why `opctx` — not `authz` — is its home.
- "`opctx` ... is deliberately narrow: only ambient request properties go here."
  A derived accessor over two ambient properties stays inside that boundary; it
  adds no new context key and no new stored value.

## Decisions

| Decision | Value | Rationale |
|---|---|---|
| opctx symbol | `EffectiveActorEntityID(ctx context.Context) (int64, bool)` | Mirrors the `(int64, bool)` shape of its two siblings exactly, so call sites read identically. |
| authz symbol | `RequireAuthenticated(ctx context.Context) error` | Reads unambiguously as an authentication-only check. The originally suggested `RequireAuthorization()` is rejected: it names the wrong security question and would be read as a grant check. |
| authz symbol form | Free function, not an interface method | Adding a method would break every `Authorizer` implementation in the workspace (mod-users, mod-repos, app-mfmanager, plus test stubs across roughly 8 modules). A free function needs no implementer changes. |
| Failure value | `apiresp.ErrUnauthenticated`, returned bare | The design doc's 401 mapping already keys on this sentinel via `errors.Is`; returning it unwrapped keeps the existing `WriteError` mapping working with no change. |

## Alternatives considered and rejected

- **`authz.RequireAuthentication`** — grammatically parallel to
  `RequireAuthorization` but a noun phrase; `RequireAuthenticated` reads as the
  assertion being made about the actor and is the form the request proposed.
- **Putting the check in `apiresp`** — rejected: `apiresp` is the wire/response
  contract, not a policy package, and the check is a policy question.
- **Putting `EffectiveActorEntityID` in `authz`** — rejected: it is a pure
  context accessor with no policy dependency, and `authz` importing `opctx`
  just to re-export a context read would invert the layering the design doc
  describes.
