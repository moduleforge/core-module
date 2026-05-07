package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/authz"
	"github.com/moduleforge/core-api/internal/fieldcrypto"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/txhelper"
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

// NaturalPersonService implements natural person CRUD with authorization,
// transactional mutation, and observer dispatch.
type NaturalPersonService struct {
	db         txhelper.DB
	az         authz.Authorizer
	obs        *observer.ObserverGroup
	cipher     *fieldcrypto.Cipher
	newQuerier func(pgx.Tx) coredb.Querier // injectable for tests; defaults to coredb.New
}

// Compile-time assertion.
var _ NaturalPersonServicer = (*NaturalPersonService)(nil)

func (s *NaturalPersonService) querier(tx pgx.Tx) coredb.Querier {
	if s.newQuerier != nil {
		return s.newQuerier(tx)
	}
	return coredb.New(tx)
}

// Create inserts entity → legal_entity → natural_person rows atomically inside
// a transaction. The caller-supplied q parameter is accepted for interface
// compatibility but the service opens its own transaction via s.db.
// Returns the new record and the entity's public UUID. Requires admin authorization.
func (s *NaturalPersonService) Create(
	ctx context.Context,
	_ coredb.Querier,
	_ Principal,
	in CreateNaturalPersonInput,
) (coredb.NaturalPerson, uuid.UUID, error) {
	// 1. Authorize.
	if err := s.az.Authorize(ctx, "create", nil); err != nil {
		return coredb.NaturalPerson{}, uuid.UUID{}, err
	}

	in.GivenName = strings.TrimSpace(in.GivenName)
	in.FamilyName = strings.TrimSpace(in.FamilyName)
	if in.GivenName == "" {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("%w: given_name is required", ErrInvalidInput)
	}
	if in.FamilyName == "" {
		return coredb.NaturalPerson{}, uuid.UUID{}, fmt.Errorf("%w: family_name is required", ErrInvalidInput)
	}

	// 2. Mutate inside a transaction; observers participate in the same tx.
	var (
		np         coredb.NaturalPerson
		entityUUID uuid.UUID
		entityID   int64
	)

	err := txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
		q := s.querier(tx)

		// Resolve the type ID for 'natural_person' from the registry.
		t, err := q.GetTypeBySlug(ctx, "natural_person")
		if err != nil {
			return fmt.Errorf("natural_person.Create resolve type: %w", err)
		}

		ent, err := q.CreateEntity(ctx, t.ID)
		if err != nil {
			return fmt.Errorf("natural_person.Create entity: %w", err)
		}

		_, err = q.CreateLegalEntity(ctx, ent.ID)
		if err != nil {
			return fmt.Errorf("natural_person.Create legal_entity: %w", err)
		}

		var ssnBlob []byte
		ssnAudit := "unchanged"
		if strings.TrimSpace(in.SSN) != "" {
			blob, err := s.cipher.Encrypt(strings.TrimSpace(in.SSN))
			if err != nil {
				return fmt.Errorf("encrypt ssn: %w", err)
			}
			ssnBlob = blob
			ssnAudit = "set"
		}

		np, err = q.CreateNaturalPerson(ctx, coredb.CreateNaturalPersonParams{
			EntityID:   ent.ID,
			GivenName:  pgtype.Text{String: in.GivenName, Valid: true},
			FamilyName: pgtype.Text{String: in.FamilyName, Valid: true},
			Ssn:        ssnBlob,
		})
		if err != nil {
			return fmt.Errorf("natural_person.Create natural_person: %w", err)
		}

		entityUUID = ent.Uuid
		entityID = ent.ID

		after := map[string]any{
			"uuid":        ent.Uuid.String(),
			"given_name":  in.GivenName,
			"family_name": in.FamilyName,
			"ssn":         ssnAudit,
		}
		return s.obs.Observe(ctx, tx, "create", "natural_person", &entityID, nil, after)
	})
	if err != nil {
		return coredb.NaturalPerson{}, uuid.UUID{}, err
	}

	// 3. Post-commit observers.
	after := map[string]any{
		"uuid":        entityUUID.String(),
		"given_name":  in.GivenName,
		"family_name": in.FamilyName,
	}
	s.obs.ObserveAfterCommit(ctx, "create", "natural_person", &entityID, nil, after)

	return np, entityUUID, nil
}

// GetByEntityUUID resolves the entity by UUID and returns its full Profile.
// Returns ErrNotFound if the entity does not exist.
// The cipher stored on the service is forwarded to ResolveProfileByEntityID so
// that TaxID/TaxIDType are always populated when the cipher is configured.
func (s *NaturalPersonService) GetByEntityUUID(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error) {
	// 1. Authorize. UUID has not been resolved to an internal ID yet.
	if err := s.az.Authorize(ctx, "read", nil); err != nil {
		return Profile{}, err
	}

	ent, err := q.GetEntityByUUID(ctx, entityUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrNotFound
		}
		return Profile{}, fmt.Errorf("natural_person.GetByEntityUUID entity: %w", err)
	}

	profile, err := ResolveProfileByEntityID(ctx, q, ent.ID, s.cipher)
	if err != nil {
		return Profile{}, fmt.Errorf("natural_person.GetByEntityUUID profile: %w", err)
	}
	return profile, nil
}

// UpdateByEntityUUID updates natural_person fields for the given entity UUID.
// Nil fields in the input are left unchanged.
// The caller-supplied q parameter is used for the pre-tx entity lookup; the
// service opens its own transaction via s.db for the mutation.
func (s *NaturalPersonService) UpdateByEntityUUID(
	ctx context.Context,
	q coredb.Querier,
	entityUUID uuid.UUID,
	in UpdateNaturalPersonInput,
	_ Principal,
) error {
	// 1. Authorize — fetch entity first to build a richer target.
	ent, err := q.GetEntityByUUID(ctx, entityUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("natural_person.UpdateByEntityUUID entity: %w", err)
	}

	eid := ent.ID
	if err := s.az.Authorize(ctx, "update", &eid); err != nil {
		return err
	}

	// 2. Mutate inside a transaction; observers participate in the same tx.
	err = txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
		txQ := s.querier(tx)

		np, err := txQ.GetNaturalPersonByEntityID(ctx, ent.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
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

		if err := txQ.UpdateNaturalPerson(ctx, coredb.UpdateNaturalPersonParams{
			EntityID:   ent.ID,
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

		return s.obs.Observe(ctx, tx, "update", "natural_person", &eid, before, after)
	})
	if err != nil {
		return err
	}

	// 3. Post-commit observers.
	s.obs.ObserveAfterCommit(ctx, "update", "natural_person", &eid, nil, nil)
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
