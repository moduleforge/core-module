# Entity typing in mod-core

## Motivation

To enable some level of type saftey and recognition at the `Entity` while also allowing composed modules to dynamically define allowed `Entity` types during application startup, we define a first-class `types` table acting as an append-only registry. Core defines its own types. Downstream modules declare theirs via their own migrations — no core edits required.

## Concepts

**Entity** is the abstract root. Every indepdent object in the system is an entity stored in the `entities` table.[^dependentobjects]

[^dependentobjects]: Dependent objects that only exist as part of an independent `Entity` reference the appropriate sub-type table and do not reference the `entities` table.

**Fundamental type** is the type assigned to an entity at creation. It lives in the `types` table and is referenced by `entities.fundamental_type_id`. An entity's fundamental type is fixed at creation and cannot change.

**Concrete vs. abstract types** — only concrete types may be assigned to entities. Abstract types serve as shared ancestors in the type hierarchy. For example, `legal_entity` is abstract; `natural_person` and `corporation` are concrete.

**Single-inheritance** — types form a tree rooted at `entity`. Each row in `types` has an optional `parent_id` pointing to its parent type. Ancestry is resolved with a recursive CTE.

## Rigid-designator semantics

An entity's fundamental type is a rigid designator: it fixes "what kind of thing this is" permanently. Two DB-level triggers enforce this:

- **Concrete-check** (`entities_fundamental_type_concrete_check`): fires BEFORE INSERT OR UPDATE OF fundamental_type_id. Looks up the referenced `types` row and rejects the operation if `concrete = false`.
- **Immutability** (`entities_fundamental_type_immutable`): fires BEFORE UPDATE. Rejects any attempt to change `fundamental_type_id` after the initial INSERT.

This means no migration or application code can retroactively reclassify an entity.[^possiblenamechanges]

[^possiblenamechanges]: While not currently allowed, type name changes may be supported in the future. The type ID would remain immutable in any case, however.

## Entity ownership

