package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	coredb "github.com/moduleforge/core-model/db"
)

// LegalEntityServicer defines legal entity operations.
type LegalEntityServicer interface {
	GetByEntityID(ctx context.Context, q coredb.Querier, entityID int64) (coredb.LegalEntity, error)
	Create(ctx context.Context, q coredb.Querier, params coredb.CreateLegalEntityParams) (coredb.LegalEntity, error)
}

// LegalEntityService implements legal entity operations.
type LegalEntityService struct{}

// Compile-time assertion.
var _ LegalEntityServicer = (*LegalEntityService)(nil)

// GetByEntityID retrieves the legal_entity row for a given entity ID.
// Returns ErrNotFound if no matching row exists.
func (s *LegalEntityService) GetByEntityID(ctx context.Context, q coredb.Querier, entityID int64) (coredb.LegalEntity, error) {
	le, err := q.GetLegalEntityByEntityID(ctx, entityID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return coredb.LegalEntity{}, ErrNotFound
		}
		return coredb.LegalEntity{}, fmt.Errorf("legal_entity.GetByEntityID: %w", err)
	}
	return le, nil
}

// Create inserts a new legal_entity row.
func (s *LegalEntityService) Create(ctx context.Context, q coredb.Querier, params coredb.CreateLegalEntityParams) (coredb.LegalEntity, error) {
	le, err := q.CreateLegalEntity(ctx, params)
	if err != nil {
		return coredb.LegalEntity{}, fmt.Errorf("legal_entity.Create: %w", err)
	}
	return le, nil
}
