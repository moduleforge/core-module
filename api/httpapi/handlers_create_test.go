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

func adminPrincipal() *fakePrincipalExtractor {
	return &fakePrincipalExtractor{p: &service.Principal{UserID: 1, IsAdmin: true}, ok: true}
}

func fakeTxOK() *fakeTxBeginner {
	return &fakeTxBeginner{tx: &fakeTx{}}
}

// --- POST /entities/natural-persons ---

func TestCreateNaturalPerson_201(t *testing.T) {
	entityUUID := uuid.New()
	npResult := coredb.NaturalPerson{
		GivenName:  pgtype.Text{String: "Alice", Valid: true},
		FamilyName: pgtype.Text{String: "Smith", Valid: true},
	}
	npSvc := &fakeNaturalPersonService{createNP: npResult, createUUID: entityUUID}
	d := buildTestDepsWithTx(adminPrincipal(), nil, npSvc, nil, nil, fakeTxOK())
	router := NewRouter(d)

	body, _ := json.Marshal(createNaturalPersonRequest{GivenName: "Alice", FamilyName: "Smith"})
	req := httptest.NewRequest(http.MethodPost, "/entities/natural-persons", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["uuid"] != entityUUID.String() {
		t.Errorf("uuid: got %v, want %v", resp["uuid"], entityUUID.String())
	}
	if resp["given_name"] != "Alice" {
		t.Errorf("given_name: got %v", resp["given_name"])
	}
}

func TestCreateNaturalPerson_400_BadJSON(t *testing.T) {
	d := buildTestDepsWithTx(adminPrincipal(), nil, &fakeNaturalPersonService{}, nil, nil, fakeTxOK())
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodPost, "/entities/natural-persons", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateNaturalPerson_422_ServiceError(t *testing.T) {
	npSvc := &fakeNaturalPersonService{err: service.ErrInvalidInput}
	d := buildTestDepsWithTx(adminPrincipal(), nil, npSvc, nil, nil, fakeTxOK())
	router := NewRouter(d)

	body, _ := json.Marshal(createNaturalPersonRequest{GivenName: "", FamilyName: ""})
	req := httptest.NewRequest(http.MethodPost, "/entities/natural-persons", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- POST /entities/corporations ---

func TestCreateCorporation_201(t *testing.T) {
	entityUUID := uuid.New()
	corpResult := coredb.Corporation{LegalName: "Acme Corp"}
	corpSvc := &fakeCorporationService{createCorp: corpResult, createUUID: entityUUID}
	d := buildTestDepsWithTx(adminPrincipal(), nil, nil, corpSvc, nil, fakeTxOK())
	router := NewRouter(d)

	body, _ := json.Marshal(createCorporationRequest{LegalName: "Acme Corp"})
	req := httptest.NewRequest(http.MethodPost, "/entities/corporations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["legal_name"] != "Acme Corp" {
		t.Errorf("legal_name: got %v", resp["legal_name"])
	}
}

func TestCreateCorporation_400_BadJSON(t *testing.T) {
	d := buildTestDepsWithTx(adminPrincipal(), nil, nil, &fakeCorporationService{}, nil, fakeTxOK())
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodPost, "/entities/corporations", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateCorporation_403_NonAdmin(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 2, IsAdmin: false}, ok: true}
	d := buildTestDepsWithTx(ext, nil, nil, &fakeCorporationService{}, nil, fakeTxOK())
	router := NewRouter(d)

	body, _ := json.Marshal(createCorporationRequest{LegalName: "X"})
	req := httptest.NewRequest(http.MethodPost, "/entities/corporations", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// --- POST /entities/service-accounts ---

func TestCreateServiceAccount_201(t *testing.T) {
	entityUUID := uuid.New()
	saResult := coredb.ServiceAccount{Label: "my-svc"}
	saSvc := &fakeServiceAccountService{createSA: saResult, createUUID: entityUUID}
	d := buildTestDepsWithTx(adminPrincipal(), nil, nil, nil, saSvc, fakeTxOK())
	router := NewRouter(d)

	body, _ := json.Marshal(createServiceAccountRequest{Label: "my-svc"})
	req := httptest.NewRequest(http.MethodPost, "/entities/service-accounts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["label"] != "my-svc" {
		t.Errorf("label: got %v", resp["label"])
	}
}

func TestCreateServiceAccount_400_BadJSON(t *testing.T) {
	d := buildTestDepsWithTx(adminPrincipal(), nil, nil, nil, &fakeServiceAccountService{}, fakeTxOK())
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodPost, "/entities/service-accounts", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateServiceAccount_403_NonAdmin(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 2, IsAdmin: false}, ok: true}
	d := buildTestDepsWithTx(ext, nil, nil, nil, &fakeServiceAccountService{}, fakeTxOK())
	router := NewRouter(d)

	body, _ := json.Marshal(createServiceAccountRequest{Label: "x"})
	req := httptest.NewRequest(http.MethodPost, "/entities/service-accounts", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// --- PUT /entities/service-accounts/{uuid} ---

func TestUpdateServiceAccount_401(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, nil, nil, nil, &fakeServiceAccountService{})
	router := NewRouter(d)

	body, _ := json.Marshal(map[string]string{"label": "x"})
	req := httptest.NewRequest(http.MethodPut, "/entities/service-accounts/"+uuid.New().String(), bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateServiceAccount_400_BadJSON(t *testing.T) {
	d := buildTestDeps(adminPrincipal(), nil, nil, nil, &fakeServiceAccountService{})
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodPut, "/entities/service-accounts/"+uuid.New().String(), bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
