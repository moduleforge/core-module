package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
	coredb "github.com/moduleforge/core-model/db"
)

// testCipher returns a deterministic Cipher suitable for unit tests.
// Uses a 32-byte zero key — never use in production.
func testCipher(t *testing.T) *fieldcrypto.Cipher {
	t.Helper()
	c, err := fieldcrypto.NewFromKey(make([]byte, 32))
	if err != nil {
		t.Fatalf("testCipher: %v", err)
	}
	return c
}

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
	entities       map[uuid.UUID]coredb.GetEntityByUUIDRow
	entitiesByID   map[int64]coredb.GetEntityByUUIDRow
	legalEntities  map[int64]int64 // entity_id -> entity_id (anchor)
	naturalPersons map[int64]coredb.NaturalPerson
	corporations   map[int64]coredb.Corporation
	serviceAccts   map[int64]coredb.ServiceAccount
	types          map[string]coredb.Type

	createEntityFn        func(ctx context.Context, fundamentalTypeID int64) (coredb.Entity, error)
	createNaturalPersonFn func(ctx context.Context, arg coredb.CreateNaturalPersonParams) (coredb.NaturalPerson, error)
	updateNaturalPersonErr error
	archiveEntityErr       error
	nextID                 int64
}

func newMockQuerier() *mockQuerier {
	m := &mockQuerier{
		entities:       make(map[uuid.UUID]coredb.GetEntityByUUIDRow),
		entitiesByID:   make(map[int64]coredb.GetEntityByUUIDRow),
		legalEntities:  make(map[int64]int64),
		naturalPersons: make(map[int64]coredb.NaturalPerson),
		corporations:   make(map[int64]coredb.Corporation),
		serviceAccts:   make(map[int64]coredb.ServiceAccount),
		types:          make(map[string]coredb.Type),
	}
	// Pre-seed core types so service Create calls work.
	m.seedTypes()
	return m
}

func (m *mockQuerier) seedTypes() {
	typeRows := []struct {
		id       int64
		slug     string
		parentID pgtype.Int8
		concrete bool
	}{
		{1, "entity", pgtype.Int8{Valid: false}, false},
		{2, "legal_entity", pgtype.Int8{Int64: 1, Valid: true}, false},
		{3, "natural_person", pgtype.Int8{Int64: 2, Valid: true}, true},
		{4, "corporation", pgtype.Int8{Int64: 2, Valid: true}, true},
		{5, "service_account", pgtype.Int8{Int64: 1, Valid: true}, true},
	}
	for _, tr := range typeRows {
		m.types[tr.slug] = coredb.Type{
			ID:       tr.id,
			Slug:     tr.slug,
			ParentID: tr.parentID,
			Concrete: tr.concrete,
			Name:     tr.slug,
		}
	}
}

func (m *mockQuerier) nextSeq() int64 {
	m.nextID++
	return m.nextID
}

// slugForTypeID returns the slug for a seeded type ID.
func (m *mockQuerier) slugForTypeID(id int64) string {
	for slug, t := range m.types {
		if t.ID == id {
			return slug
		}
	}
	return "unknown"
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
		ID:           id,
		EntityID:     arg.EntityID,
		LegalName:    arg.LegalName,
		Jurisdiction: arg.Jurisdiction,
		Ein:          arg.Ein,
	}
	m.corporations[arg.EntityID] = corp
	return corp, nil
}

