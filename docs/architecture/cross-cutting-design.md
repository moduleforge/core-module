# Cross-cutting concerns framework

## 1. Purpose

This document describes how cross-cutting concerns — authorization, audit logging, cache invalidation, outbox/notification delivery, and similar behaviours — are integrated into service-layer operations across all peer modules. The design is deliberately small: two interfaces, injected via constructors, called explicitly at known extension points. For why this approach was chosen over alternatives, see the sibling rationale doc (`cross-cutting-design-rationale.md`).

---

## 2. Two interfaces

Both interfaces are defined in `core-module`.

### `Authorizer`

```go
type Authorizer interface {
    Authorize(ctx context.Context, action string, target Entity) error
}
```

- One `Authorizer` per app. No fan-out.
- Called **pre-op** on **all** operations, including reads.
- A non-nil return error aborts the operation immediately. The caller returns the error as-is; service methods do not wrap or suppress it.
- The authorizer resolves the acting user from `ctx` (see §3). `action` and `target` are explicit parameters because they are method-specific and do not belong on the context.

### `MutationObserver`

```go
type MutationObserver interface {
    // Observe runs inside the operation's transaction. The error return is
    // passed to the caller (typically an ObserverGroup), which decides
    // whether to propagate or log-and-swallow based on its configured
    // Policy and the call variant in use (Observe / MustObserve / MayObserve).
    // See §6.
    Observe(ctx context.Context, tx pgx.Tx, op, resource string, targetEntityID *int64, before, after any) error

    // ObserveAfterCommit runs after the transaction successfully commits.
    // A non-nil error is logged; it does not unwind the committed operation.
    ObserveAfterCommit(ctx context.Context, op, resource string, targetEntityID *int64, before, after any) error
}
```

- Used for mutations only (creates, updates, deletes). Reads do not call observers.
- An implementation may be interested in only one of the two observation points. The other method should be a no-op returning nil.
- `op` is a short verb string: `"create"`, `"update"`, `"delete"`.
- `resource` is the stable resource name from `Entity.Resource()` (see §4): e.g. `"natural_person"`, `"tag"`, `"contact"`.
- `before` and `after` are JSON-serialisable snapshots; either may be nil (e.g. `before` is nil on create, `after` is nil on delete).

---

## 3. Operation context (`OpContext`)

The following values travel on `context.Context` through the service layer. They are set by HTTP middleware before the service is called.

| Key | Type | Set by | Meaning |
|-----|------|--------|---------|
| `actor_entity_id` | `int64` | auth middleware | Internal entity ID of the authenticated user. |
| `assumed_actor_entity_id` | `*int64` | auth middleware | Internal entity ID of the user whose identity an admin has assumed. Nil when no assumption is active. |
| `request_id` | `string` | HTTP middleware | Request correlation ID for logging and tracing. |

**Action and target are not context values.** They are explicit method parameters on `Authorize` and both `Observe` methods. Context values survive call boundaries poorly when they are method-specific and have different values per call.

---

## 4. Entity contract

The `Entity` interface gains two methods. The concrete shape is finalised in Phase 2; the contract is:

```go
type Entity interface {
    // Resource returns the stable resource name for this entity type.
    // Examples: "natural_person", "tag", "contact".
    // Used by observers and the authorizer to identify entity types
    // without type-asserting.
    Resource() string

    // EntityID returns the internal entity ID.
    // Nil only in narrow pre-create cases where the entity has not yet
    // been persisted and therefore has no DB-assigned ID.
    EntityID() *int64
}
```

`Resource()` is the canonical slug. It corresponds to the `fundamental_type_slug` used in the entity-typing system (see `entity-typing.md`). Observers and the authorizer use it to route behaviour without inspecting concrete types.

---

## 5. Standard service-method shape

The canonical pattern for a mutating service method. Paste and adapt; do not invent variations.

```go
func (s *FooService) Update(ctx context.Context, in FooUpdate) (Foo, error) {
    // 1. Authorize — abort immediately on error.
    if err := s.authz.Authorize(ctx, "update", in); err != nil {
        return Foo{}, err
    }

    // 2. Mutate inside a transaction; observers participate in the same tx.
    var out Foo
    err := txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
        before, err := s.fetchByID(ctx, tx, in.ID)
        if err != nil { return err }

        out, err = s.repo.Update(ctx, tx, in)
        if err != nil { return err }

        // Observe inside tx; error here rolls back.
        return s.observers.Observe(ctx, tx, "update", "foo", &in.ID, before, out)
    })
    if err != nil { return Foo{}, err }

    // 3. Post-commit observers. Errors are logged, not propagated.
    s.observers.ObserveAfterCommit(ctx, "update", "foo", &out.ID, nil, out)
    return out, nil
}
```

**Step annotations:**

