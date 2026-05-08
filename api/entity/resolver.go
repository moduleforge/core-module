package entity

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	coredb "github.com/moduleforge/core-model/db"
)

// ErrForbidden is returned by Resolver.Resolve when a UUID is not found and
// the resource has not opted into 404 behaviour. Returning a 403 instead of
// a 404 masks existence: a caller cannot distinguish "no such entity" from
// "entity exists but you lack access".
var ErrForbidden = errors.New("entity: forbidden")

// ErrNotFound is returned by Resolver.Resolve when a UUID is not found and
// the resource has explicitly opted into 404 behaviour via AllowNotFound.
var ErrNotFound = errors.New("entity: not found")

// Resolver maps an entity UUID to its internal int64 ID while applying a
// per-resource not-found policy.
//
// Default policy: return ErrForbidden when the UUID is absent from the
// entities table. This masks entity existence from callers who have not been
// granted access. Resources that should surface a 404 to legitimate callers
// (e.g. a public-facing resource lookup) may opt in via AllowNotFound.
type Resolver struct {
	notFoundIs404 map[string]bool
}

// NewResolver constructs a Resolver with the default 403-on-missing policy.
func NewResolver() *Resolver {
	return &Resolver{notFoundIs404: map[string]bool{}}
}

// AllowNotFound opts the given resource slug into 404 (ErrNotFound) behaviour
// when the UUID is absent from the entities table. Returns the receiver for
// chaining.
func (r *Resolver) AllowNotFound(resourceSlug string) *Resolver {
	r.notFoundIs404[resourceSlug] = true
	return r
}

// Resolve looks up the entity identified by u in the entities table and
// returns its internal int64 ID.
//
// When the UUID is not found:
//   - If resourceSlug has been opted in via AllowNotFound, ErrNotFound is
//     returned.
//   - Otherwise ErrForbidden is returned (default, masks existence).
//
// Other database errors are returned wrapped with context.
func (r *Resolver) Resolve(ctx context.Context, q coredb.Querier, u uuid.UUID, resourceSlug string) (int64, error) {
	row, err := q.GetEntityByUUID(ctx, u)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if r.notFoundIs404[resourceSlug] {
				return 0, ErrNotFound
			}
			return 0, ErrForbidden
		}
		return 0, fmt.Errorf("entity.Resolve %s %s: %w", resourceSlug, u, err)
	}
	return row.ID, nil
}
