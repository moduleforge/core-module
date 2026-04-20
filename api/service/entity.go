package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/audit"
	coredb "github.com/moduleforge/core-model/db"
)

// EntityServicer defines the entity operations available to httpapi handlers.
type EntityServicer interface {
	GetByUUID(ctx context.Context, q coredb.Querier, id uuid.UUID) (coredb.GetEntityByUUIDRow, error)
	GetByID(ctx context.Context, q coredb.Querier, id int64) (coredb.GetEntityByIDRow, error)
	GetSelf(ctx context.Context, q coredb.Querier, actor Principal) (Profile, error)
	Archive(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID, actor Principal) error
	ResolveProfile(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error)
}

// EntityService implements entity-level operations.
type EntityService struct {
	aw audit.Writer
}

// Compile-time assertion.
var _ EntityServicer = (*EntityService)(nil)

// GetByUUID fetches a single entity by its public UUID.
// Returns ErrNotFound if no matching row exists.
func (s *EntityService) GetByUUID(ctx context.Context, q coredb.Querier, id uuid.UUID) (coredb.GetEntityByUUIDRow, error) {
	e, err := q.GetEntityByUUID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return coredb.GetEntityByUUIDRow{}, ErrNotFound
		}
		return coredb.GetEntityByUUIDRow{}, fmt.Errorf("entity.GetByUUID: %w", err)
	}
	return e, nil
}

// GetByID fetches a single entity by internal ID.
// Returns ErrNotFound if no matching row exists.
func (s *EntityService) GetByID(ctx context.Context, q coredb.Querier, id int64) (coredb.GetEntityByIDRow, error) {
	e, err := q.GetEntityByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return coredb.GetEntityByIDRow{}, ErrNotFound
		}
		return coredb.GetEntityByIDRow{}, fmt.Errorf("entity.GetByID: %w", err)
	}
	return e, nil
}

// GetSelf returns the Profile for the authenticated caller's entity.
func (s *EntityService) GetSelf(ctx context.Context, q coredb.Querier, actor Principal) (Profile, error) {
	profile, err := ResolveProfileByEntityID(ctx, q, actor.EntityID)
	if err != nil {
		return Profile{}, fmt.Errorf("entity.GetSelf: %w", err)
	}
	return profile, nil
}

// ResolveProfile loads an entity by UUID then resolves its sub-type profile.
func (s *EntityService) ResolveProfile(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID) (Profile, error) {
	entity, err := s.GetByUUID(ctx, q, entityUUID)
	if err != nil {
		return Profile{}, err
	}
	profile, err := ResolveProfileByEntityID(ctx, q, entity.ID)
	if err != nil {
		return Profile{}, err
	}
	return profile, nil
}

// Archive soft-deletes an entity. Requires admin.
func (s *EntityService) Archive(ctx context.Context, q coredb.Querier, entityUUID uuid.UUID, actor Principal) error {
	if !actor.IsAdmin {
		return ErrForbidden
	}

	entity, err := s.GetByUUID(ctx, q, entityUUID)
	if err != nil {
		return err
	}

	if err := q.ArchiveEntity(ctx, entityUUID); err != nil {
		return fmt.Errorf("entity.Archive: %w", err)
	}

	eid := entity.ID
	_ = s.aw.Write(ctx, "delete", "entity", &eid, map[string]any{"uuid": entityUUID.String()}, nil)
	return nil
}
