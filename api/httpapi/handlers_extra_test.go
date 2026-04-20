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

func buildCorpProfile() service.Profile {
	entityUUID := uuid.New()
	corp := &coredb.Corporation{
		LegalName:    "Acme Corp",
		Jurisdiction: pgtype.Text{String: "DE", Valid: true},
	}
	return service.Profile{
		Entity:      coredb.Entity{Uuid: entityUUID},
		Kind:        "corporation",
		Corporation: corp,
	}
}

func buildSAProfile() service.Profile {
	entityUUID := uuid.New()
	sa := &coredb.ServiceAccount{Label: "my-svc"}
	return service.Profile{
		Entity:         coredb.Entity{Uuid: entityUUID},
		Kind:           "service_account",
		ServiceAccount: sa,
	}
}

// --- GET /entities/natural-persons/{uuid} ---

func TestGetNaturalPerson_401(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, nil, &fakeNaturalPersonService{}, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/natural-persons/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetNaturalPerson_200_Admin(t *testing.T) {
	profile := buildNaturalPersonProfile("Faye", "Green")
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, IsAdmin: true}, ok: true}
	npSvc := &fakeNaturalPersonService{profile: profile}
	d := buildTestDeps(ext, nil, npSvc, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/natural-persons/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetNaturalPerson_403_WrongOwner(t *testing.T) {
	entityID := int64(10)
	profile := buildNaturalPersonProfile("Gale", "Hart")
	profile.Entity.ID = entityID

	// Non-admin with different entity ID.
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 2, EntityID: 999, IsAdmin: false}, ok: true}
	npSvc := &fakeNaturalPersonService{profile: profile}
	d := buildTestDeps(ext, nil, npSvc, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/natural-persons/"+profile.Entity.Uuid.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGetNaturalPerson_404(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, IsAdmin: true}, ok: true}
	npSvc := &fakeNaturalPersonService{err: service.ErrNotFound}
	d := buildTestDeps(ext, nil, npSvc, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/natural-persons/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- PUT /entities/natural-persons/{uuid} ---

func TestUpdateNaturalPerson_401(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, nil, &fakeNaturalPersonService{}, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(map[string]string{"given_name": "X"})
	req := httptest.NewRequest(http.MethodPut, "/entities/natural-persons/"+uuid.New().String(), bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateNaturalPerson_200(t *testing.T) {
	profile := buildNaturalPersonProfile("Igor", "Jones")
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, IsAdmin: true}, ok: true}
	npSvc := &fakeNaturalPersonService{profile: profile}
	d := buildTestDeps(ext, nil, npSvc, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(map[string]string{"given_name": "Igor"})
	req := httptest.NewRequest(http.MethodPut, "/entities/natural-persons/"+profile.Entity.Uuid.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUpdateNaturalPerson_400_BadJSON(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1}, ok: true}
	d := buildTestDeps(ext, nil, &fakeNaturalPersonService{}, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodPut, "/entities/natural-persons/"+uuid.New().String(), bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- GET /entities/corporations/{uuid} ---

func TestGetCorporation_401(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, nil, nil, &fakeCorporationService{}, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/corporations/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetCorporation_200(t *testing.T) {
	profile := buildCorpProfile()
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1}, ok: true}
	corpSvc := &fakeCorporationService{profile: profile}
	d := buildTestDeps(ext, nil, nil, corpSvc, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/corporations/"+profile.Entity.Uuid.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- PUT /entities/corporations/{uuid} ---

func TestUpdateCorporation_401(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, nil, nil, &fakeCorporationService{}, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(map[string]string{"legal_name": "X"})
	req := httptest.NewRequest(http.MethodPut, "/entities/corporations/"+uuid.New().String(), bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateCorporation_403_NonAdmin(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 2, IsAdmin: false}, ok: true}
	corpSvc := &fakeCorporationService{err: service.ErrForbidden}
	d := buildTestDeps(ext, nil, nil, corpSvc, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(map[string]string{"legal_name": "X"})
	req := httptest.NewRequest(http.MethodPut, "/entities/corporations/"+uuid.New().String(), bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestUpdateCorporation_200(t *testing.T) {
	profile := buildCorpProfile()
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, IsAdmin: true}, ok: true}
	corpSvc := &fakeCorporationService{profile: profile}
	d := buildTestDeps(ext, nil, nil, corpSvc, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(map[string]string{"legal_name": "New Name"})
	req := httptest.NewRequest(http.MethodPut, "/entities/corporations/"+profile.Entity.Uuid.String(), bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- GET /entities/service-accounts/{uuid} ---

func TestGetServiceAccount_401(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, nil, nil, nil, &fakeServiceAccountService{})
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/service-accounts/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetServiceAccount_403_NonAdmin(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 2, IsAdmin: false}, ok: true}
	d := buildTestDeps(ext, nil, nil, nil, &fakeServiceAccountService{})
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/service-accounts/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGetServiceAccount_200(t *testing.T) {
	profile := buildSAProfile()
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, IsAdmin: true}, ok: true}
	saSvc := &fakeServiceAccountService{profile: profile}
	d := buildTestDeps(ext, nil, nil, nil, saSvc)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/service-accounts/"+profile.Entity.Uuid.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- PUT /entities/self (non natural_person) ---

func TestPutSelf_400_NotNaturalPerson(t *testing.T) {
	profile := buildCorpProfile()
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, EntityID: 1}, ok: true}
	entSvc := &fakeEntityService{profile: profile}
	d := buildTestDeps(ext, entSvc, nil, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(map[string]string{"given_name": "X"})
	req := httptest.NewRequest(http.MethodPut, "/entities/self", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- profileResponse SA branch ---

func TestProfileResponse_ServiceAccount(t *testing.T) {
	profile := buildSAProfile()
	resp := profileResponse(profile)
	if resp["kind"] != "service_account" {
		t.Errorf("kind: got %v", resp["kind"])
	}
	if resp["label"] != "my-svc" {
		t.Errorf("label: got %v", resp["label"])
	}
}
