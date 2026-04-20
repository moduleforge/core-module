package service

import (
	"context"
	"errors"
	"testing"
)

func TestCorporationService_Create_WritesAudit(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &CorporationService{aw: aw}
	admin := Principal{UserID: 1, EntityID: 1, IsAdmin: true}

	in := CreateCorporationInput{LegalName: "Acme Corp", Jurisdiction: "DE"}
	corp, entityUUID, err := svc.Create(context.Background(), q, admin, in)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if corp.LegalName != "Acme Corp" {
		t.Errorf("legal_name: got %q, want %q", corp.LegalName, "Acme Corp")
	}
	if entityUUID.String() == "" {
		t.Error("expected non-empty entity UUID")
	}

	if len(aw.calls) != 1 {
		t.Fatalf("expected 1 audit call, got %d", len(aw.calls))
	}
	c := aw.calls[0]
	if c.op != "create" {
		t.Errorf("audit op: got %q, want %q", c.op, "create")
	}
	if c.resource != "corporation" {
		t.Errorf("audit resource: got %q, want %q", c.resource, "corporation")
	}
}

func TestCorporationService_Create_RequiresAdmin(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &CorporationService{aw: aw}
	nonAdmin := Principal{IsAdmin: false}

	_, _, err := svc.Create(context.Background(), q, nonAdmin, CreateCorporationInput{LegalName: "Foo"})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestCorporationService_Create_EmptyLegalName(t *testing.T) {
	q := newMockQuerier()
	svc := &CorporationService{aw: &mockAuditWriter{}}
	admin := Principal{IsAdmin: true}

	_, _, err := svc.Create(context.Background(), q, admin, CreateCorporationInput{LegalName: "   "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCorporationService_GetByEntityUUID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := &CorporationService{aw: &mockAuditWriter{}}

	_, err := svc.GetByEntityUUID(context.Background(), q, randomUUID(nil))
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

func TestCorporationService_GetByEntityUUID_Found(t *testing.T) {
	q := newMockQuerier()
	svc := &CorporationService{aw: &mockAuditWriter{}}
	admin := Principal{IsAdmin: true}

	// Create via the service to set up all rows.
	_, entityUUID, err := svc.Create(context.Background(), q, admin, CreateCorporationInput{LegalName: "Beta LLC"})
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

func TestCorporationService_Update_RequiresAdmin(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &CorporationService{aw: aw}
	nonAdmin := Principal{IsAdmin: false}

	_, entityUUID, _ := (&CorporationService{aw: aw}).Create(context.Background(), q, Principal{IsAdmin: true}, CreateCorporationInput{LegalName: "Gamma Inc"})

	ln := "Delta Inc"
	err := svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateCorporationInput{LegalName: &ln}, nonAdmin)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestCorporationService_Update_AdminSucceeds(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &CorporationService{aw: aw}
	admin := Principal{IsAdmin: true}

	_, entityUUID, _ := svc.Create(context.Background(), q, admin, CreateCorporationInput{LegalName: "Epsilon Corp"})

	ln := "Epsilon Corporation"
	err := svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateCorporationInput{LegalName: &ln}, admin)
	if err != nil {
		t.Fatalf("Update: unexpected error: %v", err)
	}

	// 2 audit calls: create + update
	if len(aw.calls) != 2 {
		t.Fatalf("expected 2 audit calls, got %d", len(aw.calls))
	}
	if aw.calls[1].op != "update" {
		t.Errorf("audit op: got %q, want update", aw.calls[1].op)
	}
}