func (m *mockQuerier) CreateEntity(_ context.Context, fundamentalTypeID int64) (coredb.Entity, error) {
	if m.createEntityFn != nil {
		return m.createEntityFn(context.Background(), fundamentalTypeID)
	}
	id := m.nextSeq()
	slug := m.slugForTypeID(fundamentalTypeID)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	row := coredb.GetEntityByUUIDRow{
		ID:                  id,
		Uuid:                uuid.New(),
		FundamentalTypeID:   fundamentalTypeID,
		FundamentalTypeSlug: slug,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	m.entities[row.Uuid] = row
	m.entitiesByID[id] = row
	return coredb.Entity{
		ID:                id,
		Uuid:              row.Uuid,
		FundamentalTypeID: fundamentalTypeID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (m *mockQuerier) CreateLegalEntity(_ context.Context, entityID int64) (int64, error) {
	m.legalEntities[entityID] = entityID
	return entityID, nil
}

func (m *mockQuerier) CreateNaturalPerson(_ context.Context, arg coredb.CreateNaturalPersonParams) (coredb.NaturalPerson, error) {
	if m.createNaturalPersonFn != nil {
		return m.createNaturalPersonFn(context.Background(), arg)
	}
	id := m.nextSeq()
	np := coredb.NaturalPerson{
		ID:         id,
		EntityID:   arg.EntityID,
		GivenName:  arg.GivenName,
		FamilyName: arg.FamilyName,
		Ssn:        arg.Ssn,
	}
	m.naturalPersons[arg.EntityID] = np
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

func (m *mockQuerier) GetCorporationByEntityID(_ context.Context, entityID int64) (coredb.Corporation, error) {
	if corp, ok := m.corporations[entityID]; ok {
		return corp, nil
	}
	return coredb.Corporation{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetEntityByUUID(_ context.Context, argUuid uuid.UUID) (coredb.GetEntityByUUIDRow, error) {
	if e, ok := m.entities[argUuid]; ok {
		return e, nil
	}
	return coredb.GetEntityByUUIDRow{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetEntityByID(_ context.Context, id int64) (coredb.GetEntityByIDRow, error) {
	if e, ok := m.entitiesByID[id]; ok {
		return coredb.GetEntityByIDRow{
			ID:                  e.ID,
			Uuid:                e.Uuid,
			FundamentalTypeID:   e.FundamentalTypeID,
			FundamentalTypeSlug: e.FundamentalTypeSlug,
			CreatedAt:           e.CreatedAt,
			UpdatedAt:           e.UpdatedAt,
			ArchivedAt:          e.ArchivedAt,
		}, nil
	}
	return coredb.GetEntityByIDRow{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetLegalEntityByEntityID(_ context.Context, entityID int64) (int64, error) {
	if id, ok := m.legalEntities[entityID]; ok {
		return id, nil
	}
	return 0, pgx.ErrNoRows
}

func (m *mockQuerier) GetNaturalPersonByEntityID(_ context.Context, entityID int64) (coredb.NaturalPerson, error) {
	if np, ok := m.naturalPersons[entityID]; ok {
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

func (m *mockQuerier) GetTypeBySlug(_ context.Context, slug string) (coredb.Type, error) {
	if t, ok := m.types[slug]; ok {
		return t, nil
	}
	return coredb.Type{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetTypeByID(_ context.Context, id int64) (coredb.Type, error) {
	for _, t := range m.types {
		if t.ID == id {
			return t, nil
		}
	}
	return coredb.Type{}, pgx.ErrNoRows
}

func (m *mockQuerier) UnarchiveEntity(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) UpdateCorporation(_ context.Context, arg coredb.UpdateCorporationParams) error {
	if corp, ok := m.corporations[arg.EntityID]; ok {
		corp.LegalName = arg.LegalName
		corp.Jurisdiction = arg.Jurisdiction
		// Mirror COALESCE($4, ein): non-nil arg.Ein replaces; nil leaves unchanged.
		if arg.Ein != nil {
			corp.Ein = arg.Ein
		}
		m.corporations[arg.EntityID] = corp
	}
	return nil
}

func (m *mockQuerier) UpdateNaturalPerson(_ context.Context, arg coredb.UpdateNaturalPersonParams) error {
	if m.updateNaturalPersonErr != nil {
		return m.updateNaturalPersonErr
	}
	if np, ok := m.naturalPersons[arg.EntityID]; ok {
		np.GivenName = arg.GivenName
		np.FamilyName = arg.FamilyName
		// Mirror COALESCE($4, ssn): non-nil arg.Ssn replaces; nil leaves unchanged.
		if arg.Ssn != nil {
			np.Ssn = arg.Ssn
		}
		m.naturalPersons[arg.EntityID] = np
	}
	return nil
}

// seedNaturalPerson inserts a fully formed entity → legal_entity → natural_person
// into the mock querier and returns the entity UUID.
func (m *mockQuerier) seedNaturalPerson(givenName, familyName string) uuid.UUID {
	entityID := m.nextSeq()
	entityUUID := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	npTypeID := m.types["natural_person"].ID
	row := coredb.GetEntityByUUIDRow{
		ID:                  entityID,
		Uuid:                entityUUID,
		FundamentalTypeID:   npTypeID,
		FundamentalTypeSlug: "natural_person",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	m.entities[entityUUID] = row
	m.entitiesByID[entityID] = row

	m.legalEntities[entityID] = entityID

	npID := m.nextSeq()
	m.naturalPersons[entityID] = coredb.NaturalPerson{
		ID:         npID,
		EntityID:   entityID,
		GivenName:  pgtype.Text{String: givenName, Valid: true},
		FamilyName: pgtype.Text{String: familyName, Valid: true},
	}

	return entityUUID
}

// Compile-time: mockQuerier must satisfy coredb.Querier.
var _ coredb.Querier = (*mockQuerier)(nil)
