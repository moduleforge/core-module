package setup

import "fmt"

// sharedCTEPrefix is the recursive CTE block that appears at the top of every
// GrantTableGenerator-emitted access function body. It walks UP the actor group
// hierarchy from the requesting actor, collects all grants for any of the
// requested operation IDs, and then walks DOWN the target group hierarchy to
// enumerate all leaf target entity IDs accessible by those grants.
//
// The three CTEs are:
//   - ActorChain: the actor plus every group the actor belongs to (transitively).
//   - GrantedTarget: the set of target entities/groups that are directly granted.
//   - TargetChain: all members reachable downward from GrantedTarget.
//
// Each per-resource body then adds a UNION of:
//  1. The per-resource "own" predicate (actor owns the row directly).
//  2. A join against TargetChain to include grant-reachable rows.
const sharedCTEPrefix = `WITH RECURSIVE
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
    )`

// GrantTableGenerator is the Phase 2 production AccessFuncGenerator. It
// replaces AdminOrOwnGenerator with a data-driven approach: grants stored in
// the grants table (potentially via actor/target groups) determine what rows an
// actor can see. The per-resource "own" clause preserves the same ownership
// semantics that AdminOrOwnGenerator had, without the hard-coded admin check —
// admin access is now handled by the is_admin short-circuit in the Authorizer.
//
// Per-resource own semantics (matching AdminOrOwnGenerator):
//   - natural_person: entity_id = p_actor_entity_id (the actor IS the person).
//   - corporation:    no own clause (non-admins are never corporations; a
//     corporation's entity_id will never match an actor's entity_id).
//   - service_account: entity_id = p_actor_entity_id.
//   - legal_entity:   delegates to the concrete sub-type functions (union of
//     natural_person + corporation accessible IDs), passing p_op_ids through.
//   - tag:            owner_id = p_actor_entity_id OR subject_id = p_actor_entity_id.
//
// Adding support for a new resource requires updating GenerateForResource.
// Non-Entity dependent data (e.g. contacts) does NOT get its own access
// function; list queries for such data JOIN the parent entity's access function.
type GrantTableGenerator struct{}

// NewGrantTableGenerator constructs a GrantTableGenerator.
func NewGrantTableGenerator() *GrantTableGenerator { return &GrantTableGenerator{} }

// GenerateForResource returns the SQL SELECT body for the access function
// targeting slug. The body is wrapped in a CREATE OR REPLACE FUNCTION shell by
// GenerateFuncs; the body itself uses the sharedCTEPrefix plus a per-resource
// UNION of the own-predicate and the CTE-driven grant rows.
//
// Supported slugs: natural_person, corporation, service_account, legal_entity,
// tag. Unknown slugs return an error.
func (*GrantTableGenerator) GenerateForResource(slug string) (string, error) {
	switch slug {
	case "natural_person":
		// Own predicate: the actor IS the natural person (entity_id equality).
		// Grant path: any natural_person row whose entity_id appears in TargetChain.
		return sharedCTEPrefix + `
SELECT np.entity_id FROM natural_persons np WHERE np.entity_id = p_actor_entity_id
UNION
SELECT np.entity_id FROM natural_persons np
JOIN TargetChain tc ON tc.tid = np.entity_id`, nil

	case "corporation":
		// No own-predicate: non-admin actors are never corporations, so the actor's
		// entity_id will never match a corporation's entity_id in practice.
		// Omitting the own clause keeps the policy explicit and avoids a misleading
		// grant (see AdminOrOwnGenerator type comment for the full rationale).
		// Grant path only.
		return sharedCTEPrefix + `
SELECT c.entity_id FROM corporations c
JOIN TargetChain tc ON tc.tid = c.entity_id`, nil

	case "service_account":
		// Own predicate: the actor IS the service account (entity_id equality).
		return sharedCTEPrefix + `
SELECT sa.entity_id FROM service_accounts sa WHERE sa.entity_id = p_actor_entity_id
UNION
SELECT sa.entity_id FROM service_accounts sa
JOIN TargetChain tc ON tc.tid = sa.entity_id`, nil

	case "legal_entity":
		// Delegates to the concrete sub-type functions, passing p_op_ids through.
		// Postgres inlines LANGUAGE sql STABLE functions that call other
		// LANGUAGE sql STABLE functions, so this remains efficient.
		return `SELECT entity_id FROM accessible_natural_person_ids_for_actor(p_actor_entity_id, p_op_ids)
UNION ALL
SELECT entity_id FROM accessible_corporation_ids_for_actor(p_actor_entity_id, p_op_ids)`, nil

	case "tag":
		// Own predicate: the actor owns the tag OR the actor is the subject of the
		// tag. Matches AdminOrOwnGenerator semantics; note that the admin
		// short-circuit in the Authorizer handles admins seeing all rows rather
		// than including it here.
		return sharedCTEPrefix + `
SELECT t.entity_id FROM tags t
WHERE t.owner_id = p_actor_entity_id
   OR t.subject_id = p_actor_entity_id
UNION
SELECT t.entity_id FROM tags t
JOIN TargetChain tc ON tc.tid = t.entity_id`, nil

	default:
		return "", fmt.Errorf("GrantTableGenerator: unknown resource slug %q; update GenerateForResource to support it", slug)
	}
}
