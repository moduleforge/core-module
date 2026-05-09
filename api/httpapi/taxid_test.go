package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// buildNPProfileWithSSN builds a Profile that already has TaxID/TaxIDType
// populated (as if ResolveProfileByEntityID was called with a cipher).
func buildNPProfileWithSSN(entityID int64, givenName, familyName, ssn string) service.Profile {
	entityUUID := uuid.New()
	np := &coredb.GetNaturalPersonByEntityIDRow{
		GivenName:  pgtype.Text{String: givenName, Valid: true},
		FamilyName: pgtype.Text{String: familyName, Valid: true},
	}
	return service.Profile{
		Entity: coredb.GetEntityByUUIDRow{
			ID:                  entityID,
			Uuid:                entityUUID,
			FundamentalTypeSlug: "natural_person",
		},
		Kind:          "natural_person",
		NaturalPerson: np,
		TaxID:         ssn,
		TaxIDType:     "SSN",
	}
}

func buildCorpProfileWithEIN(entityID int64, legalName, ein string) service.Profile {
	entityUUID := uuid.New()
	corp := &coredb.GetCorporationByEntityIDRow{
		LegalName: legalName,
	}
	return service.Profile{
		Entity: coredb.GetEntityByUUIDRow{
			ID:                  entityID,
			Uuid:                entityUUID,
			FundamentalTypeSlug: "corporation",
		},
		Kind:        "corporation",
		Corporation: corp,
		TaxID:       ein,
		TaxIDType:   "EIN",
	}
}

// --- Test 1: Admin POST NP with SSN → 201 with tax_id/tax_id_type ---

