package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	coredb "github.com/moduleforge/core-model/db"
)

// Profile is a composite view of any entity with its sub-type data populated.
// Entity holds the entity row together with its resolved fundamental_type_slug.
// Kind is populated from Entity.FundamentalTypeSlug for convenience.
type Profile struct {
	Entity         coredb.GetEntityByUUIDRow
	Kind           string // "natural_person" | "corporation" | "service_account"
	NaturalPerson  *coredb.NaturalPerson
	Corporation    *coredb.Corporation
	ServiceAccount *coredb.ServiceAccount
}

// ResolveProfileByEntityID loads an entity's sub-type records from the database
// and returns a populated Profile. Dispatches via fundamental_type_slug.
func ResolveProfileByEntityID(ctx context.Context, q coredb.Querier, entityID int64) (Profile, error) {
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

	switch entity.FundamentalTypeSlug {
	case "natural_person":
		np, err := q.GetNaturalPersonByEntityID(ctx, entity.ID)
		if err != nil {
			return Profile{}, fmt.Errorf("resolve profile natural_person: %w", err)
		}
		profile.NaturalPerson = &np

	case "corporation":
		corp, err := q.GetCorporationByEntityID(ctx, entity.ID)
		if err != nil {
			return Profile{}, fmt.Errorf("resolve profile corporation: %w", err)
		}
		profile.Corporation = &corp

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
