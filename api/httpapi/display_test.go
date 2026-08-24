package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// decodeDisplayNameBody decodes rec's body into a map so tests can assert on
// whether the "display_name" key is present-and-null vs absent, which a
// typed struct decode cannot distinguish (both yield a nil pointer).
func decodeDisplayNameBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v (body: %s)", err, rec.Body.String())
	}
	return body
}

// --- GET /entities/{uuid}/display-name — fake-service tests ---

func TestGetDisplayName_200_RenderedName(t *testing.T) {
	dsp := &fakeDisplayService{name: "Ada Lovelace", available: true}
	d := buildTestDepsWithDisplay(nil, nil, nil, nil, dsp)
	router := NewRouter(d)

	target := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/entities/"+target.String()+"/display-name", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	body := decodeDisplayNameBody(t, rec)
	raw, ok := body["display_name"]
	if !ok {
		t.Fatal("display_name key missing from response")
	}
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		t.Fatalf("display_name not a string: %v", err)
	}
	if name != "Ada Lovelace" {
		t.Errorf("display_name: got %q, want %q", name, "Ada Lovelace")
	}

	uuidRaw, ok := body["uuid"]
	if !ok {
		t.Fatal("uuid key missing from response")
	}
	var gotUUID uuid.UUID
	if err := json.Unmarshal(uuidRaw, &gotUUID); err != nil {
		t.Fatalf("uuid not decodable: %v", err)
	}
	if gotUUID != target {
		t.Errorf("uuid: got %s, want %s", gotUUID, target)
	}
}

func TestGetDisplayName_200_NullWhenUnavailable(t *testing.T) {
	dsp := &fakeDisplayService{name: "", available: false}
	d := buildTestDepsWithDisplay(nil, nil, nil, nil, dsp)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String()+"/display-name", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	body := decodeDisplayNameBody(t, rec)
	raw, ok := body["display_name"]
	if !ok {
		t.Fatal("display_name key missing from response — must be present and null, not absent")
	}
	if string(raw) != "null" {
		t.Errorf("display_name: got %s, want null", raw)
	}
}

func TestGetDisplayName_200_NullWhenDisplayNil(t *testing.T) {
	// Built via the unchanged two-argument NewDeps, so Deps.Display is nil.
	svcs := &service.Services{}
	d := NewDeps(svcs, noopLogger())
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String()+"/display-name", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	body := decodeDisplayNameBody(t, rec)
	raw, ok := body["display_name"]
	if !ok {
		t.Fatal("display_name key missing from response — must be present and null, not absent")
	}
	if string(raw) != "null" {
		t.Errorf("display_name: got %s, want null", raw)
	}
}

func TestGetDisplayName_401_Unauthenticated(t *testing.T) {
	dsp := &fakeDisplayService{name: "Ada Lovelace", available: true}
	d := buildTestDepsWithDisplay(nil, nil, nil, nil, dsp)
	router := NewRouter(d)

	// No actor in context.
	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String()+"/display-name", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetDisplayName_400_InvalidUUID(t *testing.T) {
	dsp := &fakeDisplayService{name: "Ada Lovelace", available: true}
	d := buildTestDepsWithDisplay(nil, nil, nil, nil, dsp)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/not-a-uuid/display-name", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetDisplayName_500_UnmappedError(t *testing.T) {
	dsp := &fakeDisplayService{err: errors.New("boom: raw db failure detail")}
	d := buildTestDepsWithDisplay(nil, nil, nil, nil, dsp)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String()+"/display-name", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("error code: got %q, want %q", body.Error.Code, "internal_error")
	}
	if strings.Contains(rec.Body.String(), "boom: raw db failure detail") {
		t.Errorf("response body leaked raw error text: %s", rec.Body.String())
	}
}

func TestGetDisplayName_403_ForbiddenError(t *testing.T) {
	dsp := &fakeDisplayService{err: apiresp.ErrForbidden}
	d := buildTestDepsWithDisplay(nil, nil, nil, nil, dsp)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String()+"/display-name", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "forbidden" {
		t.Errorf("error code: got %q, want %q", body.Error.Code, "forbidden")
	}
}

