package httpapi

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// --- fake PrincipalExtractor ---

type fakePrincipalExtractor struct {
	p  *service.Principal
	ok bool
}

func (f *fakePrincipalExtractor) FromContext(_ context.Context) (*service.Principal, bool) {
	return f.p, f.ok
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

func (f *fakeEntityService) GetSelf(_ context.Context, _ coredb.Querier, _ service.Principal) (service.Profile, error) {
	return f.profile, f.err
}

func (f *fakeEntityService) Archive(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.Principal) error {
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

func (f *fakeNaturalPersonService) Create(_ context.Context, _ coredb.Querier, _ service.Principal, _ service.CreateNaturalPersonInput) (coredb.CreateNaturalPersonRow, uuid.UUID, error) {
	return f.createNP, f.createUUID, f.err
}

func (f *fakeNaturalPersonService) GetByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID) (service.Profile, error) {
	return f.profile, f.err
}

func (f *fakeNaturalPersonService) UpdateByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.UpdateNaturalPersonInput, _ service.Principal) error {
	return f.err
}

var _ service.NaturalPersonServicer = (*fakeNaturalPersonService)(nil)

type fakeCorporationService struct {
	profile    service.Profile
	createCorp coredb.CreateCorporationRow
	createUUID uuid.UUID
	err        error
}

func (f *fakeCorporationService) Create(_ context.Context, _ coredb.Querier, _ service.Principal, _ service.CreateCorporationInput) (coredb.CreateCorporationRow, uuid.UUID, error) {
	return f.createCorp, f.createUUID, f.err
}

func (f *fakeCorporationService) GetByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID) (service.Profile, error) {
	return f.profile, f.err
}

func (f *fakeCorporationService) UpdateByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.UpdateCorporationInput, _ service.Principal) error {
	return f.err
}

var _ service.CorporationServicer = (*fakeCorporationService)(nil)

type fakeServiceAccountService struct {
	profile    service.Profile
	createSA   coredb.ServiceAccount
	createUUID uuid.UUID
	err        error
}

func (f *fakeServiceAccountService) Create(_ context.Context, _ coredb.Querier, _ service.Principal, _ service.CreateServiceAccountInput) (coredb.ServiceAccount, uuid.UUID, error) {
	return f.createSA, f.createUUID, f.err
}

func (f *fakeServiceAccountService) GetByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID) (service.Profile, error) {
	return f.profile, f.err
}

func (f *fakeServiceAccountService) UpdateByEntityUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.UpdateServiceAccountInput, _ service.Principal) error {
	return f.err
}

var _ service.ServiceAccountServicer = (*fakeServiceAccountService)(nil)

// buildTestDeps constructs a Deps with the given service overrides.
func buildTestDeps(
	principal *fakePrincipalExtractor,
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
		Services:  svcs,
		Principal: principal,
		Logger:    noopLogger(),
	}
}

// buildTestDepsWithTx is an alias for buildTestDeps. The tx parameter is
// accepted but ignored: handlers delegate all transactions to the service
// layer, so handler tests never exercise a tx path.
func buildTestDepsWithTx(
	principal *fakePrincipalExtractor,
	entity *fakeEntityService,
	np *fakeNaturalPersonService,
	corp *fakeCorporationService,
	sa *fakeServiceAccountService,
	_ *fakeTxBeginner,
) Deps {
	return buildTestDeps(principal, entity, np, corp, sa)
}

// noopLogger returns a slog.Logger that discards all output.
func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}
