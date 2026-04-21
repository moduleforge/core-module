package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/audit"
	"github.com/moduleforge/core-api/internal/fieldcrypto"
	coredb "github.com/moduleforge/core-model/db"
)

// CreateCorporationInput carries the fields required to create a corporation.
type CreateCorporationInput struct {
	LegalName    string
	Jurisdiction string // optional
	// DisplayName is no longer stored; display is derived via the display registry.
	EIN string // optional plaintext; "" means not recorded
}

// UpdateCorporationInput carries the fields that may be updated on a corporation.
// Nil fields are left unchanged.
// For EIN: nil = leave unchanged; pointer-to-"" = clear; non-empty pointer = set.
type UpdateCorporationInput struct {
	LegalName    *string
	Jurisdiction *string
	EIN          *string
}

// CorporationServicer defines corporation operations available to httpapi handlers.
type CorporationServicer interface {
	Create(ctx context.Context, q coredb.Querier, actor Principal, in CreateCorporationInput) (coredb.Corporation, uuid.UUID, error)
	GetByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error)
	UpdateByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID, in UpdateCorporationInput, actor Principal) error
}

// CorporationService implements corporation CRUD with audit logging.
type CorporationService struct {
	aw     audit.Writer
	cipher *fieldcrypto.Cipher
}

// Compile-time assertion.
var _ CorporationServicer = (*CorporationService)(nil)

// Create inserts entity → legal_entity → corporation rows in sequence.
// Requires actor.IsAdmin.
func (s *CorporationService) Create(
	ctx context.Context,
	q coredb.Querier,
	actor Principal,
	in CreateCorporationInput,
) (coredb.Corporation, uuid.UUID, error) {
	if !actor.IsAdmin {
		return coredb.Corporation{}, uuid.UUID{}, ErrForbidden
	}

	in.LegalName = strings.TrimSpace(in.LegalName)
	if in.LegalName == "" {
		return coredb.Corporation{}, uuid.UUID{}, fmt.Errorf("%w: legal_name is required", ErrInvalidInput)
	}

	// Resolve the type ID for 'corporation' from the registry.
	t, err := q.GetTypeBySlug(ctx, "corporation")
	if err != nil {
		return coredb.Corporation{}, uuid.UUID{}, fmt.Errorf("corporation.Create resolve type: %w", err)
	}

	entity, err := q.CreateEntity(ctx, t.ID)
	if err != nil {
		return coredb.Corporation{}, uuid.UUID{}, fmt.Errorf("corporation.Create entity: %w", err)
	}

	_, err = q.CreateLegalEntity(ctx, entity.ID)
	if err != nil {
		return coredb.Corporation{}, uuid.UUID{}, fmt.Errorf("corporation.Create legal_entity: %w", err)
	}

	var einBlob []byte
	einAudit := "unchanged"
	if strings.TrimSpace(in.EIN) != "" {
		blob, err := s.cipher.Encrypt(strings.TrimSpace(in.EIN))
		if err != nil {
			return coredb.Corporation{}, uuid.UUID{}, fmt.Errorf("encrypt ein: %w", err)
		}
		einBlob = blob
		einAudit = "set"
	}

	corp, err := q.CreateCorporation(ctx, coredb.CreateCorporationParams{
		EntityID:     entity.ID,
		LegalName:    in.LegalName,
		Jurisdiction: pgtype.Text{String: in.Jurisdiction, Valid: in.Jurisdiction != ""},
		Ein:          einBlob,
	})
	if err != nil {
		return coredb.Corporation{}, uuid.UUID{}, fmt.Errorf("corporation.Create corporation: %w", err)
	}

	eid := entity.ID
	_ = s.aw.Write(ctx, "create", "corporation", &eid, nil, map[string]any{
		"uuid":         entity.Uuid.String(),
		"legal_name":   in.LegalName,
		"jurisdiction": in.Jurisdiction,
		"ein":          einAudit,
	})

	return corp, entity.Uuid, nil
}

// GetByEntityUUID resolves the entity by UUID and returns its full Profile.
// The cipher stored on the service is forwarded to ResolveProfileByEntityID so
// that TaxID/TaxIDType are always populated when the cipher is configured.
func (s *CorporationService) GetByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error) {
	entity, err := q.GetEntityByUUID(ctx, entityUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Profile{}, ErrNotFound
		}
		return Profile{}, fmt.Errorf("corporation.GetByEntityUUID entity: %w", err)
	}

	profile, err := ResolveProfileByEntityID(ctx, q, entity.ID, s.cipher)
	if err != nil {
		return Profile{}, fmt.Errorf("corporation.GetByEntityUUID profile: %w", err)
	}
	return profile, nil
}

// UpdateByEntityUUID updates corporation fields for the given entity UUID.
// Requires actor.IsAdmin.
func (s *CorporationService) UpdateByEntityUUID(
	ctx context.Context,
	q coredb.Querier,
	entityUUID uuid.UUID,
	in UpdateCorporationInput,
	actor Principal,
) error {
	if !actor.IsAdmin {
		return ErrForbidden
	}

	entity, err := q.GetEntityByUUID(ctx, entityUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("corporation.UpdateByEntityUUID entity: %w", err)
	}

	corp, err := q.GetCorporationByEntityID(ctx, entity.ID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("corporation.UpdateByEntityUUID corporation: %w", err)
	}

	before := map[string]any{
		"legal_name":   corp.LegalName,
		"jurisdiction": corp.Jurisdiction.String,
		"ein":          "unchanged",
	}

	legalName := corp.LegalName
	jurisdiction := corp.Jurisdiction
	if in.LegalName != nil {
		legalName = strings.TrimSpace(*in.LegalName)
	}
	if in.Jurisdiction != nil {
		jurisdiction = pgtype.Text{String: *in.Jurisdiction, Valid: true}
	}

	// einParam: nil = leave unchanged (COALESCE keeps DB value); []byte{} = clear; non-empty = set.
	var einParam []byte
	einAudit := "unchanged"
	if in.EIN != nil {
		val := strings.TrimSpace(*in.EIN)
		if val == "" {
			einParam = []byte{} // clear
			einAudit = "cleared"
		} else {
			b, err := s.cipher.Encrypt(val)
			if err != nil {
				return fmt.Errorf("encrypt ein: %w", err)
			}
			einParam = b
			einAudit = "set"
		}
	}

	if err := q.UpdateCorporation(ctx, coredb.UpdateCorporationParams{
		EntityID:     entity.ID,
		LegalName:    legalName,
		Jurisdiction: jurisdiction,
		Ein:          einParam,
	}); err != nil {
		return fmt.Errorf("corporation.UpdateByEntityUUID update: %w", err)
	}

	after := map[string]any{
		"legal_name":   legalName,
		"jurisdiction": jurisdiction.String,
		"ein":          einAudit,
	}

	eid := entity.ID
	_ = s.aw.Write(ctx, "update", "corporation", &eid, before, after)
	return nil
}

// GetDecryptedEIN returns the plaintext EIN for the given entity.
// Returns "" if not set. Returns an error only on decrypt failure
// (i.e. stored blob is corrupt or the key is wrong) — not for NULL.
func (s *CorporationService) GetDecryptedEIN(ctx context.Context, q coredb.Querier, entityID int64) (string, error) {
	corp, err := q.GetCorporationByEntityID(ctx, entityID)
	if err != nil {
		return "", fmt.Errorf("corporation.GetDecryptedEIN: %w", err)
	}
	return s.cipher.Decrypt(corp.Ein)
}
