package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// randomUUID returns a random UUID for use in tests that need a "missing" UUID.
// t may be nil when called outside a test function (e.g. setup helpers).
func randomUUID(t *testing.T) uuid.UUID {
	if t != nil {
		t.Helper()
	}
	return uuid.New()
}

func TestEntityService_Archive_AdminOnly(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &EntityService{aw: aw}

	// Seed an entity.
	entityUUID := q.seedNaturalPerson("Grace", "Hopper")

	// Non-admin attempt.
	nonAdmin := Principal{UserID: 2, EntityID: 999, IsAdmin: false}
	err := svc.Archive(context.Background(), q, entityUUID, nonAdmin)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestEntityService_Archive_AdminSucceeds(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &EntityService{aw: aw}

	entityUUID := q.seedNaturalPerson("Grace", "Hopper")
	admin := Principal{UserID: 1, EntityID: 1, IsAdmin: true}

	err := svc.Archive(context.Background(), q, entityUUID, admin)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if len(aw.calls) != 1 {
		t.Fatalf("expected 1 audit call, got %d", len(aw.calls))
	}
	if aw.calls[0].op != "delete" {
		t.Errorf("audit op: got %q, want %q", aw.calls[0].op, "delete")
	}
}

func TestEntityService_Archive_NotFound(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &EntityService{aw: aw}

	admin := Principal{IsAdmin: true}
	err := svc.Archive(context.Background(), q, randomUUID(t), admin)
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

func TestEntityService_GetByUUID_NotFound(t *testing.T) {
	q := newMockQuerier()
	svc := &EntityService{aw: &mockAuditWriter{}}

	_, err := svc.GetByUUID(context.Background(), q, randomUUID(t))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEntityService_GetByUUID_Found(t *testing.T) {
	q := newMockQuerier()
	svc := &EntityService{aw: &mockAuditWriter{}}

	entityUUID := q.seedNaturalPerson("Hank", "Aaron")
	e, err := svc.GetByUUID(context.Background(), q, entityUUID)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if e.Uuid != entityUUID {
		t.Errorf("uuid mismatch: got %v, want %v", e.Uuid, entityUUID)
	}
}
