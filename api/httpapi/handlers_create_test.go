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

// adminReq returns a request with actor entity ID 1 set in context.
func adminReq(method, url string, body *bytes.Buffer) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, body)
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	return withActor(req, 1)
}

// --- POST /entities/natural-persons ---

func TestCreateNaturalPerson_201(t *testing.T) {
	entityUUID := uuid.New()
	npResult := coredb.CreateNaturalPersonRow{
		GivenName:  pgtype.Text{String: "Alice", Valid: true},
		FamilyName: pgtype.Text{String: "Smith", Valid: true},
	}
	npSvc := &fakeNaturalPersonService{createNP: npResult, createUUID: entityUUID}
	d := buildTestDeps(nil, npSvc, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(createNaturalPersonRequest{GivenName: "Alice", FamilyName: "Smith"})
	req := adminReq(http.MethodPost, "/entities/natural-persons", bytes.NewBuffer(body))
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
	d := buildTestDeps(nil, &fakeNaturalPersonService{}, nil, nil)
	router := NewRouter(d)

	req := adminReq(http.MethodPost, "/entities/natural-persons", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateNaturalPerson_422_ServiceError(t *testing.T) {
	npSvc := &fakeNaturalPersonService{err: service.ErrInvalidInput}
	d := buildTestDeps(nil, npSvc, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(createNaturalPersonRequest{GivenName: "", FamilyName: ""})
	req := adminReq(http.MethodPost, "/entities/natural-persons", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- POST /entities/corporations ---

func TestCreateCorporation_201(t *testing.T) {
	entityUUID := uuid.New()
	corpResult := coredb.CreateCorporationRow{LegalName: "Acme Corp"}
	corpSvc := &fakeCorporationService{createCorp: corpResult, createUUID: entityUUID}
	d := buildTestDeps(nil, nil, corpSvc, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(createCorporationRequest{LegalName: "Acme Corp"})
	req := adminReq(http.MethodPost, "/entities/corporations", bytes.NewBuffer(body))
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
	d := buildTestDeps(nil, nil, &fakeCorporationService{}, nil)
	router := NewRouter(d)

	req := adminReq(http.MethodPost, "/entities/corporations", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateCorporation_403_NonAdmin(t *testing.T) {
	corpSvc := &fakeCorporationService{err: service.ErrForbidden}
	d := buildTestDeps(nil, nil, corpSvc, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(createCorporationRequest{LegalName: "X"})
	req := httptest.NewRequest(http.MethodPost, "/entities/corporations", bytes.NewBuffer(body))
	req = withActor(req, 2) // non-admin actor; authz denies via service
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
	d := buildTestDeps(nil, nil, nil, saSvc)
	router := NewRouter(d)

	body, _ := json.Marshal(createServiceAccountRequest{Label: "my-svc"})
	req := adminReq(http.MethodPost, "/entities/service-accounts", bytes.NewBuffer(body))
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
	d := buildTestDeps(nil, nil, nil, &fakeServiceAccountService{})
	router := NewRouter(d)

	req := adminReq(http.MethodPost, "/entities/service-accounts", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateServiceAccount_403_NonAdmin(t *testing.T) {
	saSvc := &fakeServiceAccountService{err: service.ErrForbidden}
	d := buildTestDeps(nil, nil, nil, saSvc)
	router := NewRouter(d)

	body, _ := json.Marshal(createServiceAccountRequest{Label: "x"})
	req := httptest.NewRequest(http.MethodPost, "/entities/service-accounts", bytes.NewBuffer(body))
	req = withActor(req, 2) // non-admin; authz denies via service
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// --- PUT /entities/service-accounts/{uuid} ---

func TestUpdateServiceAccount_401(t *testing.T) {
	d := buildTestDeps(nil, nil, nil, &fakeServiceAccountService{})
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
	d := buildTestDeps(nil, nil, nil, &fakeServiceAccountService{})
	router := NewRouter(d)

	req := adminReq(http.MethodPut, "/entities/service-accounts/"+uuid.New().String(), bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