func TestCreateNaturalPerson_WithSSN_AdminSeesTaxID(t *testing.T) {
	entityUUID := uuid.New()
	npResult := coredb.CreateNaturalPersonRow{
		GivenName:  pgtype.Text{String: "Alice", Valid: true},
		FamilyName: pgtype.Text{String: "Smith", Valid: true},
	}
	npSvc := &fakeNaturalPersonService{createNP: npResult, createUUID: entityUUID}
	d := buildTestDeps(nil, npSvc, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(createNaturalPersonRequest{
		GivenName:  "Alice",
		FamilyName: "Smith",
		SSN:        "123-45-6789",
	})
	req := adminReq(http.MethodPost, "/entities/natural-persons", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["tax_id"] != "123-45-6789" {
		t.Errorf("tax_id: got %v, want %q", resp["tax_id"], "123-45-6789")
	}
	if resp["tax_id_type"] != "SSN" {
		t.Errorf("tax_id_type: got %v, want %q", resp["tax_id_type"], "SSN")
	}
}

// --- Test 2: GET returns tax_id/tax_id_type when profile has SSN ---

func TestGetNaturalPerson_SeesTaxID(t *testing.T) {
	const entityID = int64(42)
	profile := buildNPProfileWithSSN(entityID, "Alice", "Smith", "123-45-6789")

	npSvc := &fakeNaturalPersonService{profile: profile}
	d := buildTestDeps(nil, npSvc, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/natural-persons/"+profile.Entity.Uuid.String(), nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["tax_id"] != "123-45-6789" {
		t.Errorf("tax_id: got %v, want %q", resp["tax_id"], "123-45-6789")
	}
	if resp["tax_id_type"] != "SSN" {
		t.Errorf("tax_id_type: got %v, want %q", resp["tax_id_type"], "SSN")
	}
}

// --- Test 3: PUT with ssn="" clears the value; subsequent GET returns absent tax_id ---

func TestUpdateNaturalPerson_ClearSSN_TaxIDAbsent(t *testing.T) {
	const entityID = int64(42)
	// After the update the service returns a profile with TaxID="" (cleared).
	clearedProfile := service.Profile{
		Entity: coredb.GetEntityByUUIDRow{
			ID:                  entityID,
			Uuid:                uuid.New(),
			FundamentalTypeSlug: "natural_person",
		},
		Kind: "natural_person",
		NaturalPerson: &coredb.GetNaturalPersonByEntityIDRow{
			GivenName:  pgtype.Text{String: "Alice", Valid: true},
			FamilyName: pgtype.Text{String: "Smith", Valid: true},
		},
		TaxID:     "", // cleared
		TaxIDType: "SSN",
	}

	npSvc := &fakeNaturalPersonService{profile: clearedProfile}
	d := buildTestDeps(nil, npSvc, nil, nil)
	router := NewRouter(d)

	emptySSN := ""
	body, _ := json.Marshal(updateNaturalPersonRequest{SSN: &emptySSN})
	req := httptest.NewRequest(http.MethodPut, "/entities/natural-persons/"+clearedProfile.Entity.Uuid.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// TaxID is "" so the gate strips it from the response.
	if _, present := resp["tax_id"]; present {
		t.Errorf("tax_id should be absent after clearing SSN, got %v", resp["tax_id"])
	}
}

// --- Test 4: PUT without ssn field leaves existing value intact ---

func TestUpdateNaturalPerson_OmitSSN_ExistingTaxIDPreserved(t *testing.T) {
	const entityID = int64(42)
	profile := buildNPProfileWithSSN(entityID, "Alice", "Smith", "123-45-6789")

	npSvc := &fakeNaturalPersonService{profile: profile}
	d := buildTestDeps(nil, npSvc, nil, nil)
	router := NewRouter(d)

	// Only update given_name; SSN field omitted entirely.
	givenName := "Alicia"
	body, _ := json.Marshal(updateNaturalPersonRequest{GivenName: &givenName})
	req := httptest.NewRequest(http.MethodPut, "/entities/natural-persons/"+profile.Entity.Uuid.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The fake service returns the pre-built profile with TaxID set.
	if resp["tax_id"] != "123-45-6789" {
		t.Errorf("tax_id: got %v, want %q — existing SSN should be preserved", resp["tax_id"], "123-45-6789")
	}
}

// --- Test 5: Admin POST Corporation with EIN → 201 contains tax_id/tax_id_type ---

func TestCreateCorporation_WithEIN_AdminSeesTaxID(t *testing.T) {
	entityUUID := uuid.New()
	corpResult := coredb.CreateCorporationRow{LegalName: "Acme Corp"}
	corpSvc := &fakeCorporationService{createCorp: corpResult, createUUID: entityUUID}
	d := buildTestDeps(nil, nil, corpSvc, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(createCorporationRequest{
		LegalName: "Acme Corp",
		EIN:       "12-3456789",
	})
	req := adminReq(http.MethodPost, "/entities/corporations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["tax_id"] != "12-3456789" {
		t.Errorf("tax_id: got %v, want %q", resp["tax_id"], "12-3456789")
	}
	if resp["tax_id_type"] != "EIN" {
		t.Errorf("tax_id_type: got %v, want %q", resp["tax_id_type"], "EIN")
	}
}

// --- profileResponse gate unit tests ---

func TestProfileResponse_IncludesTaxIDWhenNonEmpty(t *testing.T) {
	const entityID = int64(10)
	profile := buildNPProfileWithSSN(entityID, "Joe", "Doe", "999-88-7777")

	resp := profileResponse(profile)

	if resp["tax_id"] != "999-88-7777" {
		t.Errorf("expected tax_id in response, got %v", resp["tax_id"])
	}
	if resp["tax_id_type"] != "SSN" {
		t.Errorf("tax_id_type: got %v", resp["tax_id_type"])
	}
}

func TestProfileResponse_OmitsTaxIDWhenEmpty(t *testing.T) {
	// Even when non-admin, if TaxID is "" (no SSN stored) the keys are omitted.
	const entityID = int64(10)
	profile := service.Profile{
		Entity: coredb.GetEntityByUUIDRow{
			ID:                  entityID,
			Uuid:                uuid.New(),
			FundamentalTypeSlug: "natural_person",
		},
		Kind: "natural_person",
		NaturalPerson: &coredb.GetNaturalPersonByEntityIDRow{
			GivenName:  pgtype.Text{String: "Joe", Valid: true},
			FamilyName: pgtype.Text{String: "Doe", Valid: true},
		},
		TaxID:     "",
		TaxIDType: "SSN",
	}

	resp := profileResponse(profile)

	if _, present := resp["tax_id"]; present {
		t.Errorf("empty tax_id should be omitted, got %v", resp["tax_id"])
	}
}
