package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	coredb "github.com/moduleforge/core-model/db"
)

// --- mock audit.Writer ---

type mockAuditWriter struct {
	calls []auditCall
	err   error
}

type auditCall struct {
	op             string
	resource       string
	targetEntityID *int64
	before         any
	after          any
}

func (m *mockAuditWriter) Write(_ context.Context, op, resource string, targetEntityID *int64, before, after any) error {
	m.calls = append(m.calls, auditCall{
		op:             op,
		resource:       resource,
		targetEntityID: targetEntityID,
		before:         before,
		after:          after,
	})
	return m.err
}

// --- mock coredb.Querier ---

type mockQuerier struct {
	entities       map[uuid.UUID]coredb.Entity
	legalEntities  map[int64]coredb.LegalEntity
	naturalPersons map[int64]coredb.NaturalPerson
	corporations   map[int64]coredb.Corporation
	serviceAccts   map[int64]coredb.ServiceAccount

	createEntityFn         func(ctx context.Context, kind string) (coredb.Entity, error)
	createLegalEntityFn    func(ctx context.Context, arg coredb.CreateLegalEntityParams) (coredb.LegalEntity, error)
	createNaturalPersonFn  func(ctx context.Context, arg coredb.CreateNaturalPersonParams) (coredb.NaturalPerson, error)
	updateNaturalPersonErr error
	archiveEntityErr       error
	nextID                 int64
}

func newMockQuerier() *mockQuerier {
	return &mockQuerier{
		entities:       make(map[uuid.UUID]coredb.Entity),
		legalEntities:  make(map[int64]coredb.LegalEntity),
		naturalPersons: make(map[int64]coredb.NaturalPerson),
		corporations:   make(map[int64]coredb.Corporation),
		serviceAccts:   make(map[int64]coredb.ServiceAccount),
	}
}

func (m *mockQuerier) nextSeq() int64 {
	m.nextID++
	return m.nextID
}

func (m *mockQuerier) ArchiveEntity(_ context.Context, argUuid uuid.UUID) error {
	if m.archiveEntityErr != nil {
		return m.archiveEntityErr
	}
	_, ok := m.entities[argUuid]
	if !ok {
		return pgx.ErrNoRows
	}
	delete(m.entities, argUuid)
	return nil
}

func (m *mockQuerier) CreateCorporation(_ context.Context, arg coredb.CreateCorporationParams) (coredb.Corporation, error) {
	id := m.nextSeq()
	corp := coredb.Corporation{
		ID:            id,
		LegalEntityID: arg.LegalEntityID,
		LegalName:     arg.LegalName,
		Jurisdiction:  arg.Jurisdiction,
	}
	m.corporations[arg.LegalEntityID] = corp
	return corp, nil
}

func (m *mockQuerier) CreateEntity(_ context.Context, kind string) (coredb.Entity, error) {
	if m.createEntityFn != nil {
		return m.createEntityFn(context.Background(), kind)
	}
	id := m.nextSeq()
	e := coredb.Entity{
		ID:   id,
		Uuid: uuid.New(),
		Kind: kind,
	}
	m.entities[e.Uuid] = e
	return e, nil
}

func (m *mockQuerier) CreateLegalEntity(_ context.Context, arg coredb.CreateLegalEntityParams) (coredb.LegalEntity, error) {
	if m.createLegalEntityFn != nil {
		return m.createLegalEntityFn(context.Background(), arg)
	}
	id := m.nextSeq()
	le := coredb.LegalEntity{
		ID:          id,
		EntityID:    arg.EntityID,
		Kind:        arg.Kind,
		DisplayName: arg.DisplayName,
	}
	m.legalEntities[arg.EntityID] = le
	return le, nil
}

