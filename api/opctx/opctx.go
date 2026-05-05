// Package opctx provides typed accessors for the operation-context values
// that travel on context.Context through the service layer.
//
// Three values are defined:
//
//   - ActorEntityID: the internal entity ID of the authenticated user.
//   - AssumedActorEntityID: the internal entity ID of a user being impersonated
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
package opctx

import "context"

// contextKey is a package-private type for context keys. Using an unexported
// struct type prevents collisions with keys from other packages, including
// packages that also use an integer-based key type.
type contextKey struct{ name string }

var (
	actorEntityIDKey        = contextKey{"actorEntityID"}
	assumedActorEntityIDKey = contextKey{"assumedActorEntityID"}
	requestIDKey            = contextKey{"requestID"}
)

// WithActor returns a new context carrying the given actor entity ID.
// It replaces any previously set actor entity ID.
func WithActor(ctx context.Context, entityID int64) context.Context {
	return context.WithValue(ctx, actorEntityIDKey, entityID)
}

// WithAssumedActor returns a new context carrying the given assumed-actor
// entity ID (the user an admin is currently impersonating).
// It replaces any previously set assumed-actor entity ID.
func WithAssumedActor(ctx context.Context, entityID int64) context.Context {
	return context.WithValue(ctx, assumedActorEntityIDKey, entityID)
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

// AssumedActorEntityID returns the entity ID being impersonated by an admin,
// or 0, false if no impersonation is active.
func AssumedActorEntityID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(assumedActorEntityIDKey).(int64)
	return id, ok
}

// RequestID returns the request correlation ID from ctx, or "" if not set.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
