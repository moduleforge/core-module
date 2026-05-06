# Authorization design

## 1. Purpose

This document describes how authorization decisions are integrated into service-layer operations across all peer modules. Authorization is the pre-operation gate: every read, list, search, create, update, and delete consults a single per-application policy before doing any work.

For why this design (rather than middleware, decorator chains, or a hook bus) was chosen, see [`cross-cutting-design-rationale.md`](cross-cutting-design-rationale.md). For the post-operation half of the lifecycle — observers, transactions, and post-commit dispatch — see [`state-management-design.md`](state-management-design.md).

---

## 2. The `Authorizer` interface

Defined in `core-module/api/authz`:

```go
type Authorizer interface {
    Authorize(ctx context.Context, action string, target Entity) error
}
```

- **One `Authorizer` per app.** No fan-out. The application's composition root constructs a single instance and injects it into every peer-module service.
- **Called pre-op on all operations**, including reads. The first line of every service method.
- **A non-nil return aborts immediately.** Callers return the error as-is; service methods do not wrap or suppress it. HTTP handlers map known sentinel errors (`ErrUnauthenticated`, `ErrForbidden`) to 401/403 status codes.
- **The actor is resolved from `ctx`**, not from a method parameter. See §3.
- **`action` and `target` are explicit method parameters** — they are method-specific and do not belong on the context. Putting them on `ctx` would create staleness bugs when one service method calls another.

---

## 3. Operation context (`opctx`)

The following values travel on `context.Context` through the service layer. They are set by HTTP middleware before the service is called.

| Key | Type | Set by | Meaning |
|-----|------|--------|---------|
| `actor_entity_id` | `int64` | auth middleware | Internal entity ID of the authenticated user. |
| `assumed_actor_entity_id` | `*int64` | auth middleware | Internal entity ID of the user whose identity an admin has assumed. Nil when no assumption is active. |
| `request_id` | `string` | HTTP middleware | Request correlation ID for logging and tracing. |

`opctx` lives in `core-module/api/opctx`. It is deliberately narrow: only ambient request properties go here. Richer policy data (roles, scopes, tenancy, app context) is the `Authorizer` implementation's concern, not `opctx`'s — the implementation can resolve those from the actor's entity ID via its own DB lookups.

When an admin assumes another user's identity, the authorizer reads the **assumed** actor for policy purposes (the admin is acting *as* that user), and the original admin ID for audit purposes. Both are available on `ctx` simultaneously.

---

## 4. Entity contract

The `Authorizer.Authorize` call takes an `Entity`. The interface is small:

```go
type Entity interface {
    // Resource returns the stable resource name for this entity type.
    // Examples: "natural_person", "tag", "contact".
    Resource() string

    // EntityID returns the internal entity ID, or nil for not-yet-persisted
    // entities (the create case where no DB-assigned ID exists yet).
    EntityID() *int64
}
```

`Resource()` is the canonical slug. It corresponds to the `fundamental_type_slug` used in the entity-typing system (see [`entity-typing.md`](entity-typing.md)). Authorizer policy code uses it to route decisions without inspecting concrete types or importing module-specific packages.

The same interface is used by observers (see [`state-management-design.md`](state-management-design.md)) so a single `target Entity` value carries through the full Authorize → Observe lifecycle.

For create operations where the target entity does not yet have an ID, pass a zero-valued Entity (e.g. `entity.NaturalPerson{}` with nil ID) — the resource string is the only thing the authorizer needs at that point.

---

## 5. Where the Authorize call goes

Every service method begins with a single Authorize call:

```go
func (s *FooService) Update(ctx context.Context, in FooUpdate) (Foo, error) {
    if err := s.authz.Authorize(ctx, "update", in); err != nil {
        return Foo{}, err
    }
    // … rest of the standard service-method shape (see state-management-design.md §5)
}
```

Read methods collapse to:

```go
func (s *FooService) Get(ctx context.Context, id int64) (Foo, error) {
    if err := s.authz.Authorize(ctx, "read", entity.Foo{ID: &id}); err != nil {
        return Foo{}, err
    }
    return s.repo.Get(ctx, id)
}
```

Action strings are stable verbs: `"read"`, `"list"`, `"search"`, `"create"`, `"update"`, `"delete"`, plus domain-specific verbs for special operations (`"assume"`, `"login"`, `"grant"`, `"revoke"`).

Authorize is called from the **service layer**, not from HTTP handlers. Handlers must not duplicate the call — the service is the authoritative gate. This keeps the contract uniform regardless of how a service is invoked (HTTP today; potentially gRPC, message-queue handlers, or scheduled jobs in future).

---

## 6. What the framework does not authorize

- **HTTP-layer concerns** — rate limiting, quotas, idempotency keys. These run at the HTTP boundary; the service should never see a request a rate-limiter would reject.
- **Validation** — input shape, format, and field-level constraints stay inline in service methods. Authorization is policy ("may this actor do this?"), not data correctness.
- **Multi-tenancy** — folds into the `Authorizer` implementation. A multi-tenant authorizer reads tenant context already on `ctx` and applies tenant-scoped rules; no separate framework piece is needed.
- **Feature flags** — site-specific. Individual services may consult feature flags inline if a behaviour is gated, independent of the authz decision.

---

## 7. Performance

- One virtual call per service method (`Authorize`). No reflection, no marshalling, no allocations beyond what the implementation does internally.
- Authorize implementations are expected to perform at most one DB lookup per call (typically: fetch `is_admin` for the actor's entity ID). Implementations may cache.
- Authorize denial returns immediately; no transaction is opened, no observer fires.
