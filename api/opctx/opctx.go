// Package opctx provides typed accessors for the operation-context values
// that travel on context.Context through the service layer.
//
// Three values are defined:
//
//   - ActorEntityID: the internal entity ID of the authenticated user.
//   - SudoActorEntityID: the internal entity ID of a user being impersonated
//     by an admin ("assume identity" feature). Absent when no assumption is active.
//   - RequestID: an opaque correlation string set by HTTP middleware, used for
//     logging and distributed tracing.
//
// These values are set by HTTP middleware before any service method is called.
// Service methods read them by passing the incoming ctx to the accessor functions.
// They are not method parameters because they are constant for the duration of a
// request and need not be threaded through every call site individually.
//
// Action and target are NOT context values; they are explicit parameters on
// Authorize and Observe because they differ per service call and context values
// survive call boundaries poorly when they are method-specific.
//
// EffectiveActorEntityID is a derived accessor over ActorEntityID and
// SudoActorEntityID, not a fourth context value: it stores nothing and adds
// no context key. It resolves the sudo-first-then-actor policy — when an
// admin has assumed another user's identity, the assumed identity is the
// one relevant to callers deciding who is currently acting.
package opctx

import "context"

// contextKey is a package-private type for context keys. Using an unexported
// struct type prevents collisions with keys from other packages, including
// packages that also use an integer-based key type.
type contextKey struct{ name string }

var (
	actorEntityIDKey     = contextKey{"actorEntityID"}
	sudoActorEntityIDKey = contextKey{"sudoActorEntityID"}
	requestIDKey         = contextKey{"requestID"}
)

// WithActor returns a new context carrying the given actor entity ID.
// It replaces any previously set actor entity ID.
func WithActor(ctx context.Context, entityID int64) context.Context {
	return context.WithValue(ctx, actorEntityIDKey, entityID)
}

// WithSudoActor returns a new context carrying the given sudo-actor
// entity ID (the user an admin is currently impersonating).
// It replaces any previously set sudo-actor entity ID.
func WithSudoActor(ctx context.Context, entityID int64) context.Context {
	return context.WithValue(ctx, sudoActorEntityIDKey, entityID)
}

// WithRequestID returns a new context carrying the given request correlation ID.
// It replaces any previously set request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// ActorEntityID returns the authenticated user's entity ID from ctx.
// Returns 0, false if not set.
func ActorEntityID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(actorEntityIDKey).(int64)
	return id, ok
}

// SudoActorEntityID returns the entity ID being impersonated by an admin,
// or 0, false if no impersonation is active.
func SudoActorEntityID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(sudoActorEntityIDKey).(int64)
	return id, ok
}

// EffectiveActorEntityID returns the entity ID that should be used for policy
// checks: the sudo-actor's entity ID when an admin is impersonating another
// user, since the admin is acting as that user, otherwise the authenticated
// actor's entity ID. Returns 0, false if neither is set.
func EffectiveActorEntityID(ctx context.Context) (int64, bool) {
	if id, ok := SudoActorEntityID(ctx); ok {
		return id, true
	}
	return ActorEntityID(ctx)
}

// RequestID returns the request correlation ID from ctx, or "" if not set.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
