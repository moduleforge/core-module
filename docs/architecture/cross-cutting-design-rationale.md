# Cross-cutting design rationale

This document records *why* the cross-cutting framework takes the shape it does. For the *what* — interface signatures, service-method shape, composition examples — see [`cross-cutting-design.md`](cross-cutting-design.md). The audience is a future engineer deciding whether a new use case justifies reopening any of the decisions below.

## 1. The design space

Five candidate patterns were evaluated before the current design was locked.

**A. Typed interface per concern (chosen).** Constructor-injected; called explicitly at known service-method extension points. Strengths: no reflection, no startup ceremony, clear call-site ownership. Weaknesses: each concern is a named parameter in every service constructor; scaling past ~5 concerns would make constructors unwieldy.

**B. Decorator chain on service interfaces.** App composition root wires `auth(audit(metrics(svc)))`. Strengths: each decorator is self-contained and independently testable. Weaknesses: every cross-cutting concern requires a per-service wrapper type; "apply to any Entity uniformly" is awkward because it couples the decorator to the concrete service interface.

**C. Service-layer hook bus.** Central registry; services emit named events with typed payloads; consumers register handlers at startup. Strengths: concerns are fully decoupled from service code; consumers are optional and composable. Weaknesses: one handler registration per (event-type, consumer) pair scales with the product `|events| × |concerns|`. We have few concerns and many event types, so this scales worse than Pattern A in our situation.

**D. Repo-level hooks (ent/Bun-style).** Trigger callbacks on every Insert/Update/Delete in the data layer. Strengths: zero per-service boilerplate; catches all mutations automatically. Weaknesses: fires on rows, not intents. A repo-level hook cannot distinguish "password reset" from "profile edit" — both are an UPDATE on the same table. Audit and authorization both require operation-level context.

**E. HTTP middleware.** Around-handler wrappers, run before the service layer. Strengths: cross-module, reusable, well-understood. Weaknesses: runs before the service executes, so it cannot observe the entity's before/after state. HTTP middleware cannot participate in the operation's transaction.

## 2. Why Pattern A over B, C, D, E

- **D is too low.** Repo hooks see rows, not business operations. They cannot carry operation-level context (operation name, before-state) without smuggling it through the row itself, which corrupts the data model.
- **E is too high.** HTTP middleware runs before service logic. It cannot capture the actual entity state before or after the mutation, and it cannot participate in the transaction.
- **B (decorators)** would work at a small scale, but each cross-cutting concern must provide a separate wrapper per service interface. This creates a combinatorial maintenance surface as the module count grows. The "for any Entity" case is particularly awkward — a generic decorator would need the service interface to declare an Entity-returning shape, which over-constrains service design.
- **C (hook bus)** scales linearly with `|events| × |concerns|`, where Pattern A scales linearly with `|concerns|` alone. Given that we have few concerns (≤ 5 at planning time) and many event types across modules, Pattern A is the more economical choice. If the concern count grows past ~5, revisit Pattern C.

## 3. Why two interfaces, not three or more

The following concerns share a single shape: one observation point per mutation, with before-state and after-state:

