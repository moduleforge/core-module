package entity_test

import (
	"testing"

	"github.com/moduleforge/core-api/entity"
)

// int64ptr is a small helper to take the address of an int64 literal.
func int64ptr(v int64) *int64 { return &v }

// TestEntity_Resource verifies that each concrete type returns the expected
// stable resource slug, regardless of whether an ID is set.
func TestEntity_Resource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		e        entity.Entity
		wantSlug string
	}{
		{
			name:     "LegalEntity",
			e:        entity.LegalEntity{},
			wantSlug: "legal_entity",
		},
		{
			name:     "NaturalPerson",
			e:        entity.NaturalPerson{},
			wantSlug: "natural_person",
		},
		{
			name:     "Corporation",
			e:        entity.Corporation{},
			wantSlug: "corporation",
		},
		{
			name:     "ServiceAccount",
			e:        entity.ServiceAccount{},
			wantSlug: "service_account",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.e.Resource()
			if got != tc.wantSlug {
				t.Errorf("Resource() = %q; want %q", got, tc.wantSlug)
			}
		})
	}
}

// TestEntity_EntityID verifies that EntityID returns nil for a not-yet-persisted
// entity and a non-nil pointer with the correct value after the ID is set.
func TestEntity_EntityID(t *testing.T) {
	t.Parallel()

	id := int64ptr(42)

	cases := []struct {
		name    string
		nilCase entity.Entity
		idCase  entity.Entity
		wantID  *int64
	}{
		{
			name:    "LegalEntity",
			nilCase: entity.LegalEntity{},
			idCase:  entity.LegalEntity{ID: id},
			wantID:  id,
		},
		{
			name:    "NaturalPerson",
			nilCase: entity.NaturalPerson{},
			idCase:  entity.NaturalPerson{ID: id},
			wantID:  id,
		},
		{
			name:    "Corporation",
			nilCase: entity.Corporation{},
			idCase:  entity.Corporation{ID: id},
			wantID:  id,
		},
		{
			name:    "ServiceAccount",
			nilCase: entity.ServiceAccount{},
			idCase:  entity.ServiceAccount{ID: id},
			wantID:  id,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/nil", func(t *testing.T) {
			t.Parallel()
			got := tc.nilCase.EntityID()
			if got != nil {
				t.Errorf("EntityID() = %v; want nil for unpersisted entity", got)
			}
		})

		t.Run(tc.name+"/set", func(t *testing.T) {
			t.Parallel()
			got := tc.idCase.EntityID()
			if got == nil {
				t.Fatalf("EntityID() = nil; want non-nil")
			}
			if *got != *tc.wantID {
				t.Errorf("*EntityID() = %d; want %d", *got, *tc.wantID)
			}
		})
	}
}

// TestEntity_PublicUUID verifies PublicUUID returns "" for a not-yet-persisted
// entity and the correct value after the UUID is set.
func TestEntity_PublicUUID(t *testing.T) {
	t.Parallel()

	const want = "11111111-2222-3333-4444-555555555555"

	cases := []struct {
		name      string
		empty     entity.Entity
		populated entity.Entity
	}{
		{"LegalEntity", entity.LegalEntity{}, entity.LegalEntity{UUID: want}},
		{"NaturalPerson", entity.NaturalPerson{}, entity.NaturalPerson{UUID: want}},
		{"Corporation", entity.Corporation{}, entity.Corporation{UUID: want}},
		{"ServiceAccount", entity.ServiceAccount{}, entity.ServiceAccount{UUID: want}},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/empty", func(t *testing.T) {
			t.Parallel()
			if got := tc.empty.PublicUUID(); got != "" {
				t.Errorf("PublicUUID() = %q; want \"\" for unpersisted entity", got)
			}
		})
		t.Run(tc.name+"/set", func(t *testing.T) {
			t.Parallel()
			if got := tc.populated.PublicUUID(); got != want {
				t.Errorf("PublicUUID() = %q; want %q", got, want)
			}
		})
	}
}

// TestEntity_InterfaceCompliance is a compile-time check verifying each
// concrete type satisfies the Entity interface. The test body is empty —
// the var declarations are the assertion.
func TestEntity_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	var (
		_ entity.Entity = entity.LegalEntity{}
		_ entity.Entity = entity.NaturalPerson{}
		_ entity.Entity = entity.Corporation{}
		_ entity.Entity = entity.ServiceAccount{}
	)
}