// --- GET /entities/{uuid}/display-name — end-to-end, real collaborators ---
//
// Following masked_lookup_test.go's precedent, this wires a real
// service.DisplayService (backed by a real service.NewDisplayRegistry) and a
// real router via NewDepsWithDisplay, instead of the fakeDisplayService used
// above, so the full resolve -> authorize -> render chain (and the masked
// 403 it produces for an unseeded UUID) is exercised end-to-end through
// NewRouter, not just the handler function.
func TestGetDisplayName_EndToEnd_RealRegistry(t *testing.T) {
	npUUID := uuid.New()
	corpUUID := uuid.New()
	saUUID := uuid.New()
	noRendererUUID := uuid.New()

	q := &stubQuerier{
		entitiesByUUID: map[uuid.UUID]coredb.GetEntityByUUIDRow{
			npUUID:         {ID: 1, Uuid: npUUID, FundamentalTypeSlug: "natural_person"},
			corpUUID:       {ID: 2, Uuid: corpUUID, FundamentalTypeSlug: "corporation"},
			saUUID:         {ID: 3, Uuid: saUUID, FundamentalTypeSlug: "service_account"},
			noRendererUUID: {ID: 4, Uuid: noRendererUUID, FundamentalTypeSlug: "widget"},
		},
		entitiesByID: map[int64]coredb.GetEntityByIDRow{
			1: {ID: 1, Uuid: npUUID, FundamentalTypeSlug: "natural_person"},
			2: {ID: 2, Uuid: corpUUID, FundamentalTypeSlug: "corporation"},
			3: {ID: 3, Uuid: saUUID, FundamentalTypeSlug: "service_account"},
			4: {ID: 4, Uuid: noRendererUUID, FundamentalTypeSlug: "widget"},
		},
		naturalPersonsByEntityID: map[int64]coredb.GetNaturalPersonByEntityIDRow{
			1: {ID: 1, EntityID: 1, GivenName: pgtype.Text{String: "Ada", Valid: true}, FamilyName: pgtype.Text{String: "Lovelace", Valid: true}},
		},
		corporationsByEntityID: map[int64]coredb.GetCorporationByEntityIDRow{
			2: {ID: 2, EntityID: 2, LegalName: "Acme Corp"},
		},
		serviceAccountsByEntityID: map[int64]coredb.ServiceAccount{
			3: {ID: 3, EntityID: 3, Label: "ci-bot"},
		},
	}

	reg := service.NewDisplayRegistry(q)
	dsp := service.NewDisplayService(reg, &stubAuthorizer{}, entity.NewResolver())
	svcs := service.New(q, nil, &stubAuthorizer{}, observer.NewObserverGroup(), nil, entity.NewResolver(), nil)
	d := NewDepsWithDisplay(svcs, noopLogger(), dsp)
	router := NewRouter(d)

	getDisplayName := func(target uuid.UUID) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/entities/"+target.String()+"/display-name", nil)
		req = withActor(req, 1)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("natural_person renders given+family name", func(t *testing.T) {
		rec := getDisplayName(npUUID)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		body := decodeDisplayNameBody(t, rec)
		var name string
		if err := json.Unmarshal(body["display_name"], &name); err != nil {
			t.Fatalf("display_name not a string: %v (body: %s)", err, rec.Body.String())
		}
		if name != "Ada Lovelace" {
			t.Errorf("display_name: got %q, want %q", name, "Ada Lovelace")
		}
	})

	t.Run("corporation renders legal name", func(t *testing.T) {
		rec := getDisplayName(corpUUID)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		body := decodeDisplayNameBody(t, rec)
		var name string
		if err := json.Unmarshal(body["display_name"], &name); err != nil {
			t.Fatalf("display_name not a string: %v (body: %s)", err, rec.Body.String())
		}
		if name != "Acme Corp" {
			t.Errorf("display_name: got %q, want %q", name, "Acme Corp")
		}
	})

	t.Run("service_account renders label", func(t *testing.T) {
		rec := getDisplayName(saUUID)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		body := decodeDisplayNameBody(t, rec)
		var name string
		if err := json.Unmarshal(body["display_name"], &name); err != nil {
			t.Fatalf("display_name not a string: %v (body: %s)", err, rec.Body.String())
		}
		if name != "ci-bot" {
			t.Errorf("display_name: got %q, want %q", name, "ci-bot")
		}
	})

	t.Run("readable entity with no registered renderer returns null", func(t *testing.T) {
		rec := getDisplayName(noRendererUUID)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		body := decodeDisplayNameBody(t, rec)
		raw, ok := body["display_name"]
		if !ok {
			t.Fatal("display_name key missing from response — must be present and null, not absent")
		}
		if string(raw) != "null" {
			t.Errorf("display_name: got %s, want null", raw)
		}
	})

	// Mirrors TestGetEntity_MaskedMiss_Returns403Forbidden: a random,
	// unseeded UUID against the same real chain proves the resolver's
	// masked-403 propagates through this endpoint too, not just getEntity.
	t.Run("unseeded UUID returns masked 403", func(t *testing.T) {
		rec := getDisplayName(uuid.New())
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status: got %d, want %d (body: %s)", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Error.Code != "forbidden" {
			t.Errorf("error code: got %q, want %q", body.Error.Code, "forbidden")
		}
	})
}
