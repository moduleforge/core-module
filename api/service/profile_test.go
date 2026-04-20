package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	coredb "github.com/moduleforge/core-model/db"
)

func TestResolveProfileByEntityID_NaturalPerson(t *testing.T) {
	q := newMockQuerier()
	entityID := q.nextSeq()
	leID := q.nextSeq()
	q.legalEntities[entityID] = coredb.LegalEntity{ID: leID, EntityID: entityID, Kind: "natural_person"}
	q.naturalPersons[leID] = coredb.NaturalPerson{
		ID:         q.nextSeq(),
		GivenName:  pgtype.Text{String: "Alice", Valid: true},
		FamilyName: pgtype.Text{String: "Smith", Valid: true},
	}

	profile, err := ResolveProfileByEntityID(context.Background(), q, entityID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Kind != "natural_person" {
		t.Errorf("kind: got %q, want natural_person", profile.Kind)
	}
	if profile.NaturalPerson == nil {
		t.Fatal("expected non-nil NaturalPerson")
	}
	if profile.NaturalPerson.GivenName.String != "Alice" {
		t.Errorf("given_name: got %q, want Alice", profile.NaturalPerson.GivenName.String)
	}
}

func TestResolveProfileByEntityID_Corporation(t *testing.T) {
	q := newMockQuerier()
	entityID := q.nextSeq()
	leID := q.nextSeq()
	q.legalEntities[entityID] = coredb.LegalEntity{ID: leID, EntityID: entityID, Kind: "corporation"}
	q.corporations[leID] = coredb.Corporation{
		ID:        q.nextSeq(),
		LegalName: "Acme Corp",
	}

	profile, err := ResolveProfileByEntityID(context.Background(), q, entityID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Kind != "corporation" {
		t.Errorf("kind: got %q, want corporation", profile.Kind)
	}
	if profile.Corporation == nil {
		t.Fatal("expected non-nil Corporation")
	}
}

func TestResolveProfileByEntityID_ServiceAccount(t *testing.T) {
	q := newMockQuerier()
	entityID := q.nextSeq()
	q.serviceAccts[entityID] = coredb.ServiceAccount{
		ID:       q.nextSeq(),
		EntityID: entityID,
		Label:    "test-sa",
	}

	profile, err := ResolveProfileByEntityID(context.Background(), q, entityID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Kind != "service_account" {
		t.Errorf("kind: got %q, want service_account", profile.Kind)
	}
	if profile.ServiceAccount == nil {
		t.Fatal("expected non-nil ServiceAccount")
	}
}

func TestResolveProfileByEntityID_NotFound(t *testing.T) {
	q := newMockQuerier()
	_, err := ResolveProfileByEntityID(context.Background(), q, 99999)
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

func TestNaturalPersonService_GetByEntityUUID_Found(t *testing.T) {
	q := newMockQuerier()
	svc := &NaturalPersonService{aw: &mockAuditWriter{}}

	entityUUID := q.seedNaturalPerson("Bob", "Builder")

	profile, err := svc.GetByEntityUUID(context.Background(), q, entityUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Kind != "natural_person" {
		t.Errorf("kind: got %q, want natural_person", profile.Kind)
	}
	if profile.NaturalPerson.GivenName.String != "Bob" {
		t.Errorf("given_name: got %q, want Bob", profile.NaturalPerson.GivenName.String)
	}
}

func TestNaturalPersonService_GetByEntityUUID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := &NaturalPersonService{aw: &mockAuditWriter{}}

	_, err := svc.GetByEntityUUID(context.Background(), q, uuid.New())
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

func TestEntityService_GetSelf(t *testing.T) {
	q := newMockQuerier()
	svc := &EntityService{aw: &mockAuditWriter{}}

	entityUUID := q.seedNaturalPerson("Carol", "White")
	entity, _ := q.GetEntityByUUID(context.Background(), entityUUID)

	actor := Principal{EntityID: entity.ID}
	profile, err := svc.GetSelf(context.Background(), q, actor)
	if err != nil {
		t.Fatalf("GetSelf: unexpected error: %v", err)
	}
	if profile.Kind != "natural_person" {
		t.Errorf("kind: got %q, want natural_person", profile.Kind)
	}
}

func TestEntityService_ResolveProfile(t *testing.T) {
	q := newMockQuerier()
	svc := &EntityService{aw: &mockAuditWriter{}}

	entityUUID := q.seedNaturalPerson("Dan", "Brown")

	profile, err := svc.ResolveProfile(context.Background(), q, entityUUID)
	if err != nil {
		t.Fatalf("ResolveProfile: unexpected error: %v", err)
	}
	if profile.Entity.Uuid != entityUUID {
		t.Errorf("entity UUID mismatch")
	}
	if profile.Kind != "natural_person" {
		t.Errorf("kind: got %q, want natural_person", profile.Kind)
	}
}

func TestServiceAccountService_Create_WritesAudit(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &ServiceAccountService{aw: aw}
	admin := Principal{IsAdmin: true}

	sa, entityUUID, err := svc.Create(context.Background(), q, admin, CreateServiceAccountInput{Label: "my-service"})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if sa.Label != "my-service" {
		t.Errorf("label: got %q, want my-service", sa.Label)
	}
	if entityUUID.String() == "" {
		t.Error("expected non-empty UUID")
	}
	if len(aw.calls) != 1 {
		t.Fatalf("expected 1 audit call, got %d", len(aw.calls))
	}
}

func TestServiceAccountService_Create_RequiresAdmin(t *testing.T) {
	q := newMockQuerier()
	svc := &ServiceAccountService{aw: &mockAuditWriter{}}
	nonAdmin := Principal{IsAdmin: false}

	_, _, err := svc.Create(context.Background(), q, nonAdmin, CreateServiceAccountInput{Label: "x"})
	if err == nil || !isErrForbidden(err) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceAccountService_Create_EmptyLabel(t *testing.T) {
	q := newMockQuerier()
	svc := &ServiceAccountService{aw: &mockAuditWriter{}}
	admin := Principal{IsAdmin: true}

	_, _, err := svc.Create(context.Background(), q, admin, CreateServiceAccountInput{Label: ""})
	if err == nil || !isErrInvalid(err) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestLegalEntityService_Create(t *testing.T) {
	q := newMockQuerier()
	svc := &LegalEntityService{}

	entity, _ := q.CreateEntity(context.Background(), "legal_entity")
	le, err := svc.Create(context.Background(), q, coredb.CreateLegalEntityParams{
		EntityID:    entity.ID,
		Kind:        "natural_person",
		DisplayName: "Test User",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if le.Kind != "natural_person" {
		t.Errorf("kind: got %q, want natural_person", le.Kind)
	}
}

func TestLegalEntityService_GetByEntityID_Found(t *testing.T) {
	q := newMockQuerier()
	svc := &LegalEntityService{}

	entityUUID := q.seedNaturalPerson("Eve", "Gray")
	entity, _ := q.GetEntityByUUID(context.Background(), entityUUID)

	le, err := svc.GetByEntityID(context.Background(), q, entity.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if le.Kind != "natural_person" {
		t.Errorf("kind: got %q, want natural_person", le.Kind)
	}
}

func TestServices_New(t *testing.T) {
	aw := &mockAuditWriter{}
	svcs := New(newMockQuerier(), aw)
	if svcs == nil {
		t.Fatal("expected non-nil Services")
	}
	if svcs.Entity == nil {
		t.Error("Entity service should not be nil")
	}
	if svcs.NaturalPerson == nil {
		t.Error("NaturalPerson service should not be nil")
	}
	if svcs.Querier() == nil {
		t.Error("Querier() should not be nil")
	}
}

// isErrForbidden checks if err wraps ErrForbidden.
func isErrForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// isErrInvalid checks if err wraps ErrInvalidInput.
func isErrInvalid(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}