- Audit (write audit row inside the operation's tx)
- Outbox (write event row inside the tx so delivery is guaranteed)
- Cache invalidation (fire after commit so readers do not re-cache between invalidate and commit)
- Search-index sync (fire after commit)
- Webhook delivery (fire after commit; meaningless on rollback)

These are all `MutationObserver` implementations. The single interface with two methods (`Observe` and `ObserveAfterCommit`) covers all of them. There is no need for separate interfaces per concern.

Authorization has a fundamentally different shape: it fires before the operation on every call including reads, it takes a single provider (no fan-out), and a non-nil return aborts the operation. This warrants its own interface (`Authorizer`).

Metrics is the borderline case. It usually wants timing and throughput data rather than before/after entity state. HTTP middleware is a more natural home for metrics because it already wraps the full request/response cycle. Metrics is treated as out of scope for this framework.

## 4. Concerns explicitly excluded from the framework

These were considered and ruled out for service-layer interface treatment:

- **Tracing (OTel)** — propagated via `ctx`; instrumentation libraries inject spans automatically. No per-service injection required.
- **Rate limiting / quota** — HTTP-layer decision; the service layer should never see a request that a rate limiter would reject.
- **Validation** — inline in service methods; not pluggable. Pluggable validation chains add indirection without a commensurate benefit for the current use cases.
- **Multi-tenancy** — folds naturally into `Authorizer`. A multi-tenant authorizer consults tenant context already on `ctx`.
- **Idempotency / retry** — handler-layer concern; idempotency keys arrive with the request and the handler decides whether to replay or forward.
- **Transactions / unit-of-work** — a coordination mechanism, not an intercept. The operation's transaction is a `tx` parameter passed through the call graph, not something that a cross-cutting hook controls.
- **Feature flags** — site-specific; no universal hook shape fits. Individual services may check feature flags inline.

## 5. Why context carries actor, not action or target

`context.Context` carries the actor entity ID (and optional assumed-actor entity ID) because these are ambient properties of the entire request — they do not change across call boundaries within a single request.

Action and target are method parameters, not context values, because they are method-specific: each service method knows its own action name and the entity it is operating on. Placing action on `ctx` would require the caller to set it before the call and the callee to trust it — this couples the two code sites and creates staleness bugs when one service method calls another (the inner call would see the outer call's action on `ctx`). Explicit parameters eliminate this hazard.

## 6. Why no inter-observer dependencies

The `ObserverGroup` dispatches all in-tx observers in parallel (via `errgroup`) and all post-commit observers in parallel. Observers cannot declare dependencies on each other.

This is a deliberate simplification, not a fundamental constraint. The use cases at planning time — audit, outbox, cache invalidation — do not require ordering. Allowing inter-observer dependencies would add startup-time dependency resolution, sequential dispatch paths, and error-propagation complexity that is not justified until a concrete use case demands it. If such a use case appears, the `ObserverGroup` can be extended.

## 7. Why three policy variants for in-tx observation

**The problem.** Different apps — and different call sites within the same app — treat observer failures differently. Audit rows are often mandatory: if the audit write fails, the operation should abort. Cache invalidation is best-effort: a cache miss is recoverable, but aborting a user-visible write because of it is not. A single hard-coded policy (always propagate, or always swallow) satisfies neither class of use case.

**The choice.** A configurable default at composition time, plus per-call overrides:

- `Observe(...)` uses the group's configured default. `PolicyPropagate` is the default-of-defaults so that unintentionally missed audit writes are visible rather than silent.
- `MustObserve(...)` always propagates, regardless of the group's configured default. Use it when the observer's success is a correctness requirement for the operation.
- `MayObserve(...)` always logs and swallows, regardless of the group's configured default. Use it when the observer is genuinely best-effort and failure must not affect the caller.

This covers the common case cheaply (one `WithPolicy` call at composition time) while letting individual service methods deviate at a specific call site without reconfiguring the whole group.

**Why the policy lives on the group, not the interface.** Observer implementations (`auditmod`, `outboxmod`, `cacheinval`) return errors honestly. They have no knowledge of whether the application treats their failure as fatal. That is an application-level decision, not an implementation decision. The `ObserverGroup` is the right home because it is the application's assembly point: it knows which observers are wired and what contract the application expects from them.

**Why policy applies only in-tx.** `ObserveAfterCommit` errors cannot abort the operation — the transaction has already committed. Logging is the only meaningful response, so there are no `Must`/`May` variants for the post-commit method.

**What is deferred.** Per-observer policy split within a single group (e.g. "audit must propagate but cache may swallow in the same call") awaits a concrete use case. Ordering and dependency resolution between observers are also deferred (see §6).

## 8. Why pre-commit and post-commit observation points, not just one

The two methods on `MutationObserver` exist because different observers need to be at different points relative to the transaction commit:

**In-tx (`Observe`):** The observer must run inside the operation's transaction and its result must commit or roll back with the operation. This is required for:
- Audit rows — if the operation rolls back, the audit row must not persist.
- Outbox rows — the event row must commit with the data; otherwise the outbox dispatcher may never see it, or may dispatch an event for an operation that did not commit.

**Post-commit (`ObserveAfterCommit`):** The observer must run *after* the transaction commits. This is required for:
- Cache invalidation — invalidating before commit creates a window where a cache miss causes readers to re-fetch and re-cache the stale pre-commit value. The invalidation must happen after the new value is durable.
- External system notifications and webhook delivery — firing on a transaction that subsequently rolls back sends a notification about a non-event. Post-commit dispatch avoids this.

An observer that only needs one of the two points implements the other as a no-op. The framework does not force every observer to do both.

## 9. Why app-side multi-observer composition rather than a core-side registry

Two reasons:

1. **Consistency with dependency injection.** App-side composition is already where services are wired with their DB pools, config, and peer-service references. Wiring observers in the same place is consistent rather than introducing a second injection mechanism.
2. **The multi-observer wrapper is trivial.** A generic wrapper over N `MutationObserver` values that dispatches them in parallel is a small amount of code. A registry with module-side registration would add module startup ordering, discovery, and deregistration logic. That complexity is unjustified when the concern count is small and known at app compile time.

If a future architecture requires dynamic observer registration (e.g., plugin-loaded observers), a registry can be introduced then. The `MutationObserver` interface does not need to change.

## 10. Open questions deferred to future revisions

- **Inter-observer dependencies.** If a use case appears where observer B must run after observer A succeeds, the multi-observer dispatch model will need to support ordering or sequential batches. See section 6.
- **General operation lifecycle hook.** The current framework covers mutation audit-shape use cases. A broader "operation lifecycle" hook (suitable for validation chains, projection updates, or pre/post hooks on reads) is not addressed. If that use case arises, evaluate whether to extend `MutationObserver` or introduce a third interface.
- **Registry-based variant.** If the number of cross-cutting concern interfaces grows past ~5, Pattern C (hook bus) or a lightweight registry should be re-evaluated. See section 2.
