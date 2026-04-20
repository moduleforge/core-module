// Package display provides a per-(typeSlug, fieldName) renderer registry for
// surfacing human-readable strings from entity sub-type data. Each common field
// (name, description, and any future additions) is registered and invoked
// independently, so call sites never need more fields than they request and new
// fields can be added without changing existing renderers.
package display

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	coredb "github.com/moduleforge/core-model/db"
)

// FieldRenderer renders a single named field for a given entity.
// The entityID is the internal (BIGINT) entity ID. The transaction tx provides
// access to the DB in the context of an ongoing operation; it may be nil for
// read-only rendering outside a transaction (implementations should handle both).
type FieldRenderer func(ctx context.Context, tx pgx.Tx, entityID int64) (string, error)

// Canonical field name constants. Downstream modules should use these constants
// when registering or looking up renderers to avoid string typos.
const (
	FieldName        = "name"
	FieldDescription = "description"
)

// ErrRendererNotRegistered is returned by Render when no renderer is wired for
// the (typeSlug, fieldName) combination. Callers may check for this with
// errors.Is to distinguish missing-renderer from rendering errors.
var ErrRendererNotRegistered = errors.New("display: no renderer registered for type/field")

// Registry dispatches field rendering per (typeSlug, fieldName). It is
// concurrent-safe; Register and Render may be called from multiple goroutines.
//
// The registry holds a reference to a Querier so it can resolve an entity's
// fundamental_type_slug at render time, then dispatch to the registered renderer.
type Registry struct {
	mu        sync.RWMutex
	renderers map[string]map[string]FieldRenderer // [typeSlug][fieldName]
	q         coredb.Querier
}

// NewRegistry creates an empty Registry backed by q for type-slug resolution.
func NewRegistry(q coredb.Querier) *Registry {
	return &Registry{
		renderers: make(map[string]map[string]FieldRenderer),
		q:         q,
	}
}

// Register wires a renderer for one field of one type. Subsequent calls for
// the same (typeSlug, fieldName) replace the previous renderer.
func (r *Registry) Register(typeSlug, fieldName string, fn FieldRenderer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.renderers[typeSlug] == nil {
		r.renderers[typeSlug] = make(map[string]FieldRenderer)
	}
	r.renderers[typeSlug][fieldName] = fn
}

// Render looks up the entity's fundamental_type_slug via GetEntityByID, then
// invokes the renderer registered for (typeSlug, fieldName). Returns ("", nil)
// plus ErrRendererNotRegistered if no renderer is wired for that combination.
// Any DB error from slug resolution is returned directly.
func (r *Registry) Render(ctx context.Context, tx pgx.Tx, entityID int64, fieldName string) (string, error) {
	// Resolve the entity's fundamental type slug.
	var q coredb.Querier
	if tx != nil {
		q = coredb.New(tx)
	} else {
		q = r.q
	}

	entity, err := q.GetEntityByID(ctx, entityID)
	if err != nil {
		return "", fmt.Errorf("display.Render: resolve entity %d: %w", entityID, err)
	}

	typeSlug := entity.FundamentalTypeSlug

	r.mu.RLock()
	byField, ok := r.renderers[typeSlug]
	var fn FieldRenderer
	if ok {
		fn = byField[fieldName]
	}
	r.mu.RUnlock()

	if fn == nil {
		return "", fmt.Errorf("%w: typeSlug=%q fieldName=%q", ErrRendererNotRegistered, typeSlug, fieldName)
	}

	// The renderer is invoked after releasing the read lock to avoid holding the
	// lock during the database round-trip performed by the renderer function.
	return fn(ctx, tx, entityID)
}
