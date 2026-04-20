package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	coredb "github.com/moduleforge/core-model/db"
)

// LegalEntityServicer defines legal entity operations.
type LegalEntityServicer interface {
	GetByEntityID(ctx context.Context, q coredb.Querier, entityID int64) (int64, error)
	Create(ctx context.Context, q coredb.Querier, entityID int64) (int64, error)
}

// LegalEntityService implements legal entity operations.
type LegalEntityService struct{}

// Compile-time assertion.
var _ LegalEntityServicer = (*LegalEntityService)(nil)

// GetByEntityID retrieves the entity_id from the legal_entity anchor row for a given entity ID.
// Returns ErrNotFound if no matching row exists.
func (s *LegalEntityService) GetByEntityID(ctx context.Context, q coredb.Querier, entityID int64) (int64, error) {
	id, err := q.GetLegalEntityByEntityID(ctx, entityID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("legal_entity.GetByEntityID: %w", err)
	}
	return id, nil
}

// Create inserts a new legal_entity anchor row for the given entity.
func (s *LegalEntityService) Create(ctx context.Context, q coredb.Querier, entityID int64) (int64, error) {
	id, err := q.CreateLegalEntity(ctx, entityID)
	if err != nil {
		return 0, fmt.Errorf("legal_entity.Create: %w", err)
	}
	return id, nil
}
