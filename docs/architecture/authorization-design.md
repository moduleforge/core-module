# Authorization design

## Purpose and scope

This document describes how authorization decisions are integrated into service-layer operations across all peer modules. Authorization is the pre-operation gate: every request consults a single per-application policy before doing any work.

For why this design (rather than middleware, decorator chains, or a hook bus) was chosen, see [`cross-cutting-design-rationale.md`](cross-cutting-design-rationale.md). For the post-operation half of the lifecycle — observers, transactions, and post-commit dispatch — see [`state-management-design.md`](state-management-design.md).

## The `Authorizer` interface

Defined in `core-module/api/authz`:

```go
type Authorizer interface {
    Authorize(ctx context.Context, operation string, target *int64) error
}
```

- **One `Authorizer` per app.** No fan-out. The application's composition root constructs a single instance and injects it into every peer-module service.
- **Called pre-op on all operations**, including reads. Requests *must* be authorized before any action is taken.
- **A non-nil return aborts immediately.** Callers return the error as-is; service methods do not wrap or suppress it. HTTP handlers map known sentinel errors (`ErrUnauthenticated`, `ErrForbidden`) to 401/403 status codes.
- **`target` is an `*int64` whose meaning depends on the operation** — see the call-shape table below. The Authorizer operates *before* any retrieval or instantiation of the target.
- **The actor is resolved from `ctx`**, not from a method parameter. See [Operation context](#operation-context-opctx).
- **`operation` and `target` are explicit method parameters** — they are method-specific and do not belong on the context. Putting them on `ctx` would create staleness bugs when one service method calls another.

### Call-shape table

| Operation | Target | Resolved via | Notes |
|---|---|---|---|
| `create` | `*int64` typeID | `TypeResolver` | The type ID of the entity being created (slug → ID via the `types` registry). Conveys "may this actor create entities of this type?" |
| `list` (entity) | `*int64` typeID | `TypeResolver` | Same convention as create. "May this actor list entities of this type at all?" Row-level scoping is applied separately by the access function (see [Row-level scoping](#row-level-scoping)). |
| `list` (dependent data) | `*int64` parent entityID | request input | e.g. contacts under a legal entity, tags under a subject. Authz delegates to the parent entity's read policy. |
| `read` / `update` / `delete` | `*int64` entityID | `EntityResolver` for UUID-keyed routes | The specific entity being acted on. UUID inputs are resolved to entity IDs before the Authorize call. |
| `assume` | `*int64` entityID | request input | The entity whose identity is being assumed. |
| `login` | `*int64` entityID | resolved from credentials | The user_account entity being authenticated. |
| `grant` / `revoke` | `*int64` entityID | request input | The entity being granted or revoked a permission. |

The distinction between typeID and entityID targets is a convention enforced by the call site. Both are `int64` and there is no runtime tag distinguishing them. Implementations that need the distinction MUST consult this table.

## Operation context (`opctx`)

The following values travel on `context.Context` through the service layer. They are set by HTTP middleware before the service is called.

| Key | Type | Set by | Meaning |
|-----|------|--------|---------|
| `actor_entity_id` | `int64` | auth middleware | Internal entity ID of the authenticated user. |
| `assumed_actor_entity_id` | `*int64` | auth middleware | Internal entity ID of the user whose identity an admin has assumed. Nil when no assumption is active. |
| `request_id` | `string` | HTTP middleware | Request correlation ID for logging and tracing. |

`opctx` lives in `core-module/api/opctx`. It is deliberately narrow: only ambient request properties go here. Richer policy data (roles, scopes, tenancy, app context) is the `Authorizer` implementation's concern, not `opctx`'s — the implementation can resolve those from the actor's entity ID via its own DB lookups.

When an admin assumes another user's identity, the authorizer reads the **assumed** actor for policy purposes (the admin is acting *as* that user), and the original admin ID for audit purposes. Both are available on `ctx` simultaneously.

Authorizations do not consider involved `Entities` outside the subject and target `Entities`. I.e., if a user is authorized to read an `Entity`, then they can, by implication, generally associate and reference that `Entity`. If there is a business rule that would forbid this in certain cases, that question must be resolved at the application level and cannot be enforced through the authorization regime alone. E.g., let's say an application has the concept of "my groups" consisting of the user's own projects and the user also has read access to projects which are not there own. Currently, the authorization question can only resolve "is user X authorized to edit group Y". Whether or not project Z may be included in group Y must be resolved by the application either through another authorization check or other means.

## Entity contract

The `Authorizer` does not consume the `Entity` interface directly — it only takes an ID. But the broader system uses an `Entity` interface for service-layer types. The interface is small:

```go
type Entity interface {
    // Resource returns the stable resource name for this entity type.
    // Examples: "natural_person", "tag", "contact".
    Resource() string

    // EntityID returns the internal entity ID, or nil for not-yet-persisted
    // entities (the create case where no DB-assigned ID exists yet). This
    // value should never be publicly displayed, logged, or persisted anywhere.
    // Admins may specifically request the ID.
    EntityID() *int64

    // PublicUUID returns the public UUID. This value should be used for any
    // public reference, including display to the user and log entries.
    // Returns "" for not-yet-persisted entities.
    PublicUUID() string
}
```

`Resource()` is the canonical slug. It corresponds to the `fundamental_type_slug` used in the entity-typing system (see [`entity-typing.md`](entity-typing.md)). It is consumed by observers (see [`state-management-design.md`](state-management-design.md)) and by SQL access functions for row-level scoping (see [Row-level scoping](#row-level-scoping)).

`PublicUUID()` is what callers should use whenever they need to display, log, or otherwise externally reference an entity. `EntityID()` stays internal.

For UUID-keyed routes (`GET /natural-persons/{uuid}`, etc.), services use the `EntityResolver` to translate UUID → internal ID before calling `Authorize`. The resolver applies a per-resource not-found policy:

- **Default: `ErrForbidden`.** A missing UUID looks identical to "you don't have access" — the API does not reveal whether the entity exists. This is the privacy-conservative default and is what every resource uses today.
- **Opt-in: `ErrNotFound`.** The composition root may call `EntityResolver.AllowNotFound("<slug>")` for resources where existence-leak is acceptable (typically: public reference data). No resource opts in today; the option is reserved for future use cases.

## Where the `Authorize()` call goes

Every service method begins with a single Authorize call:

```go
func (s *FooService) Update(ctx context.Context, in FooUpdate) (Foo, error) {
    if err := s.authz.Authorize(ctx, "update", in.EntityID()); err != nil {
        return nil, err
    }
    // … rest of the standard service-method shape (see state-management-design.md §5)
    return updatedFoo, nil
}
```

Read methods that already have an internal ID:

```go
func (s *FooService) Get(ctx context.Context, id int64) (Foo, error) {
    if err := s.authz.Authorize(ctx, "read", &id); err != nil {
        return nil, err
    }
    return s.repo.Get(ctx, id), nil
}
```

UUID-keyed reads resolve UUID → ID first via `EntityResolver`, then authorize:

```go
func (s *FooService) GetByUUID(ctx context.Context, uuid uuid.UUID) (Foo, error) {
    id, err := s.entityResolver.Resolve(ctx, s.q, uuid, "foo") // returns ErrForbidden if not found
    if err != nil { return Foo{}, err }
    if err := s.authz.Authorize(ctx, "read", &id); err != nil {
        return Foo{}, err
    }
    return s.repo.Get(ctx, id)
}
```

Create and entity-level list methods authorize against the type ID:

```go
func (s *FooService) Create(ctx context.Context, in CreateFooInput) (Foo, error) {
    typeID := s.typeResolver.IDForSlugMust("foo")
    if err := s.authz.Authorize(ctx, "create", &typeID); err != nil {
        return Foo{}, err
    }
    // ...
}
```

Operation strings are stable lower-case verbs: `read`, `list`, `create`, `update`, `delete`, plus domain-specific verbs (`assume`, `login`, `grant`, `revoke`) for special cases. **`search` is not a separate operation — searches use `list`.** A user authorized to `list` a resource is authorized to discover it by any filter; a user not authorized to `list` cannot search either. The two represent the same security question.

`Authorize()` is called from the **service layer**, not from HTTP handlers. Handlers must not duplicate the call — the service is the authoritative gate. This keeps the contract uniform regardless of how a service is invoked (HTTP today; potentially gRPC, message-queue handlers, or scheduled jobs in future).

## Row-level scoping

`Authorize()` is a binary gate — it answers "may this actor perform this operation at all?" with a single return value. It does not answer "of the rows that match a list/search query, which ones may this actor see?" That second question — row-level scoping — is handled by **SQL set-returning policy functions**, not by the `Authorizer` interface.

### Convention

For each Entity resource that supports `list`, the schema declares a function:

```sql
CREATE OR REPLACE FUNCTION accessible_<resource>_ids_for_actor(p_actor_entity_id BIGINT)
RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE AS $$
    -- policy body: which entity IDs may this actor see?
$$;
```

List/search sqlc queries `JOIN` against this function:

```sql
SELECT t.entity_id, t.owner_id, ..., e.uuid
FROM tags t
JOIN entities e ON e.id = t.entity_id
JOIN accessible_tag_ids_for_actor(@actor_entity_id) acc ON acc.entity_id = t.entity_id
WHERE ...
LIMIT @limit OFFSET @offset;
```

This pushes row-level filtering into Postgres where the planner can index-scan, instead of fetching all rows and filtering in Go. Combined with mandatory pagination (`LIMIT`/`OFFSET`), the query returns at most `limit` rows that the actor may see.

### Function bodies are app-level wiring, not migration content

Each peer module ships **stubs** in its migration files (`0099_access_stubs.sql`, `0299_access_stubs.sql`, etc.) that define the function with a placeholder body returning the empty set. This satisfies `sqlc compile` and lets the schema migrate cleanly.

At app startup, after migrations have been applied, the composition root calls `setup.ApplyFuncs(ctx, pool, generator, slugs)` from `core-module/api/authz/setup`. The generator (an `AccessFuncGenerator` implementation supplied by the chosen Authorizer) replaces each stub body via `CREATE OR REPLACE FUNCTION` with the real policy. Different apps may use different generators (admin-or-own, grant-table-based, etc.) without changing the peer-module schemas.

For tests, `setup.PermissiveGenerator(tableForSlug)` produces bodies that return all rows for all actors; `setup.DenyingGenerator()` produces bodies that return the empty set.

### Two mechanisms, one design

`Authorize` and the access functions are complementary:

- `Authorize` runs in Go before any query. It gates the operation. Cheap to deny.
- The access function runs in SQL as part of the list/search query. It scopes the result set. Cheap to evaluate when the function inlines (`LANGUAGE sql STABLE` single-SELECT bodies inline cleanly in Postgres 12+).

A `list` operation must satisfy both: `Authorize(ctx, "list", &typeID)` permits the operation in principle; the access function then determines which specific rows are visible. A user denied at the gate gets 403; a user permitted at the gate but with no accessible rows gets an empty paged result.

Single-target operations (`read`, `update`, `delete`) only consult `Authorize` — the entity ID is already known and there's no result set to scope.

### Production policy — `GrantTableGenerator` (Phase 2, active)

The current production policy replaces `AdminOrOwnGenerator` with a data-driven `GrantTableGenerator` backed by the `authz-module` schema (migration range `0500–0599`). The high-level pieces are:

#### Operation registry and `OpResolver`

`authz-module` owns an `authz_operations` table seeded with 11 operations:

| slug | implies |
|---|---|
| `read` | — |
| `sread` | `read` |
| `list` | `read` |
| `update` | `read` |
| `delete` | `read` |
| `swrite` | `sread`, `update` |
| `manage` | all other ops (the all-ops grant) |
| `assume` | — |
| `login` | — |
| `grant` | — |
| `revoke` | — |

`OperationRegistry` (in `authz-module/api/authz`) loads these at startup and caches two transitive-closure views:

- **Forward implies**: what does this op grant transitively? (For management UI display.)
- **Reverse / `SatisfiedBy`**: which ops, if granted, satisfy a request for this op? This closure drives every `Authorize` call and every `JOIN ... op_ids` in list queries.

The `OpResolver` interface in `core-module/api/authz` exposes `SatisfiedBy` to peer modules without requiring them to import `authz-api`:

```go
type OpResolver interface {
    SatisfiedBy(slug string) ([]int32, error)
}
```

`OperationRegistry` implements `OpResolver`. It is injected at the composition root and passed to every service constructor that issues list queries.

The `SatisfiedByMust` / `SatisfiedBy` distinction: `Must` panics on unknown slugs and is used at composition root init time; `SatisfiedBy` returns an error and is used at call sites inside service methods.

#### Actor and target groups — schema and cycle prevention

`authz-module` adds two additional entity types:

- `authz_actor_groups` — nestable groups of actors (UserAccounts or other actor groups). CTI under `entities`.
- `authz_target_groups` — nestable groups of targets (any Entity). CTI under `entities`.

Join tables (`authz_actor_group_members`, `authz_target_group_members`) reference `entities.id`. Cycle prevention is enforced at write time via database triggers; the triggers walk the membership chain and reject an insert that would create a cycle. Member-type rules (actor group members must be user_accounts or actor groups; target group members may be any entity) are likewise enforced by triggers using `core-module`'s `type_is_or_descends_from()` helper function.

Both group types carry `ON DELETE CASCADE` from `entities`, so archiving the underlying entity removes group membership automatically.

#### Grants table

```sql
CREATE TABLE grants (
    id           BIGINT PRIMARY KEY,
    actor_id     BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    operation_id INT    NOT NULL REFERENCES authz_operations(id),
    target_id    BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    granted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by   BIGINT NOT NULL REFERENCES entities(id),
    UNIQUE (actor_id, operation_id, target_id)
);
```

Both `actor_id` and `target_id` cascade on entity delete. Revoking a grant is a hard delete (not an archive); the unique constraint prevents duplicate grants.

#### Access function shape (Phase 2)

The access function signature gained an `op_ids` parameter in Phase 2:

```sql
accessible_<resource>_ids_for_actor(p_actor_entity_id BIGINT, p_op_ids INT[])
RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE
```

The shared recursive-CTE prefix generated by `GrantTableGenerator`:

```sql
WITH RECURSIVE
    ActorChain AS (
        SELECT p_actor_entity_id AS aid
        UNION
        SELECT agm.group_id
        FROM authz_actor_group_members agm
        JOIN ActorChain ac ON agm.member_id = ac.aid
    ),
    GrantedTarget AS (
        SELECT g.target_id AS tid
        FROM grants g
        JOIN ActorChain ac ON g.actor_id = ac.aid
        WHERE g.operation_id = ANY(p_op_ids)
    ),
    TargetChain AS (
        SELECT tid FROM GrantedTarget
        UNION
        SELECT atgm.member_id
        FROM authz_target_group_members atgm
        JOIN TargetChain tc ON atgm.group_id = tc.tid
    )
```

This is prepended to each per-resource UNION of (a) the "own" predicate (actor owns the row directly) and (b) a JOIN against `TargetChain` to include grant-reachable rows.

List/search sqlc queries pass `SatisfiedBy("read")` (or the appropriate op slug) as `op_ids`:

```sql
JOIN accessible_tag_ids_for_actor(@actor_entity_id, sqlc.arg(op_ids)::int[]) acc
  ON acc.entity_id = t.entity_id
```

Service layer:

```go
opIDs, err := s.opRes.SatisfiedBy("read")
if err != nil { return nil, fmt.Errorf("resolve op_ids: %w", err) }
return s.repo.List(ctx, ListParams{ActorEntityID: actorID, OpIds: opIDs, Limit: p.Limit, Offset: p.Offset})
```

#### Single-row Authorize semantics

For `Authorize(ctx, "update", &target)`:

1. Resolve `"update"` → satisfied-by closure via `OperationRegistry.SatisfiedBy`.
2. Short-circuit if actor has `is_admin = true` (current simplification; see below).
3. Otherwise: recursive-CTE query that walks UP from the actor (via `ActorChain`) and UP from the target (via `TargetChain`), returning `EXISTS(SELECT 1 FROM grants WHERE actor_id IN ActorChain AND target_id IN TargetChain AND operation_id = ANY(op_ids))`.

"Own" semantics for single-row reads are expressed per-resource either as a separate predicate or folded into the access-function body; the impl chooses the cleaner approach per resource.

#### `is_admin` short-circuit (current simplification)

The `Authorizer` implementation checks `is_admin` in Go before issuing any grants query. Admins bypass row-level scoping entirely — they see all rows and pass every `Authorize` call. This is the current simplification (Q8-A from phase-2-design.md). The `manage` operation is available for grant-driven admin equivalents in the future; the short-circuit is documented as a simplification rather than a long-term design decision.

#### Admin API for authz management (Phase 2.4)

`authz-module/api` provides CRUD endpoints for operations, actor groups, target groups, grants, and memberships under `/v1/authz/*`. All endpoints are admin-only via the `is_admin` short-circuit. See the phase-2-design.md §Phase 2.4 for the full endpoint list.

### Future evolution — Phase 3

**Trigger-maintained access tables.** If indirect-grant fan-out (e.g. group-of-users grants org-of-entities) creates measurable contention or perf issues, per-resource `*_access` tables (e.g. `tag_access(actor_entity_id, tag_entity_id, can_read)`) materialise the grants for fast JOIN. Triggers on `grants` keep the access tables in sync. Function bodies become `SELECT entity_id FROM tag_access WHERE actor_entity_id = $1 AND can_read` — still a single-SELECT `LANGUAGE sql STABLE` shape that inlines. Peer-module query files still don't change.

The function-as-policy-boundary is the durable contract across both Phase 2 and Phase 3.

## What the framework does not authorize

- **HTTP-layer concerns** — rate limiting, quotas, idempotency keys. These run at the HTTP boundary; the service should never see a request a rate-limiter would reject.
- **Validation** — input shape, format, and field-level constraints stay inline in service methods. Authorization is policy ("may this actor do this?"), not data correctness.
- **Multi-tenancy** — folds into the `Authorizer` implementation. A multi-tenant authorizer reads tenant context already on `ctx` and applies tenant-scoped rules; no separate framework piece is needed.
- **Feature flags** — site-specific. Individual services may consult feature flags inline if a behaviour is gated, independent of the authz decision.

## Performance

- One virtual call per service method (`Authorize`). No reflection, no marshalling, no allocations beyond what the implementation does internally.
- Authorize implementations are expected to perform at most one DB lookup per call, though this may involve fetching from multiple sources such as determining whether the actor `is_admin` or has an explicit grant. Implementations may cache.
- Authorize denial returns immediately; no transaction is opened, no observer fires. Authorization requests and results may be logged by the authorization implementation directly.
