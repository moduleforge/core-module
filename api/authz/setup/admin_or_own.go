package setup

import "fmt"

// AdminOrOwnGenerator is the Phase 1 production AccessFuncGenerator. It
// implements the policy: an actor can see entities they own or entities they
// ARE, plus admins can see everything.
//
// This generator is application-aware: it encodes table names and ownership
// columns for the bundled peer-module Entity resources (natural_person,
// corporation, service_account, legal_entity, tag). Adding support for a new
// resource requires updating GenerateForResource.
//
// Non-Entity dependent data (e.g. contacts, which attach to legal_entities and
// have no row in the entities table) does NOT get its own access function.
// List queries for dependent data JOIN the parent's access function instead —
// for contacts that means JOINing accessible_legal_entity_ids_for_actor on
// contacts.legal_entity_id. The Authorize call for a dependent-data list
// passes the parent entity ID, not a per-resource type ID.
//
// The generator lives in core's authz/setup package because it is
// application-level wiring code, not peer-module code — peer modules must not
// depend on each other.
//
// Corporation policy note: corporations are admin-only for non-admins.
// Non-admin actors are always natural persons or service accounts, never
// corporations. A corporation entity_id will never match an actor's
// entity_id in practice, so the "OR entity_id = p_actor_entity_id" clause
// would be dead code. Omitting it keeps the policy explicit and avoids a
// misleading grant.
type AdminOrOwnGenerator struct{}

// NewAdminOrOwnGenerator constructs an AdminOrOwnGenerator.
func NewAdminOrOwnGenerator() *AdminOrOwnGenerator { return &AdminOrOwnGenerator{} }

// GenerateForResource returns the SQL SELECT body for the access function
// targeting slug. The body is wrapped in a CREATE OR REPLACE FUNCTION shell
// by GenerateFuncs.
//
// Supported slugs: natural_person, corporation, service_account, legal_entity,
// tag. Unknown slugs return an error.
func (*AdminOrOwnGenerator) GenerateForResource(slug string) (string, error) {
	switch slug {
	case "natural_person":
		return `SELECT np.entity_id FROM natural_persons np
WHERE np.entity_id = p_actor_entity_id
   OR EXISTS (SELECT 1 FROM user_accounts ua WHERE ua.account_holder = p_actor_entity_id AND ua.is_admin)`, nil

	case "corporation":
		// Admin-only: non-admin actors are never corporations, so there is no
		// per-actor ownership to grant. See type-level comment.
		return `SELECT c.entity_id FROM corporations c
WHERE EXISTS (SELECT 1 FROM user_accounts ua WHERE ua.account_holder = p_actor_entity_id AND ua.is_admin)`, nil

	case "service_account":
		return `SELECT sa.entity_id FROM service_accounts sa
WHERE sa.entity_id = p_actor_entity_id
   OR EXISTS (SELECT 1 FROM user_accounts ua WHERE ua.account_holder = p_actor_entity_id AND ua.is_admin)`, nil

	case "legal_entity":
		// Composed from the concrete sub-type functions via UNION ALL.
		// Postgres can inline LANGUAGE sql STABLE functions that call other
		// LANGUAGE sql STABLE functions, so this remains efficient.
		return `SELECT entity_id FROM accessible_natural_person_ids_for_actor(p_actor_entity_id)
UNION ALL
SELECT entity_id FROM accessible_corporation_ids_for_actor(p_actor_entity_id)`, nil

	case "tag":
		return `SELECT t.entity_id FROM tags t
WHERE t.owner_id = p_actor_entity_id
   OR t.subject_id = p_actor_entity_id
   OR EXISTS (SELECT 1 FROM user_accounts ua WHERE ua.account_holder = p_actor_entity_id AND ua.is_admin)`, nil

	default:
		return "", fmt.Errorf("AdminOrOwnGenerator: unknown resource slug %q; update GenerateForResource to support it", slug)
	}
}
