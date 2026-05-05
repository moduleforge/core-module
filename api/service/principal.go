// Package service exposes tx-aware entity CRUD for consumer apps.
// Consumer modules wire in their own auth implementation via PrincipalExtractor
// and observation via observer.MutationObserver.
package service

import "context"

// Principal is the normalized caller identity passed to service methods
// for authorization checks. It intentionally omits email, roles, and
// assumed-user info — those belong in the consumer's richer context type.
type Principal struct {
	UserID   int64
	EntityID int64
	IsAdmin  bool
}

// PrincipalExtractor lifts a Principal from request context. Consumers
// implement this against their own auth middleware's context keys.
// Returns (nil, false) if the caller is unauthenticated.
type PrincipalExtractor interface {
	FromContext(ctx context.Context) (*Principal, bool)
}
