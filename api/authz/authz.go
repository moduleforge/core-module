// Package authz defines the Authorizer interface consumed by all service
// methods in the system.
//
// Authorizer is intentionally narrow: one method, called pre-op, on every
// operation including reads. The implementation is consumer-supplied; this
// package defines only the contract.
//
// The acting user's identity is resolved from ctx by the implementation using
// the opctx package (ActorEntityID, AssumedActorEntityID). action and target
// are explicit parameters because they differ per service call and context
// values survive call boundaries poorly when they are method-specific.
//
// A non-nil error aborts the calling service method immediately. Implementations
// should return distinguishable error values so HTTP handlers can map
// "not authenticated" to 401 and "authenticated but not allowed" to 403.
package authz

import (
	"context"

	"github.com/moduleforge/core-api/entity"
)

// Authorizer gates every operation in the system. A single Authorizer
// implementation is wired per application at the composition root; peer modules
// receive it via their service constructors.
//
// action is a stable, lower-case verb string describing the operation:
//
//	"read", "list", "create", "update", "delete", "assume"
//
// Domain-specific verbs are permitted where the standard set is insufficient.
//
// target is the Entity being acted on. For create operations where no entity ID
// exists yet, the target may carry only the result of Resource() and return nil
// from EntityID().
//
// The Authorizer resolves the effective actor from ctx via the opctx accessors.
// When AssumedActorEntityID is set on ctx, the Authorizer must treat that ID as
// the effective actor for all policy checks (the admin's own identity is not the
// subject of the request for the duration of an assume session).
type Authorizer interface {
	Authorize(ctx context.Context, action string, target entity.Entity) error
}
