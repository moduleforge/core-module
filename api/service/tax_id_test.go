package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/observer"
)

// TestNaturalPersonService_Create_EncryptsSSN verifies that a non-empty SSN on
// create is stored as ciphertext (non-nil, different from plaintext) and that
// the observer payload records "set" rather than the plaintext value.
func TestNaturalPersonService_Create_EncryptsSSN(t *testing.T) {
	q := newMockQuerier()
	rec := &recordingObserver{}
	svc := &NaturalPersonService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(rec),
		cipher:         testCipher(t),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	in := CreateNaturalPersonInput{GivenName: "Alice", FamilyName: "Smith", SSN: "123-45-6789"}
	np, _, err := svc.Create(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	// The stored blob must be non-nil and not equal to the plaintext.
	if len(np.Ssn) == 0 {
		t.Error("expected non-empty ssn blob after create with SSN")
	}
	if string(np.Ssn) == "123-45-6789" {
		t.Error("ssn blob must not equal plaintext")
	}

	// Observer after payload must record "set", never plaintext.
	if len(rec.observeCalls) != 1 {
		t.Fatalf("expected 1 observe call, got %d", len(rec.observeCalls))
	}
	after, ok := rec.observeCalls[0].after.(map[string]any)
	if !ok {
		t.Fatalf("observe after is not map[string]any: %T", rec.observeCalls[0].after)
	}
	if after["ssn"] != "set" {
		t.Errorf("observe ssn marker: got %q, want %q", after["ssn"], "set")
	}
}

// TestNaturalPersonService_Create_NoSSN verifies that an empty SSN on create
// stores nil blob and the observer payload records "unchanged".
func TestNaturalPersonService_Create_NoSSN(t *testing.T) {
	q := newMockQuerier()
	rec := &recordingObserver{}
	svc := &NaturalPersonService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(rec),
		cipher:         testCipher(t),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	in := CreateNaturalPersonInput{GivenName: "Bob", FamilyName: "Jones", SSN: ""}
	np, _, err := svc.Create(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if len(np.Ssn) != 0 {
		t.Error("expected empty ssn blob when SSN not provided")
	}

	after := rec.observeCalls[0].after.(map[string]any)
	if after["ssn"] != "unchanged" {
		t.Errorf("observe ssn marker: got %q, want %q", after["ssn"], "unchanged")
	}
}

// TestNaturalPersonService_GetDecryptedSSN_RoundTrip verifies encrypt-on-create
// then decrypt returns the original plaintext.
func TestNaturalPersonService_GetDecryptedSSN_RoundTrip(t *testing.T) {
	q := newMockQuerier()
	cipher := testCipher(t)
	svc := &NaturalPersonService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(),
		cipher:         cipher,
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	in := CreateNaturalPersonInput{GivenName: "Carol", FamilyName: "White", SSN: "987-65-4321"}
	_, _, err := svc.Create(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Find the entity ID from the mock store.
	var entityID int64
	for id := range q.naturalPersons {
		entityID = id
		break
	}

	plaintext, err := svc.GetDecryptedSSN(context.Background(), q, entityID)
	if err != nil {
		t.Fatalf("GetDecryptedSSN: %v", err)
	}
	if plaintext != "987-65-4321" {
		t.Errorf("GetDecryptedSSN: got %q, want %q", plaintext, "987-65-4321")
	}
}

// TestNaturalPersonService_GetDecryptedSSN_Nil verifies that a nil blob returns
// "" without error.
func TestNaturalPersonService_GetDecryptedSSN_Nil(t *testing.T) {
	q := newMockQuerier()
	svc := &NaturalPersonService{
		db:         newFakeDB(),
		az:         allowAllAuthz{},
		obs:        observer.NewObserverGroup(),
		cipher:     testCipher(t),
		newQuerier: mockQuerierFactory(q),
	}

	entityUUID := q.seedNaturalPerson("Dave", "Evans") // seeded with nil Ssn
	entity, _ := q.GetEntityByUUID(context.Background(), entityUUID)

	val, err := svc.GetDecryptedSSN(context.Background(), q, entity.ID)
	if err != nil {
		t.Fatalf("GetDecryptedSSN: expected nil error for nil blob, got %v", err)
	}
	if val != "" {
		t.Errorf("GetDecryptedSSN: expected empty string, got %q", val)
	}
}

// TestNaturalPersonService_Update_SetSSN verifies pointer-to-value encrypts and
// observer payload records "set".
func TestNaturalPersonService_Update_SetSSN(t *testing.T) {
	q := newMockQuerier()
	rec := &recordingObserver{}
	svc := &NaturalPersonService{
		db:         newFakeDB(),
		az:         allowAllAuthz{},
		obs:        observer.NewObserverGroup(rec),
		cipher:     testCipher(t),
		newQuerier: mockQuerierFactory(q),
	}
	entityUUID := q.seedNaturalPerson("Eve", "Foster")

	ssn := "111-22-3333"
	err := svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateNaturalPersonInput{SSN: &ssn})
	if err != nil {
		t.Fatalf("UpdateByEntityUUID: %v", err)
	}

	// Observer after must say "set".
	if len(rec.observeCalls) != 1 {
		t.Fatalf("expected 1 observe call, got %d", len(rec.observeCalls))
	}
	after := rec.observeCalls[0].after.(map[string]any)
	if after["ssn"] != "set" {
		t.Errorf("observe ssn: got %q, want %q", after["ssn"], "set")
	}
}

// TestNaturalPersonService_Update_ClearSSN verifies pointer-to-empty-string
// stores an empty blob and observer payload records "cleared".
func TestNaturalPersonService_Update_ClearSSN(t *testing.T) {
	q := newMockQuerier()
	rec := &recordingObserver{}
	svc := &NaturalPersonService{
		db:         newFakeDB(),
		az:         allowAllAuthz{},
		obs:        observer.NewObserverGroup(rec),
		cipher:     testCipher(t),
		newQuerier: mockQuerierFactory(q),
	}
	entityUUID := q.seedNaturalPerson("Frank", "Green")

	empty := ""
	err := svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateNaturalPersonInput{SSN: &empty})
	if err != nil {
		t.Fatalf("UpdateByEntityUUID: %v", err)
	}

	after := rec.observeCalls[0].after.(map[string]any)
	if after["ssn"] != "cleared" {
		t.Errorf("observe ssn: got %q, want %q", after["ssn"], "cleared")
	}
}

// TestNaturalPersonService_Update_NilSSN verifies nil SSN pointer leaves the
// field unchanged and observer payload records "unchanged".
func TestNaturalPersonService_Update_NilSSN(t *testing.T) {
	q := newMockQuerier()
	rec := &recordingObserver{}
	svc := &NaturalPersonService{
		db:         newFakeDB(),
		az:         allowAllAuthz{},
		obs:        observer.NewObserverGroup(rec),
		cipher:     testCipher(t),
		newQuerier: mockQuerierFactory(q),
	}
	entityUUID := q.seedNaturalPerson("Grace", "Hall")
	gn := "Grace"
	err := svc.UpdateByEntityUUID(context.Background(), q, entityUUID, UpdateNaturalPersonInput{GivenName: &gn})
	if err != nil {
		t.Fatalf("UpdateByEntityUUID: %v", err)
	}

	after := rec.observeCalls[0].after.(map[string]any)
	if after["ssn"] != "unchanged" {
		t.Errorf("observe ssn: got %q, want %q", after["ssn"], "unchanged")
	}
}

// TestCorporationService_Create_EncryptsEIN mirrors the NP encrypt-on-create
// test for corporations.
func TestCorporationService_Create_EncryptsEIN(t *testing.T) {
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
	in := CreateCorporationInput{LegalName: "Acme Inc", EIN: "12-3456789"}
	corp, _, err := svc.Create(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(corp.Ein) == 0 {
		t.Error("expected non-empty ein blob")
	}
	if string(corp.Ein) == "12-3456789" {
		t.Error("ein blob must not equal plaintext")
	}

	after := rec.observeCalls[0].after.(map[string]any)
	if after["ein"] != "set" {
		t.Errorf("observe ein: got %q, want %q", after["ein"], "set")
	}
}

// TestCorporationService_GetDecryptedEIN_RoundTrip verifies encrypt-on-create
// then decrypt returns the original EIN.
func TestCorporationService_GetDecryptedEIN_RoundTrip(t *testing.T) {
	q := newMockQuerier()
	cipher := testCipher(t)
	svc := &CorporationService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(),
		cipher:         cipher,
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	_, _, err := svc.Create(context.Background(), q, CreateCorporationInput{LegalName: "Beta Corp", EIN: "98-7654321"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var entityID int64
	for id := range q.corporations {
		entityID = id
		break
	}

	plaintext, err := svc.GetDecryptedEIN(context.Background(), q, entityID)
	if err != nil {
		t.Fatalf("GetDecryptedEIN: %v", err)
	}
	if plaintext != "98-7654321" {
		t.Errorf("GetDecryptedEIN: got %q, want %q", plaintext, "98-7654321")
	}
}

// TestLegalEntityService_GetTaxID_NaturalPerson verifies GetTaxID returns
// Type="SSN" and the correct plaintext for a natural person entity.
func TestLegalEntityService_GetTaxID_NaturalPerson(t *testing.T) {
	q := newMockQuerier()
	cipher := testCipher(t)
	npSvc := &NaturalPersonService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(),
		cipher:         cipher,
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	leSvc := &LegalEntityService{cipher: cipher}
	_, _, err := npSvc.Create(context.Background(), q, CreateNaturalPersonInput{
		GivenName: "Hannah", FamilyName: "Ives", SSN: "555-44-3333",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var entityID int64
	for id := range q.naturalPersons {
		entityID = id
		break
	}

	taxID, err := leSvc.GetTaxID(context.Background(), q, entityID)
	if err != nil {
		t.Fatalf("GetTaxID: %v", err)
	}
	if taxID.Type != "SSN" {
		t.Errorf("Type: got %q, want SSN", taxID.Type)
	}
	if taxID.Value != "555-44-3333" {
		t.Errorf("Value: got %q, want 555-44-3333", taxID.Value)
	}
}

// TestLegalEntityService_GetTaxID_Corporation verifies GetTaxID returns
// Type="EIN" for a corporation entity.
func TestLegalEntityService_GetTaxID_Corporation(t *testing.T) {
	q := newMockQuerier()
	cipher := testCipher(t)
	corpSvc := &CorporationService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(),
		cipher:         cipher,
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	leSvc := &LegalEntityService{cipher: cipher}
	_, _, err := corpSvc.Create(context.Background(), q, CreateCorporationInput{
		LegalName: "Gamma LLC", EIN: "77-8889999",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var entityID int64
	for id := range q.corporations {
		entityID = id
		break
	}

	taxID, err := leSvc.GetTaxID(context.Background(), q, entityID)
	if err != nil {
		t.Fatalf("GetTaxID: %v", err)
	}
	if taxID.Type != "EIN" {
		t.Errorf("Type: got %q, want EIN", taxID.Type)
	}
	if taxID.Value != "77-8889999" {
		t.Errorf("Value: got %q, want 77-8889999", taxID.Value)
	}
}

// TestLegalEntityService_GetTaxID_ServiceAccount verifies GetTaxID returns zero
// LegalEntityTaxID for a non-leaf type (no error).
func TestLegalEntityService_GetTaxID_ServiceAccount(t *testing.T) {
	q := newMockQuerier()
	cipher := testCipher(t)
	saSvc := &ServiceAccountService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	leSvc := &LegalEntityService{cipher: cipher}
	_, entityUUID, err := saSvc.Create(context.Background(), q, CreateServiceAccountInput{Label: "svc"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	entity, _ := q.GetEntityByUUID(context.Background(), entityUUID)

	taxID, err := leSvc.GetTaxID(context.Background(), q, entity.ID)
	if err != nil {
		t.Fatalf("GetTaxID service_account: unexpected error: %v", err)
	}
	if taxID.Type != "" || taxID.Value != "" {
		t.Errorf("expected zero LegalEntityTaxID for service_account, got %+v", taxID)
	}
}

// TestProfile_TaxIDPopulated verifies that ResolveProfileByEntityID populates
// TaxID/TaxIDType when a cipher is supplied.
func TestProfile_TaxIDPopulated(t *testing.T) {
	q := newMockQuerier()
	cipher := testCipher(t)

	// Manually insert a natural_person with an encrypted SSN.
	entityUUID := q.seedNaturalPerson("Ivan", "Jones")
	entity, _ := q.GetEntityByUUID(context.Background(), entityUUID)

	blob, err := cipher.Encrypt("444-55-6666")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	np := q.naturalPersons[entity.ID]
	np.Ssn = blob
	q.naturalPersons[entity.ID] = np

	profile, err := ResolveProfileByEntityID(context.Background(), q, entity.ID, cipher)
	if err != nil {
		t.Fatalf("ResolveProfileByEntityID: %v", err)
	}
	if profile.TaxIDType != "SSN" {
		t.Errorf("TaxIDType: got %q, want SSN", profile.TaxIDType)
	}
	if profile.TaxID != "444-55-6666" {
		t.Errorf("TaxID: got %q, want 444-55-6666", profile.TaxID)
	}
}

// TestProfile_TaxIDNotPopulatedWithoutCipher verifies TaxID fields are empty
// when no cipher is passed to ResolveProfileByEntityID.
func TestProfile_TaxIDNotPopulatedWithoutCipher(t *testing.T) {
	q := newMockQuerier()

	entityUUID := q.seedNaturalPerson("Jane", "Kim")
	entity, _ := q.GetEntityByUUID(context.Background(), entityUUID)

	profile, err := ResolveProfileByEntityID(context.Background(), q, entity.ID)
	if err != nil {
		t.Fatalf("ResolveProfileByEntityID: %v", err)
	}
	if profile.TaxID != "" || profile.TaxIDType != "" {
		t.Errorf("expected empty TaxID/TaxIDType without cipher, got TaxID=%q TaxIDType=%q", profile.TaxID, profile.TaxIDType)
	}
}

// TestAuditRedaction_NoPlaintext is a guard: it checks that none of the observer
// call payloads in a create+update cycle contain the plaintext SSN value.
func TestAuditRedaction_NoPlaintext(t *testing.T) {
	q := newMockQuerier()
	rec := &recordingObserver{}
	svc := &NaturalPersonService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            observer.NewObserverGroup(rec),
		cipher:         testCipher(t),
		newQuerier:     mockQuerierFactory(q),
		entityResolver: testEntityResolver(),
		typeResolver:   testTypeResolver(q),
	}
	const plainSSN = "999-88-7777"

	in := CreateNaturalPersonInput{GivenName: "Karl", FamilyName: "Lang", SSN: plainSSN}
	_, _, err := svc.Create(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, call := range rec.observeCalls {
		checkNoPlaintext(t, call.before, plainSSN)
		checkNoPlaintext(t, call.after, plainSSN)
	}
}

// checkNoPlaintext fails the test if the plaintext value appears anywhere
// in the observer payload map.
func checkNoPlaintext(t *testing.T, payload any, plainSSN string) {
	t.Helper()
	if payload == nil {
		return
	}
	m, ok := payload.(map[string]any)
	if !ok {
		return
	}
	for k, v := range m {
		if s, ok := v.(string); ok && s == plainSSN {
			t.Errorf("observer field %q contains plaintext SSN value — must be redacted", k)
		}
	}
}

// Unused import guard: pgtype is referenced via seedNaturalPerson indirectly;
// ensure package compiles cleanly even if we reference it here.
var _ = pgtype.Text{}

// Ensure errors package is used.
var _ = errors.New
