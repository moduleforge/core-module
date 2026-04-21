package service

import (
	"context"
	"errors"
	"testing"
)

// Tests for GetByID, ServiceAccountService.GetByEntityUUID,
// ServiceAccountService.UpdateByEntityUUID, LegalEntityService.GetByEntityID
// error path — covers the remaining uncovered lines.

func TestEntityService_GetByID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := &EntityService{aw: &mockAuditWriter{}}

	_, err := svc.GetByID(context.Background(), q, 9999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEntityService_GetByID_Found(t *testing.T) {
	q := newMockQuerier()
	svc := &EntityService{aw: &mockAuditWriter{}}

	entityUUID := q.seedNaturalPerson("Hank", "Williams")
	entity, _ := q.GetEntityByUUID(context.Background(), entityUUID)

	row, err := svc.GetByID(context.Background(), q, entity.ID)
	if err != nil {
		t.Fatalf("GetByID: unexpected error: %v", err)
	}
	if row.ID != entity.ID {
		t.Errorf("ID mismatch: got %d, want %d", row.ID, entity.ID)
	}
}

func TestServiceAccountService_GetByEntityUUID_Found(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &ServiceAccountService{aw: aw}
	admin := Principal{IsAdmin: true}

	_, entityUUID, err := svc.Create(context.Background(), q, admin, CreateServiceAccountInput{Label: "svc-x"})
	if err != nil {
		t.Fatalf("setup Create: %v", err)
	}

	profile, err := svc.GetByEntityUUID(context.Background(), q, entityUUID)
	if err != nil {
		t.Fatalf("GetByEntityUUID: %v", err)
	}
	if profile.Kind != "service_account" {
		t.Errorf("kind: got %q, want service_account", profile.Kind)
	}
	if profile.ServiceAccount.Label != "svc-x" {
		t.Errorf("label: got %q", profile.ServiceAccount.Label)
	}
}

func TestServiceAccountService_GetByEntityUUID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := &ServiceAccountService{aw: &mockAuditWriter{}}

	_, err := svc.GetByEntityUUID(context.Background(), q, randomUUID(t))
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

func TestServiceAccountService_UpdateByEntityUUID_RequiresAdmin(t *testing.T) {
	q := newMockQuerier()
	svc := &ServiceAccountService{aw: &mockAuditWriter{}}
	nonAdmin := Principal{IsAdmin: false}

	err := svc.UpdateByEntityUUID(context.Background(), q, randomUUID(t), UpdateServiceAccountInput{}, nonAdmin)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceAccountService_UpdateByEntityUUID_NilLabel(t *testing.T) {
	q := newMockQuerier()
	svc := &ServiceAccountService{aw: &mockAuditWriter{}}
	admin := Principal{IsAdmin: true}

	// When label is nil, returns nil immediately (nothing to update).
	err := svc.UpdateByEntityUUID(context.Background(), q, randomUUID(t), UpdateServiceAccountInput{Label: nil}, admin)
	if err != nil {
		t.Errorf("expected nil for no-op update, got %v", err)
	}
}

func TestServiceAccountService_UpdateByEntityUUID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := &ServiceAccountService{aw: &mockAuditWriter{}}
	admin := Principal{IsAdmin: true}

	label := "x"
	err := svc.UpdateByEntityUUID(context.Background(), q, randomUUID(t), UpdateServiceAccountInput{Label: &label}, admin)
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

func TestLegalEntityService_GetByEntityID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := &LegalEntityService{}

	_, err := svc.GetByEntityID(context.Background(), q, 99999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCorporationService_UpdateByEntityUUID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := &CorporationService{aw: &mockAuditWriter{}, cipher: testCipher(t)}
	admin := Principal{IsAdmin: true}

	ln := "X"
	err := svc.UpdateByEntityUUID(context.Background(), q, randomUUID(t), UpdateCorporationInput{LegalName: &ln}, admin)
	if err == nil {
		t.Error("expected error for missing entity")
	}
}
