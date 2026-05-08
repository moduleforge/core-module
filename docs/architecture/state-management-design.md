# State-management design

## Purpose and scope

This document describes how state-changing operations — creates, updates, deletes, and other mutations — flow through the service layer: the transaction lifecycle, the post-mutation observation points, and how observers compose to deliver audit logging, cache invalidation, outbox/notification dispatch, search-index synchronisation, and similar behaviours.

For the pre-operation gate (authorization), see [`authorization-design.md`](authorization-design.md). For why this design (rather than middleware, decorator chains, hook bus, or repo-level hooks) was chosen, see [`cross-cutting-design-rationale.md`](cross-cutting-design-rationale.md).

## The `MutationObserver` interface

Defined in `core-module/api/observer`:

```go
type MutationObserver interface {
    // Observe runs inside the operation's transaction. The error return is
    // passed to the caller (typically an ObserverGroup), which decides
    // whether to propagate or log-and-swallow based on its configured
    // Policy and the call variant in use (see §4).
    Observe(ctx context.Context, tx pgx.Tx, op, resource string, targetEntityID *int64, before, after any) error

    // ObserveAfterCommit runs after the transaction successfully commits.
    // Callers MUST pass nil for the before parameter (post-commit observers
    // re-fetch from the DB if they need before-state). A non-nil error is
    // logged; it does not unwind the committed operation.
    ObserveAfterCommit(ctx context.Context, op, resource string, targetEntityID *int64, before, after any) error
}
```

- Used for **mutations only**. Reads do not call observers.
- An implementation may care about only one of the two phases. The other method should be a no-op returning nil.
- `op` is a short verb string: `"create"`, `"update"`, `"delete"`, plus domain-specific verbs (`"assume"`, `"login"`, `"grant"`, `"revoke"`) when audit needs them.
- `resource` is the stable resource name from `Entity.Resource()` (see [`authorization-design.md` §4](authorization-design.md)).
- `before` and `after` are JSON-serialisable snapshots; either may be nil (`before` is nil on create; `after` is nil on delete; both are nil for stateless events like `login`).

### Why two phases

Different observers need to fire at different points relative to commit:

- **In-tx (`Observe`)** must run inside the operation's transaction so that its result commits or rolls back atomically. Required for **audit rows** (a rolled-back operation must not leave an audit trail) and **outbox rows** (the event row must commit with the data, or the dispatcher will never see it — or worse, dispatch an event for an operation that did not commit).
- **Post-commit (`ObserveAfterCommit`)** must run *after* commit. Required for **cache invalidation** (invalidating before commit creates a window where readers re-cache the stale pre-commit value), **external notifications and webhooks** (firing on a transaction that subsequently rolls back sends a notification about a non-event), and **search-index sync** (same reason).

An observer that needs only one phase implements the other as `return nil`.

## The `txhelper.Run` helper

Defined in `core-module/api/txhelper`. Owns transaction lifecycle for service methods.

```go
err := txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
    // fetch before; perform the mutation; call observers in-tx
    return nil
})
```

`Run` begins a transaction, calls the function, then commits on success or rolls back on error. Service code never opens its own transaction — the helper owns the lifecycle.

After a successful commit, `Run` dispatches any post-commit observer calls that were queued via `txhelper.QueuePostCommit(ctx, ...)` from inside the function. (Currently service code calls `ObserverGroup.ObserveAfterCommit` directly after `Run` returns; `QueuePostCommit` is available for future cases where the post-commit dispatch must be enqueued from deep inside the closure.)

`txhelper.DB` is a minimal interface (`BeginTx`); `*pgxpool.Pool` satisfies it.

## The `ObserverGroup` and its three call variants

`core-module/api/observer.ObserverGroup` is a concrete fan-out helper that wraps any number of `MutationObserver` implementations. Service methods take `*ObserverGroup` (not the bare interface) so they can choose between three in-transaction call variants per operation:

| Method | Policy applied |
|--------|---------------|
| `Observe(...)` | Uses the group's configured default. The default-of-defaults is `PolicyPropagate`. |
| `MustObserve(...)` | Always propagates the first non-nil error, aborting the operation regardless of group config. |
| `MayObserve(...)` | Always logs each non-nil error per-observer and returns nil, regardless of group config. |

All three call every wrapped observer **in parallel** via `errgroup`. `Observe` and `MustObserve` return the first non-nil error; `MayObserve` logs all errors and returns nil.

`ObserveAfterCommit(...)` always logs errors and never propagates them. There are no `Must`/`May` variants — the transaction has already committed, so propagation is meaningless.

### Per-call policy: when to use each variant

- **`Observe`** — the default. Use it for ~95% of mutations. The group's configured policy decides whether observer failures abort the operation.
- **`MustObserve`** — for operations where observation success is a correctness requirement regardless of app config. Example: a privileged grant where the audit record is non-negotiable.
- **`MayObserve`** — for operations where the data change must succeed even if observation fails. Example: a high-volume hot path where audit is best-effort.

### Why the policy lives on the group, not on the implementation

