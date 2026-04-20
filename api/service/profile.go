package service

import (
	"context"
	"fmt"

	coredb "github.com/moduleforge/core-model/db"
)

// Profile is a composite view of any entity with its sub-type data populated.
type Profile struct {
	Entity         coredb.Entity
	Kind           string // "natural_person" | "corporation" | "service_account"
	NaturalPerson  *coredb.NaturalPerson
	Corporation    *coredb.Corporation
	ServiceAccount *coredb.ServiceAccount
}

// ResolveProfileByEntityID loads an entity's sub-type records from the database
// and returns a populated Profile. This is the logic extracted from
// users-module's buildEntityInfo helper.
func ResolveProfileByEntityID(ctx context.Context, q coredb.Querier, entityID int64) (Profile, error) {
	// Try legal_entity path first (natural_person + corporation).
	le, err := q.GetLegalEntityByEntityID(ctx, entityID)
	if err != nil {
		// Fall through to service_account.
		sa, saErr := q.GetServiceAccountByEntityID(ctx, entityID)
		if saErr != nil {
			return Profile{}, fmt.Errorf("resolve profile: %w", ErrNotFound)
		}
		return Profile{
			Kind:           "service_account",
			ServiceAccount: &sa,
		}, nil
	}

	profile := Profile{Kind: le.Kind}

	switch le.Kind {
	case "natural_person":
		np, err := q.GetNaturalPersonByLegalEntityID(ctx, le.ID)
		if err != nil {
			return Profile{}, fmt.Errorf("resolve profile natural_person: %w", err)
		}
		profile.NaturalPerson = &np
	case "corporation":
		corp, err := q.GetCorporationByLegalEntityID(ctx, le.ID)
		if err != nil {
			return Profile{}, fmt.Errorf("resolve profile corporation: %w", err)
		}
		profile.Corporation = &corp
	default:
		return Profile{}, fmt.Errorf("resolve profile: unknown legal_entity kind %q", le.Kind)
	}

	return profile, nil
}
