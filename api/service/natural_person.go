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

// CreateNaturalPersonInput carries the fields required to create a natural person.
type CreateNaturalPersonInput struct {
	GivenName  string
	FamilyName string
	// DisplayName is no longer stored; display is derived via the display registry.
	SSN string // optional plaintext; "" means not recorded
}

// UpdateNaturalPersonInput carries the fields that may be updated on a natural person.
// Nil fields are left unchanged.
// For SSN: nil = leave unchanged; pointer-to-"" = clear; non-empty pointer = set.
type UpdateNaturalPersonInput struct {
	GivenName  *string
	FamilyName *string
	SSN        *string
}

// NaturalPersonServicer defines natural person operations available to httpapi handlers.
type NaturalPersonServicer interface {
	Create(ctx context.Context, q coredb.Querier, actor Principal, in CreateNaturalPersonInput) (coredb.NaturalPerson, uuid.UUID, error)
	GetByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error)
	UpdateByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID, in UpdateNaturalPersonInput, actor Principal) error
}

// NaturalPersonService implements natural person CRUD with audit logging.
type NaturalPersonService struct {
	aw     audit.Writer
	cipher *fieldcrypto.Cipher
}

// Compile-time assertion.
var _ NaturalPersonServicer = (*NaturalPersonService)(nil)

// Create inserts entity → legal_entity → natural_person rows in sequence.
// The caller is responsible for transaction lifecycle: pass a tx-backed Querier
// (via coredb.New(tx)) for multi-table atomicity. Returns the new record and
// the entity's public UUID. Requires actor.IsAdmin.
func (s *NaturalPersonService) Create(
	ctx context.Context,
	q coredb.Querier,
	actor Principal,
	in CreateNaturalPersonInput,
) (coredb.NaturalPerson, uuid.UUID, error) {
	if !actor.IsAdmin {
		return coredb.NaturalPerson{}, uuid.UUID{}, ErrForbidden
	}

	in.GivenName = strings.TrimSpace(in.GivenName)
	in.FamilyName = strings.TrimSpace(in.FamilyName)
	if in.GivenName == "" {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("%w: given_name is required", ErrInvalidInput)
	}
	if in.FamilyName == "" {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("%w: family_name is required", ErrInvalidInput)
	}

	// Resolve the type ID for 'natural_person' from the registry.
	t, err := q.GetTypeBySlug(ctx, "natural_person")
	if err != nil {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("natural_person.Create resolve type: %w", err)
	}

	entity, err := q.CreateEntity(ctx, t.ID)
	if err != nil {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("natural_person.Create entity: %w", err)
	}

	_, err = q.CreateLegalEntity(ctx, entity.ID)
	if err != nil {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("natural_person.Create legal_entity: %w", err)
	}

	var ssnBlob []byte
	ssnAudit := "unchanged"
	if strings.TrimSpace(in.SSN) != "" {
		blob, err := s.cipher.Encrypt(strings.TrimSpace(in.SSN))
		if err != nil {
			return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("encrypt ssn: %w", err)
		}
		ssnBlob = blob
		ssnAudit = "set"
	}

	np, err := q.CreateNaturalPerson(ctx, coredb.CreateNaturalPersonParams{
		EntityID:   entity.ID,
		GivenName:  pgtype.Text{String: in.GivenName, Valid: true},
		FamilyName: pgtype.Text{String: in.FamilyName, Valid: true},
		Ssn:        ssnBlob,
	})
	if err != nil {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("natural_person.Create natural_person: %w", err)
	}

	eid := entity.ID
	_ = s.aw.Write(ctx, "create", "natural_person", &eid, nil, map[string]any{
		"uuid":        entity.Uuid.String(),
		"given_name":  in.GivenName,
		"family_name": in.FamilyName,
		"ssn":         ssnAudit,
	})

	return np, entity.Uuid, nil
}

// GetByEntityUUID resolves the entity by UUID and returns its full Profile.
// Returns ErrNotFound if the entity does not exist.
// The cipher stored on the service is forwarded to ResolveProfileByEntityID so
// that TaxID/TaxIDType are always populated when the cipher is configured.
func (s *NaturalPersonService) GetByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error) {
	entity, err := q.GetEntityByUUID(ctx, entityUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Profile{}, ErrNotFound
		}
		return Profile{}, fmt.Errorf("natural_person.GetByEntityUUID entity: %w", err)
	}

	profile, err := ResolveProfileByEntityID(ctx, q, entity.ID, s.cipher)
	if err != nil {
		return Profile{}, fmt.Errorf("natural_person.GetByEntityUUID profile: %w", err)
	}
	return profile, nil
}

// UpdateByEntityUUID updates natural_person fields for the given entity UUID.
// Non-admin callers may only update their own entity; admins may update any.
// Nil fields in the input are left unchanged.
func (s *NaturalPersonService) UpdateByEntityUUID(
	ctx context.Context,
	q coredb.Querier,
	entityUUID uuid.UUID,
	in UpdateNaturalPersonInput,
	actor Principal,
) error {
	entity, err := q.GetEntityByUUID(ctx, entityUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("natural_person.UpdateByEntityUUID entity: %w", err)
	}

	if !actor.IsAdmin && actor.EntityID != entity.ID {
		return ErrForbidden
	}

	np, err := q.GetNaturalPersonByEntityID(ctx, entity.ID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("natural_person.UpdateByEntityUUID natural_person: %w", err)
	}

	before := map[string]any{
		"given_name":  np.GivenName.String,
		"family_name": np.FamilyName.String,
		"ssn":         "unchanged",
	}

	gn := np.GivenName
	fn := np.FamilyName
	if in.GivenName != nil {
		gn = pgtype.Text{String: strings.TrimSpace(*in.GivenName), Valid: true}
	}
	if in.FamilyName != nil {
		fn = pgtype.Text{String: strings.TrimSpace(*in.FamilyName), Valid: true}
	}

	// ssnParam: nil = leave unchanged (COALESCE keeps DB value); []byte{} = clear; non-empty = set.
	var ssnParam []byte
	ssnAudit := "unchanged"
	if in.SSN != nil {
		val := strings.TrimSpace(*in.SSN)
		if val == "" {
			ssnParam = []byte{} // clear
			ssnAudit = "cleared"
		} else {
			b, err := s.cipher.Encrypt(val)
			if err != nil {
				return fmt.Errorf("encrypt ssn: %w", err)
			}
			ssnParam = b
			ssnAudit = "set"
		}
	}

	if err := q.UpdateNaturalPerson(ctx, coredb.UpdateNaturalPersonParams{
		EntityID:   entity.ID,
		GivenName:  gn,
		FamilyName: fn,
		Ssn:        ssnParam,
	}); err != nil {
		return fmt.Errorf("natural_person.UpdateByEntityUUID update: %w", err)
	}

	after := map[string]any{
		"given_name":  gn.String,
		"family_name": fn.String,
		"ssn":         ssnAudit,
	}

	eid := entity.ID
	_ = s.aw.Write(ctx, "update", "natural_person", &eid, before, after)
	return nil
}

// GetDecryptedSSN returns the plaintext SSN for the given entity.
// Returns "" if not set. Returns an error only on decrypt failure
// (i.e. stored blob is corrupt or the key is wrong) — not for NULL.
func (s *NaturalPersonService) GetDecryptedSSN(ctx context.Context, q coredb.Querier, entityID int64) (string, error) {
	np, err := q.GetNaturalPersonByEntityID(ctx, entityID)
	if err != nil {
		return "", fmt.Errorf("natural_person.GetDecryptedSSN: %w", err)
	}
	return s.cipher.Decrypt(np.Ssn)
}