Observer implementations (`audit.Observer`, `outbox.Observer`, `cache.Invalidator`) return errors honestly. They have no knowledge of whether the application treats their failure as fatal — that is an application-level decision. The `ObserverGroup` is the application's assembly point, so it is the right place to encode that decision.

### Empty-group fast path

`NewObserverGroup()` with no arguments returns a no-op group. Service code does not need nil checks; the empty-group path is a single length check with no goroutine allocation.

## Standard service-method shape

The canonical pattern. Every mutating service method across every module follows this exactly:

```go
func (s *FooService) Update(ctx context.Context, in FooUpdate) (Foo, error) {
    // 1. Authorize — abort immediately on error.
    if err := s.authz.Authorize(ctx, "update", &in.ID); err != nil {
        return Foo{}, err
    }

    // 2. Mutate inside a transaction; observers participate in the same tx.
    var out Foo
    err := txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
        before, err := s.fetchByID(ctx, tx, in.ID)
        if err != nil { return err }

        out, err = s.repo.Update(ctx, tx, in)
        if err != nil { return err }

        // Observe inside tx; an error here rolls back the operation
        // (subject to the group's policy and the chosen call variant).
        return s.observers.Observe(ctx, tx, "update", "foo", &in.ID, before, out)
    })
    if err != nil { return Foo{}, err }

    // 3. Post-commit observers. Errors are logged, not propagated.
    s.observers.ObserveAfterCommit(ctx, "update", "foo", &out.ID, nil, out)
    return out, nil
}
```

There are no variations. The agent-facing one-pager at [`skill.cross-cutting.md`](../../../skill.cross-cutting.md) is the practical checklist; this section is the architectural specification.

Read methods collapse to a single Authorize call followed by the fetch — no transaction, no observers. UUID-keyed reads resolve UUID → internal ID via `EntityResolver` before authorizing (see [`authorization-design.md` "Where the Authorize() call goes"](authorization-design.md#where-the-authorize-call-goes)).

List/search methods authorize against the type ID (resolved via `TypeResolver`) for entity-level lists, or against the parent entity ID for dependent-data lists. Row-level scoping is handled separately by SQL access functions; see [`authorization-design.md` "Row-level scoping"](authorization-design.md#row-level-scoping).

## App-side composition

Cross-cutting behaviour is composed at the application's composition root. No module knows about any peer module:

```go
authz := authzimpl.New(...)

observers := observer.NewObserverGroup(
    audit.New(...),       // writes audit_log rows in-tx
    outbox.New(...),      // writes outbox rows in-tx for async delivery
    cache.New(...),       // invalidates cache entries post-commit
).WithPolicy(observer.PolicySwallow) // omit to keep the default PolicyPropagate

userSvc    := usersservice.New(userDB, pool, authz, observers, ...)
tagSvc     := tagsservice.New(tagDB,  pool, authz, observers, ...)
contactSvc := contactsservice.New(pool, authz, observers, ...)
```

Each cross-cutting module exports a single `MutationObserver` (or a function returning one). The composition root decides which observers are active, in what combination, and what the default error policy is. Module packages remain isolated; adding or removing a cross-cutting concern is a single-place change.

Service constructors accept `*ObserverGroup` (not the bare interface) so individual service methods can call `MustObserve` or `MayObserve` at sites that need to deviate from the group's default.

## Use cases

The interface covers the common cross-cutting concerns that need to react to mutations:

| Use case | Phase | Notes |
|----------|-------|-------|
| Audit logging | in-tx | A rolled-back operation must not leave an audit row. Implemented today by `audit-module`. |
| Outbox / event dispatch | in-tx | Event row must commit with the data so the dispatcher always sees it. |
| Cache invalidation | post-commit | Invalidating before commit risks readers re-caching stale pre-commit values. |
| Search-index sync | post-commit | Indexing a rolled-back operation pollutes the index. |
| Webhook / external notification | post-commit | Notifying about an event that subsequently rolls back is incorrect. |

Some observers may want both phases (e.g. an outbox that also caches): they implement both methods accordingly.

## Performance properties

| Operation | Cost |
|-----------|------|
| `Observe` (in-tx) | One virtual call per service method. With no observers wired, one length check. |
| `ObserveAfterCommit` | Same as `Observe`. |
| `ObserverGroup` fan-out | O(n) goroutines per batch where n = observer count. n is typically ≤ 5. |
| `txhelper.Run` | Adds one tx begin / commit-or-rollback per mutating operation. |

No reflection. No event marshalling. No per-call allocations beyond what individual observer implementations perform internally.

## What this design does not do

- **No service-layer middleware or decorator chain.** Extension points are explicit interface calls, not wrapping.
- **No event bus, no event registry, no reflection.**
- **No interceptor pattern.** The framework does not intercept method calls; service methods call the interfaces directly at known points.
- **No inter-observer dependencies.** Observers run in parallel without ordering guarantees. (A deliberate simplification; see [`cross-cutting-design-rationale.md` §6](cross-cutting-design-rationale.md).)
- **Tracing** is handled by `ctx`-propagated OTel libraries; no service-layer interface is needed.