1. **Authorize** — single call, single interface, single error path. If denied, nothing else runs.
2. **Mutate + in-tx observe** — `repo.Update` and `Observe` share the same transaction. If either fails, the transaction rolls back and the caller sees the error.
   - `s.observers.Observe(...)` uses the app's configured default policy (see §6).
   - `s.observers.MustObserve(...)` is available as a per-call override: always propagates errors and aborts, regardless of the group's default.
   - `s.observers.MayObserve(...)` is available as a per-call override: always logs and swallows errors, regardless of the group's default.
3. **Post-commit observe** — fires only after the transaction commits successfully. `ObserveAfterCommit` has no `Must`/`May` variants: the transaction has already committed, so propagation is meaningless. Errors are always logged and never returned to the caller.

**Read-only methods** collapse to:

```go
func (s *FooService) Get(ctx context.Context, id int64) (Foo, error) {
    if err := s.authz.Authorize(ctx, "read", &fooStub{id: id}); err != nil {
        return Foo{}, err
    }
    return s.repo.Get(ctx, id)
}
```

No observers, no transaction helper (unless the read needs one for consistency).

---

## 6. Multi-observer composition

`core-module` provides a concrete `ObserverGroup` type that wraps N `MutationObserver` implementations and dispatches to all of them. Service methods receive `*ObserverGroup` (not the bare `MutationObserver` interface) so they can call the per-call policy variants described below.

```go
// Constructor — exact name to be finalised in Phase 2.
func NewObserverGroup(observers ...MutationObserver) *ObserverGroup

// Optional: set the default in-tx policy at construction time.
func (g *ObserverGroup) WithPolicy(p Policy) *ObserverGroup
```

### In-tx methods

`ObserverGroup` exposes three methods for in-transaction work:

| Method | Policy applied |
|--------|---------------|
| `Observe(...)` | Uses the group's configured default. Default-of-defaults is `PolicyPropagate`. |
| `MustObserve(...)` | Always propagates the first non-nil error, aborting the operation regardless of group config. |
| `MayObserve(...)` | Always logs each non-nil error per-observer and returns nil, regardless of group config. |

All three call every wrapped observer in parallel via `errgroup`. `Observe` and `MustObserve` return the first non-nil error; `MayObserve` logs all errors and returns nil.

### Post-commit method

`ObserveAfterCommit(...)` always logs errors and never propagates them. There are no `Must`/`May` variants because the transaction has already committed.

### Additional notes

**No inter-observer dependencies.** Observers run without ordering guarantees. This is a simplification, not a fundamental constraint; ordering or dependency resolution may be added if a concrete use case appears.

**No per-observer policy split within a group.** A future revision may support configuring individual observers differently (e.g. audit-must, cache-may). For now, one policy applies to the whole group at each call site.

**Zero-observer case.** `NewObserverGroup()` with no arguments returns a no-op group; service code does not need nil checks.

---

## 7. App-side composition

Cross-cutting behaviour is composed in the application's composition root. No module knows about any peer module.

```go
authz := authzimpl.New(...)

observers := core.NewObserverGroup(
    auditmod.New(...),       // writes audit_log rows
    outboxmod.New(...),      // writes outbox rows for async delivery
    cacheinval.New(...),     // invalidates cache entries post-commit
).WithPolicy(core.PolicySwallow)  // omit to keep the default PolicyPropagate

userSvc := usersservice.New(userDB, authz, observers)
tagSvc  := tagsservice.New(tagDB,  authz, observers)
```

Each module (`auditmod`, `outboxmod`, `cacheinval`) exports a single `MutationObserver`. The app decides which observers are active, in what combination, and what the default error policy is. Module packages remain isolated.

Service constructors accept `*ObserverGroup` (not the bare `MutationObserver` interface) so that individual service methods can call `MustObserve` or `MayObserve` at sites that need to deviate from the group's default.

---

## 8. What this design does not do

- **No service-layer middleware or decorator chain.** Extension points are explicit interface calls, not wrapping.
- **No event bus, no event registry, no reflection.**
- **No interceptor pattern.** The framework does not intercept method calls; service methods call the interfaces directly at known points.
- **Tracing** is handled by `ctx`-propagated OTel libraries; no service-layer interface is needed.
- **Rate limiting, quota, and idempotency** are HTTP-layer concerns and are not part of this framework.

---

## 9. Performance properties

| Operation | Cost |
|-----------|------|
| `Authorize` | One virtual call per service method. |
| `Observe` (in-tx) | One virtual call per service method. With no observers wired, one nil check. |
| `ObserveAfterCommit` | Same as `Observe`. |
| `ObserverGroup` fan-out | O(n) goroutines per batch where n = observer count. n is typically ≤ 5. |

No reflection. No event marshalling. No per-call allocations beyond what individual observer implementations perform internally.
