// Package authz defines the Authorizer interface consumed by all service
// methods in the system.
//
// Authorizer is intentionally narrow: one method, called pre-op, on every
// operation including reads. The implementation is consumer-supplied; this
// package defines only the contract.
//
// The acting user's identity is resolved from ctx by the implementation using
// the opctx package (ActorEntityID, SudoActorEntityID). operation and target
// are explicit parameters because they differ per service call and context
// values survive call boundaries poorly when they are method-specific.
//
// A non-nil error aborts the calling service method immediately. Implementations
// should return distinguishable error values so HTTP handlers can map
// "not authenticated" to 401 and "authenticated but not allowed" to 403.
package authz

import "context"

// Authorizer gates every operation in the system. A single Authorizer
// implementation is wired per application at the composition root; peer modules
// receive it via their service constructors.
//
// operation is a stable, lower-case verb string describing the operation:
//
//	"read", "list", "create", "update", "delete"
//
// "search" is not a separate operation — searches authorize as "list" (same
// security question: may this actor discover this resource?). Domain-specific
// verbs (e.g. "assume", "login", "grant", "revoke") are permitted where the
// standard set is insufficient.
//
// target is the internal entity ID of the object being acted on, or nil when
// no specific target exists yet (create / list / search). The Authorizer
// operates *before* any retrieval or instantiation of the target; it never
// needs more than the ID. If policy needs the target's resource type, the
// implementation looks it up by ID.
//
// The Authorizer resolves the effective actor from ctx via the opctx accessors.
// When SudoActorEntityID is set on ctx, the Authorizer must treat that ID as
// the effective actor for all policy checks (the admin's own identity is not the
// subject of the request for the duration of an assume session).
type Authorizer interface {
	Authorize(ctx context.Context, operation string, target *int64) error
}

// OpResolver resolves an operation slug to the set of operation IDs that
// satisfy it (the reverse-implies / satisfied-by closure). Peer modules that
// issue list queries against access functions use this to compute the op_ids
// slice without importing the full authz-api module.
//
// The returned slice is owned by the resolver; callers MUST NOT mutate it.
// Returns nil and an error for unknown slugs.
//
// OperationRegistry in authz-api implements this interface. Inject it at the
// composition root (users-module main.go) and pass it down to every service
// constructor that issues list queries.
type OpResolver interface {
	SatisfiedBy(slug string) ([]int32, error)
}
