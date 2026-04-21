package service

import (
	"context"
	"errors"
	"testing"
)

func TestNaturalPersonService_Create_WritesAudit(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &NaturalPersonService{aw: aw, cipher: testCipher(t)}
	admin := Principal{UserID: 1, EntityID: 1, IsAdmin: true}

	in := CreateNaturalPersonInput{GivenName: "Alice", FamilyName: "Smith"}
	np, entityUUID, err := svc.Create(context.Background(), q, admin, in)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if np.GivenName.String != "Alice" {
		t.Errorf("given_name: got %q, want %q", np.GivenName.String, "Alice")
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
	if c.resource != "natural_person" {
		t.Errorf("audit resource: got %q, want %q", c.resource, "natural_person")
	}
	if c.after == nil {
		t.Error("audit after: expected non-nil")
	}
	if c.before != nil {
		t.Errorf("audit before: expected nil for create, got %v", c.before)
	}
}

func TestNaturalPersonService_Create_RequiresAdmin(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &NaturalPersonService{aw: aw, cipher: testCipher(t)}
	nonAdmin := Principal{UserID: 2, EntityID: 2, IsAdmin: false}

	_, _, err := svc.Create(context.Background(), q, nonAdmin, CreateNaturalPersonInput{
		GivenName: "Bob", FamilyName: "Jones",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
	if len(aw.calls) != 0 {
		t.Error("expected no audit calls on forbidden create")
	}
}

func TestNaturalPersonService_Create_ValidationErrors(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &NaturalPersonService{aw: aw, cipher: testCipher(t)}
	admin := Principal{IsAdmin: true}

	cases := []struct {
		name string
		in   CreateNaturalPersonInput
	}{
		{"empty given_name", CreateNaturalPersonInput{FamilyName: "Smith"}},
		{"empty family_name", CreateNaturalPersonInput{GivenName: "Alice"}},
		{"both empty", CreateNaturalPersonInput{}},
		{"whitespace given_name", CreateNaturalPersonInput{GivenName: "   ", FamilyName: "Smith"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := svc.Create(context.Background(), q, admin, tc.in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestNaturalPersonService_Update_Forbidden(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &NaturalPersonService{aw: aw, cipher: testCipher(t)}

	// Seed entity belonging to entity ID 10.
	entityUUID := q.seedNaturalPerson("Charlie", "Brown")

	// Non-admin caller with a different entity ID.
	nonAdmin := Principal{UserID: 2, EntityID: 999, IsAdmin: false}
	gn := "Changed"
	err := svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateNaturalPersonInput{GivenName: &gn}, nonAdmin)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
	if len(aw.calls) != 0 {
		t.Error("expected no audit calls on forbidden update")
	}
}

func TestNaturalPersonService_Update_SelfAllowed(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &NaturalPersonService{aw: aw, cipher: testCipher(t)}

	entityUUID := q.seedNaturalPerson("Dave", "Evans")
	// Get the entity ID for the seeded entity.
	entity, err := q.GetEntityByUUID(context.Background(), entityUUID)
	if err != nil {
		t.Fatalf("setup: get entity: %v", err)
	}

	// Non-admin actor whose EntityID matches the target entity.
	self := Principal{UserID: 3, EntityID: entity.ID, IsAdmin: false}
	gn := "David"
	err = svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateNaturalPersonInput{GivenName: &gn}, self)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	// Verify audit was written.
	if len(aw.calls) != 1 {
		t.Fatalf("expected 1 audit call, got %d", len(aw.calls))
	}
	if aw.calls[0].op != "update" {
		t.Errorf("audit op: got %q, want %q", aw.calls[0].op, "update")
	}
}

func TestNaturalPersonService_Update_AdminAllowed(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &NaturalPersonService{aw: aw, cipher: testCipher(t)}

	entityUUID := q.seedNaturalPerson("Eve", "Foster")

	admin := Principal{UserID: 1, EntityID: 1, IsAdmin: true}
	fn := "Forster"
	err := svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateNaturalPersonInput{FamilyName: &fn}, admin)
	if err != nil {
		t.Fatalf("expected success for admin update, got %v", err)
	}
}

func TestNaturalPersonService_Update_NotFound(t *testing.T) {
	q := newMockQuerier()
	aw := &mockAuditWriter{}
	svc := &NaturalPersonService{aw: aw, cipher: testCipher(t)}

	name := "Bob"
	err := svc.UpdateByEntityUUID(context.Background(), q, randomUUID(t), UpdateNaturalPersonInput{GivenName: &name}, Principal{IsAdmin: true})
	// The mock returns a generic error for missing UUID; service wraps it.
	if err == nil {
		t.Error("expected error for missing entity")
	}
}
