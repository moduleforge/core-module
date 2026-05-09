package service

import (
	"context"
	"errors"
	"testing"

	"github.com/moduleforge/core-api/observer"
)

func newCorpService(t *testing.T, q *mockQuerier) *CorporationService {
	t.Helper()
	return &CorporationService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(),
		cipher:         testCipher(t),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
}

func TestCorporationService_Create_WritesObserver(t *testing.T) {
	q := newMockQuerier()
	rec := &recordingObserver{}
	svc := &CorporationService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(rec),
		cipher:         testCipher(t),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}

	in := CreateCorporationInput{LegalName: "Acme Corp", Jurisdiction: "DE"}
	corp, entityUUID, err := svc.Create(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if corp.LegalName != "Acme Corp" {
		t.Errorf("legal_name: got %q, want %q", corp.LegalName, "Acme Corp")
	}
	if entityUUID.String() == "" {
		t.Error("expected non-empty entity UUID")
	}

	if len(rec.observeCalls) != 1 {
		t.Fatalf("expected 1 in-tx observe call, got %d", len(rec.observeCalls))
	}
	c := rec.observeCalls[0]
	if c.op != "create" {
		t.Errorf("op: got %q, want create", c.op)
	}
	if c.resource != "corporation" {
		t.Errorf("resource: got %q, want corporation", c.resource)
	}
}

func TestCorporationService_Create_AuthzDenied(t *testing.T) {
	q := newMockQuerier()
	authzErr := errors.New("unauthorized")
	svc := &CorporationService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: authzErr},
		obs:            observer.NewObserverGroup(),
		cipher:         testCipher(t),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}

	_, _, err := svc.Create(context.Background(), q, CreateCorporationInput{LegalName: "Foo"})
	if !errors.Is(err, authzErr) {
		t.Errorf("expected authz error, got %v", err)
	}
}

func TestCorporationService_Create_EmptyLegalName(t *testing.T) {
	q := newMockQuerier()
	svc := newCorpService(t, q)

	_, _, err := svc.Create(context.Background(), q, CreateCorporationInput{LegalName: "   "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCorporationService_GetByEntityUUID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := newCorpService(t, q)

	_, err := svc.GetByEntityUUID(context.Background(), q, randomUUID(nil))
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

func TestCorporationService_GetByEntityUUID_Found(t *testing.T) {
	q := newMockQuerier()
	svc := newCorpService(t, q)

	// Create via the service to set up all rows.
	_, entityUUID, err := svc.Create(context.Background(), q, CreateCorporationInput{LegalName: "Beta LLC"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	profile, err := svc.GetByEntityUUID(context.Background(), q, entityUUID)
	if err != nil {
		t.Fatalf("GetByEntityUUID: %v", err)
	}
	if profile.Kind != "corporation" {
		t.Errorf("kind: got %q, want corporation", profile.Kind)
	}
}

func TestCorporationService_Update_AuthzDenied(t *testing.T) {
	q := newMockQuerier()
	authzErr := errors.New("unauthorized")

	// First create via allow-all service.
	setupSvc := newCorpService(t, q)
	_, entityUUID, err := setupSvc.Create(context.Background(), q, CreateCorporationInput{LegalName: "Gamma Inc"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := &CorporationService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: authzErr},
		obs:            observer.NewObserverGroup(),
		cipher:         testCipher(t),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	ln := "Delta Inc"
	err = svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateCorporationInput{LegalName: &ln})
	if !errors.Is(err, authzErr) {
		t.Errorf("expected authz error, got %v", err)
	}
}

func TestCorporationService_Update_AdminSucceeds(t *testing.T) {
	q := newMockQuerier()
	rec := &recordingObserver{}
	svc := &CorporationService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(rec),
		cipher:         testCipher(t),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}

	_, entityUUID, err := svc.Create(context.Background(), q, CreateCorporationInput{LegalName: "Epsilon Corp"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Reset recording after create.
	rec.observeCalls = nil
	rec.observeAfterCommitCalls = nil

	ln := "Epsilon Corporation"
	err = svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateCorporationInput{LegalName: &ln})
	if err != nil {
		t.Fatalf("Update: unexpected error: %v", err)
	}

	if len(rec.observeCalls) != 1 {
		t.Fatalf("expected 1 in-tx observe call for update, got %d", len(rec.observeCalls))
	}
	if rec.observeCalls[0].op != "update" {
		t.Errorf("op: got %q, want update", rec.observeCalls[0].op)
	}
}

func TestCorporationService_UpdateByEntityUUID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := newCorpService(t, q)

	ln := "X"
	err := svc.UpdateByEntityUUID(context.Background(), q, randomUUID(t), UpdateCorporationInput{LegalName: &ln})
	if err == nil {
		t.Error("expected error for missing entity")
	}
}
