package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/moduleforge/core-api/internal/fieldcrypto"
	coredb "github.com/moduleforge/core-model/db"
)

// Profile is a composite view of any entity with its sub-type data populated.
// Entity holds the entity row together with its resolved fundamental_type_slug.
// Kind is populated from Entity.FundamentalTypeSlug for convenience.
//
// TaxID and TaxIDType are populated (plaintext) when a cipher is available.
// The http layer is responsible for stripping them from responses when the
// caller lacks authorization to view them.
type Profile struct {
	Entity         coredb.GetEntityByUUIDRow
	Kind           string // "natural_person" | "corporation" | "service_account"
	NaturalPerson  *coredb.GetNaturalPersonByEntityIDRow
	Corporation    *coredb.GetCorporationByEntityIDRow
	ServiceAccount *coredb.ServiceAccount
	TaxID          string // plaintext; "" if none or not decrypted
	TaxIDType      string // "SSN" | "EIN" | ""
}

// ResolveProfileByEntityID loads an entity's sub-type records from the database
// and returns a populated Profile. Dispatches via fundamental_type_slug.
// cipher is optional; when non-nil, TaxID/TaxIDType are populated from the
// decrypted field. When nil they remain empty strings.
func ResolveProfileByEntityID(ctx context.Context, q coredb.Querier, entityID int64, cipher ...*fieldcrypto.Cipher) (Profile, error) {
	entity, err := q.GetEntityByID(ctx, entityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrNotFound
		}
		return Profile{}, fmt.Errorf("resolve profile: entity %d: %w", entityID, err)
	}

	profile := Profile{
		Entity: coredb.GetEntityByUUIDRow{
			ID:                  entity.ID,
			Uuid:                entity.Uuid,
			FundamentalTypeID:   entity.FundamentalTypeID,
			FundamentalTypeSlug: entity.FundamentalTypeSlug,
			CreatedAt:           entity.CreatedAt,
			UpdatedAt:           entity.UpdatedAt,
			ArchivedAt:          entity.ArchivedAt,
		},
		Kind: entity.FundamentalTypeSlug,
	}

	// Resolve optional cipher (variadic, at most one element).
	var c *fieldcrypto.Cipher
	if len(cipher) > 0 {
		c = cipher[0]
	}

	switch entity.FundamentalTypeSlug {
	case "natural_person":
		np, err := q.GetNaturalPersonByEntityID(ctx, entity.ID)
		if err != nil {
			return Profile{}, fmt.Errorf("resolve profile natural_person: %w", err)
		}
		profile.NaturalPerson = &np
		if c != nil {
			val, err := c.Decrypt(np.Ssn)
			if err != nil {
				return Profile{}, fmt.Errorf("resolve profile decrypt ssn: %w", err)
			}
			profile.TaxID = val
			profile.TaxIDType = "SSN"
		}

	case "corporation":
		corp, err := q.GetCorporationByEntityID(ctx, entity.ID)
		if err != nil {
			return Profile{}, fmt.Errorf("resolve profile corporation: %w", err)
		}
		profile.Corporation = &corp
		if c != nil {
			val, err := c.Decrypt(corp.Ein)
			if err != nil {
				return Profile{}, fmt.Errorf("resolve profile decrypt ein: %w", err)
			}
			profile.TaxID = val
			profile.TaxIDType = "EIN"
		}

	case "service_account":
		sa, err := q.GetServiceAccountByEntityID(ctx, entity.ID)
		if err != nil {
			return Profile{}, fmt.Errorf("resolve profile service_account: %w", err)
		}
		profile.ServiceAccount = &sa

	default:
		return Profile{}, fmt.Errorf("resolve profile: unknown fundamental_type_slug %q", entity.FundamentalTypeSlug)
	}

	return profile, nil
}