Every entity may have an owner: a nullable, self-referential `entities.owner_id BIGINT REFERENCES entities(id)` column, backed by the partial index `entities_owner_id_idx ON entities(owner_id) WHERE owner_id IS NOT NULL`. `owner_id` is the authoritative record of "who owns this entity" for [row-level authorization scoping](authorization-design.md#row-level-scoping) — the [generic access-function body's](authorization-design.md#access-function-shape-phase-2) own-arm reads it directly (`e.owner_id = p_actor_entity_id`), scoped by the requested type.

Two triggers govern `owner_id`, mirroring the rigid-designator pattern above:

- **Owns-itself default** (function `entities_owner_default_self`, trigger `entities_owner_self_default`): fires `BEFORE INSERT`. When `owner_id IS NULL` and the entity's fundamental type descends from `natural_person` or `service_account`, the trigger defaults `owner_id := NEW.id` — a person or service account owns itself unless the caller supplies an explicit `owner_id`. `corporation` and the `authz_actor_group`/`authz_target_group` types are never matched and keep `owner_id = NULL`. The trigger name is chosen to sort alphabetically after `entities_fundamental_type_concrete_check`, so type validation runs first on the same INSERT.
- **Immutability** (function `entities_immutable_owner`, trigger `entities_owner_immutable`): fires `BEFORE UPDATE OF owner_id`, rejecting any change once a value has been written (`IS DISTINCT FROM`, mirroring `entities_fundamental_type_immutable`). This includes the *first* NULL → non-NULL write made via `UPDATE` — the trigger does not distinguish "setting for the first time" from "changing an existing value." Consequently, any entity that is not self-owning (e.g. a `tag` or a `task`, whose owner is a different actor) must have `owner_id` set at INSERT time, not by a follow-up `UPDATE`. The model layer provides `CreateEntityWithOwner` alongside the unchanged `CreateEntity` for exactly this case (`INSERT INTO entities (fundamental_type_id, owner_id) VALUES (...)`); owning modules (`mod-tags`, `mod-tasks`) call it directly instead of `CreateEntity` followed by an owner `UPDATE`.

Migration `0013_entity_ownership.sql` adds the column and index, creates the owns-itself-default trigger, backfills existing `natural_person`/`service_account` rows to self-ownership, then creates the immutability trigger — the backfill must run before `entities_owner_immutable` exists, because backfilling afterward would trip the very trigger being added (a NULL → id transition is a `distinct` change even though nothing meaningful changed).

**Local, denormalized owner columns.** Some owning modules retain their own `owner_id` column permanently in addition to `entities.owner_id` — e.g. `mod-tags`' `tags.owner_id` and `mod-tasks`' `tasks.owner_id`. `entities.owner_id` is authoritative for authorization; the local columns remain authoritative for constraints and query shapes `entities.owner_id` cannot support directly (a cross-table `UNIQUE` index is not expressible in Postgres, and a join-based rewrite would reintroduce a sort that a single-table composite index was built to eliminate). Both local columns are immutable after insert and are set from the same value, in the same transaction, as `entities.owner_id`, so the two copies cannot diverge. Downstream modules document their own specific rationale.

## Append-only registry

The `types` table is append-only per row. New type slugs may be INSERTed; existing rows may not be UPDATEd (except to set/unset `deprecated_at`) and may never be DELETEd. Two triggers enforce this:

```sql
-- Fires BEFORE DELETE: always raises.
CREATE TRIGGER types_no_delete
  BEFORE DELETE ON types
  FOR EACH ROW EXECUTE FUNCTION types_reject_mutation();

-- Fires BEFORE UPDATE: allows only deprecated_at changes.
CREATE TRIGGER types_append_only_update
  BEFORE UPDATE ON types
  FOR EACH ROW EXECUTE FUNCTION types_reject_mutation();
```

`deprecated_at` is the single off-ramp: setting it signals that a slug is retired but preserves referential integrity for existing entities.

## Per-module type registration

Each module that introduces new subtypes adds a migration file that INSERTs into the `types` table using a parent lookup by slug:

```sql
-- Example: registering a downstream 'user_account' type under 'entity'
INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'user_account',
  id,
  true,
  'User Account',
  'An authenticated access point tied to a Legal Entity.'
FROM types WHERE slug = 'entity';
```

mod-core seeds its five built-in types in migrations `0003–0007`. Downstream modules follow the same pattern in their own migrations — no core file is touched.

The `type_is_or_descends_from(p_type_id BIGINT, p_target_slug TEXT) RETURNS BOOLEAN` helper defined in `0002_types.sql` resolves ancestry via a recursive CTE. Subtype-table triggers use this to assert correct parentage.

## Display-rendering pattern

Each concrete type can expose human-readable strings (name, description) for entities of that type. These are not stored in the DB; they are derived from subtype data at runtime by registered renderers.

The `api/display` package provides a concurrent-safe `Registry`:

```go
type FieldRenderer func(ctx context.Context, tx pgx.Tx, entityID int64) (string, error)

const (
    FieldName        = "name"
    FieldDescription = "description"
)

reg.Register("natural_person", display.FieldName, func(ctx, tx, entityID) (string, error) {
    np, _ := q.GetNaturalPersonByEntityID(ctx, entityID)
    return strings.TrimSpace(np.GivenName.String + " " + np.FamilyName.String), nil
})

name, err := reg.Render(ctx, tx, entityID, display.FieldName)
```

Each field is an independent registration. Callers request only the fields they need; there is no "render everything" API. This avoids fetching data the caller does not want and lets new fields be added without modifying existing renderers.

`Render` resolves the entity's `fundamental_type_slug` from the DB, then dispatches to the registered renderer. If no renderer is wired, it returns `ErrRendererNotRegistered` — callers may check with `errors.Is`.

mod-core registers default renderers for `natural_person`, `corporation`, and `service_account` via `service.RegisterBuiltins(reg, q)`. Downstream modules register their own renderers in their service init.

## Why accidental typing is deferred

Role-style relationships (e.g., a user account "acting as" a legal entity) are already modelled as FK links between entity rows. There is no need to tag entities with roles. Tagged accidental typing — where an entity simultaneously belongs to multiple type chains — is deferred until a concrete use case appears. Single fundamental typing covers all current requirements.

## Forward note: type-constrained FKs

Some FK columns should logically only reference entities whose fundamental type descends from a specific ancestor. For example, `user_accounts.account_holder` should only point to entities of a `legal_entity` subtype. This constraint is not yet enforced at the DB layer (it would require a trigger or generated column). It is expected to be enforced at the API service layer in a follow-on phase. The `type_is_or_descends_from` helper is available for that purpose when needed.
