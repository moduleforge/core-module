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

// buildNaturalPersonProfile returns a Profile for testing GET responses.
func buildNaturalPersonProfile(givenName, familyName string) service.Profile {
	entityUUID := uuid.New()
	np := &coredb.NaturalPerson{
		GivenName:  pgtype.Text{String: givenName, Valid: true},
		FamilyName: pgtype.Text{String: familyName, Valid: true},
	}
	return service.Profile{
		Entity:        coredb.Entity{Uuid: entityUUID},
		Kind:          "natural_person",
		NaturalPerson: np,
	}
}

// --- GET /entities/self ---

func TestGetSelf_401_WhenNoPrincipal(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, &fakeEntityService{}, nil, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/self", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetSelf_200_HappyPath(t *testing.T) {
	profile := buildNaturalPersonProfile("Alice", "Smith")
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, EntityID: 1}, ok: true}
	entSvc := &fakeEntityService{profile: profile}
	d := buildTestDeps(ext, entSvc, nil, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/self", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["kind"] != "natural_person" {
		t.Errorf("kind: got %v, want natural_person", body["kind"])
	}
	if body["given_name"] != "Alice" {
		t.Errorf("given_name: got %v, want Alice", body["given_name"])
	}
}

// --- PUT /entities/self ---

func TestPutSelf_400_BadJSON(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, EntityID: 1}, ok: true}
	profile := buildNaturalPersonProfile("Alice", "Smith")
	entSvc := &fakeEntityService{profile: profile}
	npSvc := &fakeNaturalPersonService{profile: profile}
	d := buildTestDeps(ext, entSvc, npSvc, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodPut, "/entities/self", bytes.NewBufferString("not-json{{{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPutSelf_401_WhenNoPrincipal(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, &fakeEntityService{}, nil, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(map[string]string{"given_name": "Bob"})
	req := httptest.NewRequest(http.MethodPut, "/entities/self", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- POST /entities/natural-persons ---

func TestCreateNaturalPerson_403_NonAdmin(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 2, IsAdmin: false}, ok: true}
	d := buildTestDeps(ext, nil, &fakeNaturalPersonService{}, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(createNaturalPersonRequest{GivenName: "Bob", FamilyName: "Jones"})
	req := httptest.NewRequest(http.MethodPost, "/entities/natural-persons", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCreateNaturalPerson_401_Unauthenticated(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, nil, &fakeNaturalPersonService{}, nil, nil)
	router := NewRouter(d)

	body, _ := json.Marshal(createNaturalPersonRequest{GivenName: "Bob", FamilyName: "Jones"})
	req := httptest.NewRequest(http.MethodPost, "/entities/natural-persons", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- DELETE /entities/{uuid} ---

func TestArchiveEntity_401_Unauthenticated(t *testing.T) {
	ext := &fakePrincipalExtractor{ok: false}
	d := buildTestDeps(ext, &fakeEntityService{}, nil, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodDelete, "/entities/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestArchiveEntity_404_NotFound(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, IsAdmin: true}, ok: true}
	entSvc := &fakeEntityService{err: service.ErrNotFound}
	d := buildTestDeps(ext, entSvc, nil, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodDelete, "/entities/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestArchiveEntity_403_NonAdmin(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 2, IsAdmin: false}, ok: true}
	entSvc := &fakeEntityService{err: service.ErrForbidden}
	d := buildTestDeps(ext, entSvc, nil, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodDelete, "/entities/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestArchiveEntity_204_Success(t *testing.T) {
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1, IsAdmin: true}, ok: true}
	entSvc := &fakeEntityService{err: nil}
	d := buildTestDeps(ext, entSvc, nil, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodDelete, "/entities/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
}

// --- GET /entities/{uuid} ---

func TestGetEntity_200_HappyPath(t *testing.T) {
	profile := buildNaturalPersonProfile("Frank", "Zappa")
	ext := &fakePrincipalExtractor{p: &service.Principal{UserID: 1}, ok: true}
	entSvc := &fakeEntityService{profile: profile}
	d := buildTestDeps(ext, entSvc, nil, nil, nil)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- Response helpers ---

func TestWriteServiceErr_MapsCorrectly(t *testing.T) {
	cases := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{service.ErrNotFound, http.StatusNotFound, "not_found"},
		{service.ErrForbidden, http.StatusForbidden, "forbidden"},
		{service.ErrInvalidInput, http.StatusBadRequest, "invalid_input"},
	}

	for _, tc := range cases {
		t.Run(tc.wantCode, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeServiceErr(rec, tc.err)

			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}

			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body["error"] != tc.wantCode {
				t.Errorf("error code: got %q, want %q", body["error"], tc.wantCode)
			}
		})
	}
}
