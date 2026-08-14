// Package service — legal_entity.go
//
// Design note for GetTaxID: rather than passing NaturalPersonServicer /
// CorporationServicer interfaces into every call (which forces callers to thread
// two extra servicer references), LegalEntityService holds a cipher directly and
// issues the table-specific query inline. This keeps the call signature simple
// and avoids circular dependency through the servicer interfaces.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	coredb "github.com/moduleforge/core-model/db"
)

// LegalEntityTaxID is a uniform view of a legal entity's tax identifier.
// Type is "SSN" for natural persons, "EIN" for corporations, or "" if the
// entity is not one of those leaf kinds. Value is plaintext; empty string
// means "not recorded".
type LegalEntityTaxID struct {
	Type  string
	Value string
}

// LegalEntityServicer defines legal entity operations.
type LegalEntityServicer interface {
	GetByEntityID(ctx context.Context, q coredb.Querier, entityID int64) (int64, error)
	Create(ctx context.Context, q coredb.Querier, entityID int64) (int64, error)
	GetTaxID(ctx context.Context, q coredb.Querier, entityID int64) (LegalEntityTaxID, error)
}

// LegalEntityService implements legal entity operations. It is never
// constructed in practice today: it has no constructor, is absent from the
// Services aggregate, and this field is unexported, so no caller inside or
// outside mod-core can build one with a non-nil cipher. It is kept
// type-correct (cipher held as *RotatingCipher, matching every other
// encrypted-column service) so the package compiles and stays consistent with
// the rotation design; it carries no runtime risk and needs no integration
// test.
type LegalEntityService struct {
	cipher *RotatingCipher
}

// Compile-time assertion.
var _ LegalEntityServicer = (*LegalEntityService)(nil)

// GetByEntityID retrieves the entity_id from the legal_entity anchor row for a given entity ID.
// Returns ErrNotFound if no matching row exists.
func (s *LegalEntityService) GetByEntityID(ctx context.Context, q coredb.Querier, entityID int64) (int64, error) {
	id, err := q.GetLegalEntityByEntityID(ctx, entityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

// GetTaxID returns the decrypted tax identifier for the given entity.
// Dispatches on the entity's fundamental_type_slug:
//   - natural_person → SSN
//   - corporation    → EIN
//   - anything else  → zero LegalEntityTaxID (not an error)
func (s *LegalEntityService) GetTaxID(ctx context.Context, q coredb.Querier, entityID int64) (LegalEntityTaxID, error) {
	entity, err := q.GetEntityByID(ctx, entityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LegalEntityTaxID{}, ErrNotFound
		}
		return LegalEntityTaxID{}, fmt.Errorf("legal_entity.GetTaxID entity: %w", err)
	}

	switch entity.FundamentalTypeSlug {
	case "natural_person":
		np, err := q.GetNaturalPersonByEntityID(ctx, entityID)
		if err != nil {
			return LegalEntityTaxID{}, fmt.Errorf("legal_entity.GetTaxID natural_person: %w", err)
		}
		val, err := s.cipher.DecryptSSN(ctx, entityID, np.Ssn)
		if err != nil {
			return LegalEntityTaxID{}, fmt.Errorf("legal_entity.GetTaxID decrypt ssn: %w", err)
		}
		return LegalEntityTaxID{Type: "SSN", Value: val}, nil

	case "corporation":
		corp, err := q.GetCorporationByEntityID(ctx, entityID)
		if err != nil {
			return LegalEntityTaxID{}, fmt.Errorf("legal_entity.GetTaxID corporation: %w", err)
		}
		val, err := s.cipher.DecryptEIN(ctx, entityID, corp.Ein)
		if err != nil {
			return LegalEntityTaxID{}, fmt.Errorf("legal_entity.GetTaxID decrypt ein: %w", err)
		}
		return LegalEntityTaxID{Type: "EIN", Value: val}, nil

	default:
		// Not a leaf kind that carries a tax id — return zero, not an error.
		return LegalEntityTaxID{}, nil
	}
}