func (m *mockQuerier) CreateNaturalPerson(_ context.Context, arg coredb.CreateNaturalPersonParams) (coredb.NaturalPerson, error) {
	if m.createNaturalPersonFn != nil {
		return m.createNaturalPersonFn(context.Background(), arg)
	}
	id := m.nextSeq()
	np := coredb.NaturalPerson{
		ID:            id,
		LegalEntityID: arg.LegalEntityID,
		GivenName:     arg.GivenName,
		FamilyName:    arg.FamilyName,
	}
	m.naturalPersons[arg.LegalEntityID] = np
	return np, nil
}

func (m *mockQuerier) CreateServiceAccount(_ context.Context, arg coredb.CreateServiceAccountParams) (coredb.ServiceAccount, error) {
	id := m.nextSeq()
	sa := coredb.ServiceAccount{
		ID:       id,
		EntityID: arg.EntityID,
		Label:    arg.Label,
	}
	m.serviceAccts[arg.EntityID] = sa
	return sa, nil
}

func (m *mockQuerier) GetCorporationByLegalEntityID(_ context.Context, legalEntityID int64) (coredb.Corporation, error) {
	if corp, ok := m.corporations[legalEntityID]; ok {
		return corp, nil
	}
	return coredb.Corporation{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetEntityByUUID(_ context.Context, argUuid uuid.UUID) (coredb.Entity, error) {
	if e, ok := m.entities[argUuid]; ok {
		return e, nil
	}
	return coredb.Entity{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetLegalEntityByEntityID(_ context.Context, entityID int64) (coredb.LegalEntity, error) {
	if le, ok := m.legalEntities[entityID]; ok {
		return le, nil
	}
	return coredb.LegalEntity{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetNaturalPersonByLegalEntityID(_ context.Context, legalEntityID int64) (coredb.NaturalPerson, error) {
	if np, ok := m.naturalPersons[legalEntityID]; ok {
		return np, nil
	}
	return coredb.NaturalPerson{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetServiceAccountByEntityID(_ context.Context, entityID int64) (coredb.ServiceAccount, error) {
	if sa, ok := m.serviceAccts[entityID]; ok {
		return sa, nil
	}
	return coredb.ServiceAccount{}, pgx.ErrNoRows
}

func (m *mockQuerier) UnarchiveEntity(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) UpdateCorporation(_ context.Context, arg coredb.UpdateCorporationParams) error {
	if corp, ok := m.corporations[arg.LegalEntityID]; ok {
		corp.LegalName = arg.LegalName
		corp.Jurisdiction = arg.Jurisdiction
		m.corporations[arg.LegalEntityID] = corp
	}
	return nil
}

func (m *mockQuerier) UpdateNaturalPerson(_ context.Context, arg coredb.UpdateNaturalPersonParams) error {
	if m.updateNaturalPersonErr != nil {
		return m.updateNaturalPersonErr
	}
	if np, ok := m.naturalPersons[arg.LegalEntityID]; ok {
		np.GivenName = arg.GivenName
		np.FamilyName = arg.FamilyName
		m.naturalPersons[arg.LegalEntityID] = np
	}
	return nil
}

// seedNaturalPerson inserts a fully formed entity → legal_entity → natural_person
// into the mock querier and returns the entity UUID.
func (m *mockQuerier) seedNaturalPerson(givenName, familyName string) uuid.UUID {
	entityID := m.nextSeq()
	entityUUID := uuid.New()
	m.entities[entityUUID] = coredb.Entity{ID: entityID, Uuid: entityUUID, Kind: "legal_entity"}

	leID := m.nextSeq()
	m.legalEntities[entityID] = coredb.LegalEntity{
		ID: leID, EntityID: entityID, Kind: "natural_person",
		DisplayName: givenName + " " + familyName,
	}

	npID := m.nextSeq()
	m.naturalPersons[leID] = coredb.NaturalPerson{
		ID:            npID,
		LegalEntityID: leID,
		GivenName:     pgtype.Text{String: givenName, Valid: true},
		FamilyName:    pgtype.Text{String: familyName, Valid: true},
	}

	return entityUUID
}

// Compile-time: mockQuerier must satisfy coredb.Querier.
var _ coredb.Querier = (*mockQuerier)(nil)
