package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/audit"
	coredb "github.com/moduleforge/core-model/db"
)

// CreateNaturalPersonInput carries the fields required to create a natural person.
type CreateNaturalPersonInput struct {
	GivenName  string
	FamilyName string
	// DisplayName is no longer stored; display is derived via the display registry.
}

// UpdateNaturalPersonInput carries the fields that may be updated on a natural person.
// Nil fields are left unchanged.
type UpdateNaturalPersonInput struct {
	GivenName  *string
	FamilyName *string
}

// NaturalPersonServicer defines natural person operations available to httpapi handlers.
type NaturalPersonServicer interface {
	Create(ctx context.Context, q coredb.Querier, actor Principal, in CreateNaturalPersonInput) (coredb.NaturalPerson, uuid.UUID, error)
	GetByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error)
	UpdateByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID, in UpdateNaturalPersonInput, actor Principal) error
}

// NaturalPersonService implements natural person CRUD with audit logging.
type NaturalPersonService struct {
	aw audit.Writer
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

	np, err := q.CreateNaturalPerson(ctx, coredb.CreateNaturalPersonParams{
		EntityID:   entity.ID,
		GivenName:  pgtype.Text{String: in.GivenName, Valid: true},
		FamilyName: pgtype.Text{String: in.FamilyName, Valid: true},
	})
	if err != nil {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("natural_person.Create natural_person: %w", err)
	}

	eid := entity.ID
	_ = s.aw.Write(ctx, "create", "natural_person", &eid, nil, map[string]any{
		"uuid":        entity.Uuid.String(),
		"given_name":  in.GivenName,
		"family_name": in.FamilyName,
	})

	return np, entity.Uuid, nil
}

// GetByEntityUUID resolves the entity by UUID and returns its full Profile.
// Returns ErrNotFound if the entity does not exist.
func (s *NaturalPersonService) GetByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error) {
	entity, err := q.GetEntityByUUID(ctx, entityUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Profile{}, ErrNotFound
		}
		return Profile{}, fmt.Errorf("natural_person.GetByEntityUUID entity: %w", err)
	}

	profile, err := ResolveProfileByEntityID(ctx, q, entity.ID)
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
	}

	gn := np.GivenName
	fn := np.FamilyName
	if in.GivenName != nil {
		gn = pgtype.Text{String: strings.TrimSpace(*in.GivenName), Valid: true}
	}
	if in.FamilyName != nil {
		fn = pgtype.Text{String: strings.TrimSpace(*in.FamilyName), Valid: true}
	}

	if err := q.UpdateNaturalPerson(ctx, coredb.UpdateNaturalPersonParams{
		EntityID:   entity.ID,
		GivenName:  gn,
		FamilyName: fn,
	}); err != nil {
		return fmt.Errorf("natural_person.UpdateByEntityUUID update: %w", err)
	}

	after := map[string]any{
		"given_name":  gn.String,
		"family_name": fn.String,
	}

	eid := entity.ID
	_ = s.aw.Write(ctx, "update", "natural_person", &eid, before, after)
	return nil
}
