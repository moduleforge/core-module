package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/moduleforge/core-api/authz"
	"github.com/moduleforge/core-api/display"
	"github.com/moduleforge/core-api/entity"
	coredb "github.com/moduleforge/core-model/db"
)

// NewDisplayRegistry constructs a display.Registry backed by q and registers
// mod-core's builtin renderers (natural_person, corporation, service_account)
// on it. This is the single call an mfgen-composed app makes to obtain a
// registry that already knows mod-core's three concrete types.
//
// display_builtins.go's RegisterBuiltins remains callable directly for a
// caller that already holds its own registry.
func NewDisplayRegistry(q coredb.Querier) *display.Registry {
	reg := display.NewRegistry(q)
	RegisterBuiltins(reg, q)
	return reg
}

// DisplayServicer defines the display-rendering operation available to HTTP
// handlers.
type DisplayServicer interface {
	// RenderField resolves entityUUID, authorizes "read" on the resulting
	// entity, then renders fieldName through the service's registry.
	//
	// available is true when a renderer produced a value and false when the
	// value could not be produced for an expected, non-error reason (no
	// renderer registered for the entity's type, or no registry wired).
	// available == false is only ever returned alongside a nil error: a
	// failure to resolve or authorize the entity is a real, propagated error,
	// never collapsed into an "unavailable" result.
	RenderField(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID, fieldName string) (string, bool, error)
}

// DisplayService implements DisplayServicer by resolving and authorizing the
// target entity, then dispatching through a display.Registry.
type DisplayService struct {
	reg            *display.Registry
	az             authz.Authorizer
	entityResolver *entity.Resolver
}

// Compile-time assertion.
var _ DisplayServicer = (*DisplayService)(nil)

// NewDisplayService constructs a DisplayService. reg may be nil — a
// deployment that wires no registry is a valid state; RenderField reports it
// as available == false rather than failing.
func NewDisplayService(reg *display.Registry, az authz.Authorizer, entityResolver *entity.Resolver) *DisplayService {
	return &DisplayService{reg: reg, az: az, entityResolver: entityResolver}
}

// RenderField implements DisplayServicer.
func (s *DisplayService) RenderField(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID, fieldName string) (string, bool, error) {
	// Resolve UUID → internal ID; default policy masks missing as 403. Errors
	// are propagated verbatim — the resolver's sentinels are already
	// apiresp-classifiable.
	internalID, err := s.entityResolver.Resolve(ctx, q, entityUUID, "entity")
	if err != nil {
		return "", false, err
	}

	// Authorize the read. This is the real authorization gate: holding a UUID
	// entitles a caller to nothing.
	if err := s.az.Authorize(ctx, "read", &internalID); err != nil {
		return "", false, err
	}

	// A deployment that wires no registry is a valid state, not a failure.
	// This check comes after authorization so the "unavailable" outcome is
	// never reachable without a successful read authorization.
	if s.reg == nil {
		return "", false, nil
	}

	// nil tx: the registry and every builtin renderer fall back to their base
	// querier for read-only rendering outside a transaction.
	value, err := s.reg.Render(ctx, nil, internalID, fieldName)
	if err != nil {
		if errors.Is(err, display.ErrRendererNotRegistered) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("display.RenderField: %w", err)
	}
	return value, true, nil
}
