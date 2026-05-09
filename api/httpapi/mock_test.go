package httpapi

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// actorCtx returns a context with the given actor entity ID set.
// Use entityID=0 to simulate an unauthenticated request (opctx stores int64;
// 0 is never a valid DB entity ID, but FromContext returns (0, false) for
// missing keys, so we model unauthenticated by omitting the actor entirely).
func actorCtx(entityID int64) context.Context {
	return opctx.WithActor(context.Background(), entityID)
}

// --- fake service implementations ---

type fakeEntityService struct {
	profile service.Profile
	err     error
}

func (f *fakeEntityService) GetByUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID) (coredb.GetEntityByUUIDRow, error) {
	return f.profile.Entity, f.err
}

func (f *fakeEntityService) GetByID(_ context.Context, _ coredb.Querier, _ int64) (coredb.GetEntityByIDRow, error) {
	row := coredb.GetEntityByIDRow{
		ID:                  f.profile.Entity.ID,
		Uuid:                f.profile.Entity.Uuid,
		FundamentalTypeID:   f.profile.Entity.FundamentalTypeID,
		FundamentalTypeSlug: f.profile.Entity.FundamentalTypeSlug,
		CreatedAt:           f.profile.Entity.CreatedAt,
		UpdatedAt:           f.profile.Entity.UpdatedAt,
		ArchivedAt:          f.profile.Entity.ArchivedAt,
	}
	return row, f.err
}

func (f *fakeEntityService) GetSelf(_ context.Context, _ coredb.Querier) (service.Profile, error) {
	return f.profile, f.err
}

func (f *fakeEntityService) Archive(_ context.Context, _ coredb.Querier, _ uuid.UUID) error {
	return f.err
}

func (f *fakeEntityService) ResolveProfile(_ context.Context, _ coredb.Querier, _ uuid.UUID) (service.Profile, error) {
	return f.profile, f.err
}

var _ service.EntityServicer = (*fakeEntityService)(nil)

type fakeNaturalPersonService struct {
	profile    service.Profile
	createNP   coredb.CreateNaturalPersonRow
	createUUID uuid.UUID
	err        error
}

func (f *fakeNaturalPersonService) Create(_ context.Context, _ coredb.Querier, _ service.CreateNaturalPersonInput) (coredb.CreateNaturalPersonRow, uuid.UUID, error) {
	return f.createNP, f.createUUID, f.err
}

func (f *fakeNaturalPersonService) CreateInTx(_ context.Context, _ pgx.Tx, _ service.CreateNaturalPersonInput) (coredb.CreateNaturalPersonRow, uuid.UUID, int64, error) {
	return f.createNP, f.createUUID, 0, f.err
}

func (f *fakeNaturalPersonService) GetByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID) (service.Profile, error) {
	return f.profile, f.err
}

func (f *fakeNaturalPersonService) UpdateByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.UpdateNaturalPersonInput) error {
	return f.err
}

var _ service.NaturalPersonServicer = (*fakeNaturalPersonService)(nil)

type fakeCorporationService struct {
	profile    service.Profile
	createCorp coredb.CreateCorporationRow
	createUUID uuid.UUID
	err        error
}

func (f *fakeCorporationService) Create(_ context.Context, _ coredb.Querier, _ service.CreateCorporationInput) (coredb.CreateCorporationRow, uuid.UUID, error) {
	return f.createCorp, f.createUUID, f.err
}

func (f *fakeCorporationService) GetByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID) (service.Profile, error) {
	return f.profile, f.err
}

func (f *fakeCorporationService) UpdateByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.UpdateCorporationInput) error {
	return f.err
}

var _ service.CorporationServicer = (*fakeCorporationService)(nil)

type fakeServiceAccountService struct {
	profile    service.Profile
	createSA   coredb.ServiceAccount
	createUUID uuid.UUID
	err        error
}

func (f *fakeServiceAccountService) Create(_ context.Context, _ coredb.Querier, _ service.CreateServiceAccountInput) (coredb.ServiceAccount, uuid.UUID, error) {
	return f.createSA, f.createUUID, f.err
}

func (f *fakeServiceAccountService) GetByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID) (service.Profile, error) {
	return f.profile, f.err
}

func (f *fakeServiceAccountService) UpdateByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.UpdateServiceAccountInput) error {
	return f.err
}

var _ service.ServiceAccountServicer = (*fakeServiceAccountService)(nil)

// buildTestDeps constructs a Deps with the given service overrides.
func buildTestDeps(
	entity *fakeEntityService,
	np *fakeNaturalPersonService,
	corp *fakeCorporationService,
	sa *fakeServiceAccountService,
) Deps {
	svcs := &service.Services{}
	if entity != nil {
		svcs.Entity = entity
	}
	if np != nil {
		svcs.NaturalPerson = np
	}
	if corp != nil {
		svcs.Corporation = corp
	}
	if sa != nil {
		svcs.ServiceAccount = sa
	}

	return Deps{
		Services: svcs,
		Logger:   noopLogger(),
	}
}

// noopLogger returns a slog.Logger that discards all output.
func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}
