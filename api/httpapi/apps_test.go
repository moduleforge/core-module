package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	coredb "github.com/moduleforge/core-model/db"
)

// --- fake txhelper.DB / pgx.Tx for AppsHandler tests ---

// fakeAppsTx is a minimal pgx.Tx that satisfies the interface. AppsHandler
// tests wire appsFakeQuerier into the handler's newQuerier field directly,
// so the SQL-shaped methods below are never called.
type fakeAppsTx struct {
	commitErr   error
	rollbackErr error
	committed   bool
	rolledBack  bool
}

func (t *fakeAppsTx) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }
func (t *fakeAppsTx) Commit(_ context.Context) error {
	t.committed = true
	return t.commitErr
}
func (t *fakeAppsTx) Rollback(_ context.Context) error {
	t.rolledBack = true
	return t.rollbackErr
}
func (t *fakeAppsTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeAppsTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *fakeAppsTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (t *fakeAppsTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeAppsTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *fakeAppsTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (t *fakeAppsTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row        { return nil }
func (t *fakeAppsTx) Conn() *pgx.Conn                                               { return nil }

var _ pgx.Tx = (*fakeAppsTx)(nil)

// fakeAppsDB implements txhelper.DB, always returning the same fakeAppsTx.
type fakeAppsDB struct {
	tx  pgx.Tx
	err error
}

func (d *fakeAppsDB) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return d.tx, d.err
}

func newFakeAppsDB() *fakeAppsDB {
	return &fakeAppsDB{tx: &fakeAppsTx{}}
}

// --- fake coredb.Querier for AppsHandler tests ---

// appsFakeQuerier implements coredb.Querier with working in-memory behaviour
// for the entity + apps operations AppsHandler exercises (GetTypeBySlug,
// CreateEntity, InsertApp, GetAppByUUID, ListApps, UpdateApp, ArchiveEntity).
// All other methods are no-ops; AppsHandler never calls them.
type appsFakeQuerier struct {
	nextID   int64
	entities map[int64]coredb.Entity
	apps     map[int64]coredb.GetAppByUUIDRow // keyed by id (== entity id)

	getTypeErr      error
	createEntityErr error
	insertAppErr    error
	updateAppErr    error
	archiveErr      error
	getAppByUUIDErr error
	listAppsErr     error
}

func newAppsFakeQuerier() *appsFakeQuerier {
	return &appsFakeQuerier{
		entities: make(map[int64]coredb.Entity),
		apps:     make(map[int64]coredb.GetAppByUUIDRow),
	}
}

func (q *appsFakeQuerier) nextSeq() int64 {
	q.nextID++
	return q.nextID
}

// seedApp inserts an app directly (bypassing Create) and returns its uuid.
func (q *appsFakeQuerier) seedApp(slug, name string) uuid.UUID {
	id := q.nextSeq()
	appUUID := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	q.apps[id] = coredb.GetAppByUUIDRow{
		ID: id, Uuid: appUUID, Slug: slug, Name: name,
		CreatedAt: now, UpdatedAt: now,
	}
	return appUUID
}

func (q *appsFakeQuerier) GetTypeBySlug(_ context.Context, slug string) (coredb.Type, error) {
	if q.getTypeErr != nil {
		return coredb.Type{}, q.getTypeErr
	}
	return coredb.Type{ID: 99, Slug: slug, Concrete: true, Name: slug}, nil
}

func (q *appsFakeQuerier) CreateEntity(_ context.Context, fundamentalTypeID int64) (coredb.Entity, error) {
	if q.createEntityErr != nil {
		return coredb.Entity{}, q.createEntityErr
	}
	id := q.nextSeq()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	ent := coredb.Entity{ID: id, Uuid: uuid.New(), FundamentalTypeID: fundamentalTypeID, CreatedAt: now, UpdatedAt: now}
	q.entities[id] = ent
	return ent, nil
}

func (q *appsFakeQuerier) InsertApp(_ context.Context, arg coredb.InsertAppParams) (coredb.App, error) {
	if q.insertAppErr != nil {
		return coredb.App{}, q.insertAppErr
	}
	ent := q.entities[arg.ID]
	q.apps[arg.ID] = coredb.GetAppByUUIDRow{
		ID: arg.ID, Uuid: ent.Uuid, Slug: arg.Slug, Name: arg.Name,
		CreatedAt: ent.CreatedAt, UpdatedAt: ent.UpdatedAt,
	}
	return coredb.App{ID: arg.ID, Slug: arg.Slug, Name: arg.Name}, nil
}

func (q *appsFakeQuerier) GetAppByUUID(_ context.Context, argUuid uuid.UUID) (coredb.GetAppByUUIDRow, error) {
	if q.getAppByUUIDErr != nil {
		return coredb.GetAppByUUIDRow{}, q.getAppByUUIDErr
	}
	for _, a := range q.apps {
		if a.Uuid == argUuid {
			return a, nil
		}
	}
	return coredb.GetAppByUUIDRow{}, pgx.ErrNoRows
}

func (q *appsFakeQuerier) GetAppBySlug(_ context.Context, slug string) (coredb.GetAppBySlugRow, error) {
	for _, a := range q.apps {
		if a.Slug == slug {
			return coredb.GetAppBySlugRow{
				ID: a.ID, Uuid: a.Uuid, Slug: a.Slug, Name: a.Name,
				CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, ArchivedAt: a.ArchivedAt,
			}, nil
		}
	}
	return coredb.GetAppBySlugRow{}, pgx.ErrNoRows
}

func (q *appsFakeQuerier) ListApps(_ context.Context) ([]coredb.ListAppsRow, error) {
	if q.listAppsErr != nil {
		return nil, q.listAppsErr
	}
	out := make([]coredb.ListAppsRow, 0, len(q.apps))
	for _, a := range q.apps {
		if a.ArchivedAt != nil {
			continue
		}
		out = append(out, coredb.ListAppsRow{
			ID: a.ID, Uuid: a.Uuid, Slug: a.Slug, Name: a.Name,
			CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, ArchivedAt: a.ArchivedAt,
		})
	}
	return out, nil
}

func (q *appsFakeQuerier) UpdateApp(_ context.Context, arg coredb.UpdateAppParams) error {
	if q.updateAppErr != nil {
		return q.updateAppErr
	}
	if a, ok := q.apps[arg.ID]; ok {
		a.Slug = arg.Slug
		a.Name = arg.Name
		q.apps[arg.ID] = a
	}
	return nil
}

func (q *appsFakeQuerier) ArchiveEntity(_ context.Context, argUuid uuid.UUID) error {
	if q.archiveErr != nil {
		return q.archiveErr
	}
	for id, a := range q.apps {
		if a.Uuid == argUuid {
			now := time.Now()
			a.ArchivedAt = &now
			q.apps[id] = a
		}
	}
	return nil
}

// --- required no-op stubs to satisfy coredb.Querier ---

func (q *appsFakeQuerier) CreateCorporation(_ context.Context, _ coredb.CreateCorporationParams) (coredb.CreateCorporationRow, error) {
	return coredb.CreateCorporationRow{}, nil
}
func (q *appsFakeQuerier) CreateEntityWithOwner(_ context.Context, _ coredb.CreateEntityWithOwnerParams) (coredb.Entity, error) {
	return coredb.Entity{}, nil
}
func (q *appsFakeQuerier) CreateLegalEntity(_ context.Context, _ int64) (int64, error) { return 0, nil }
func (q *appsFakeQuerier) CreateNaturalPerson(_ context.Context, _ coredb.CreateNaturalPersonParams) (coredb.CreateNaturalPersonRow, error) {
	return coredb.CreateNaturalPersonRow{}, nil
}
func (q *appsFakeQuerier) CreateServiceAccount(_ context.Context, _ coredb.CreateServiceAccountParams) (coredb.ServiceAccount, error) {
	return coredb.ServiceAccount{}, nil
}
func (q *appsFakeQuerier) GetCorporationByEntityID(_ context.Context, _ int64) (coredb.GetCorporationByEntityIDRow, error) {
	return coredb.GetCorporationByEntityIDRow{}, nil
}
func (q *appsFakeQuerier) GetEntityByID(_ context.Context, _ int64) (coredb.GetEntityByIDRow, error) {
	return coredb.GetEntityByIDRow{}, nil
}
func (q *appsFakeQuerier) GetEntityByUUID(_ context.Context, _ uuid.UUID) (coredb.GetEntityByUUIDRow, error) {
	return coredb.GetEntityByUUIDRow{}, nil
}
func (q *appsFakeQuerier) GetLegalEntityByEntityID(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (q *appsFakeQuerier) GetNaturalPersonByEntityID(_ context.Context, _ int64) (coredb.GetNaturalPersonByEntityIDRow, error) {
	return coredb.GetNaturalPersonByEntityIDRow{}, nil
}
func (q *appsFakeQuerier) GetServiceAccountByEntityID(_ context.Context, _ int64) (coredb.ServiceAccount, error) {
	return coredb.ServiceAccount{}, nil
}
func (q *appsFakeQuerier) GetTypeByID(_ context.Context, _ int64) (coredb.Type, error) {
	return coredb.Type{}, nil
}
func (q *appsFakeQuerier) ListAllTypes(_ context.Context) ([]coredb.Type, error) { return nil, nil }
func (q *appsFakeQuerier) UnarchiveEntity(_ context.Context, _ uuid.UUID) error  { return nil }
func (q *appsFakeQuerier) UpdateCorporation(_ context.Context, _ coredb.UpdateCorporationParams) error {
	return nil
}
func (q *appsFakeQuerier) UpdateNaturalPerson(_ context.Context, _ coredb.UpdateNaturalPersonParams) error {
	return nil
}

var _ coredb.Querier = (*appsFakeQuerier)(nil)

// --- fake authz.Authorizer for AppsHandler tests ---

type appsFakeAuthorizer struct {
	err error
}

func (a *appsFakeAuthorizer) Authorize(_ context.Context, _ string, _ *int64) error {
	return a.err
}

// --- test helpers ---

// newTestAppsHandler wires an AppsHandler whose newQuerier always returns q,
// bypassing real SQL/tx routing (mirrors service/mock_test.go's
// mockQuerierFactory pattern in this repo's other test package).
func newTestAppsHandler(q *appsFakeQuerier, az *appsFakeAuthorizer) *AppsHandler {
	h := NewAppsHandler(newFakeAppsDB(), q, az, observer.NewObserverGroup(), entity.NewResolver())
	h.newQuerier = func(_ pgx.Tx) coredb.Querier { return q }
	return h
}

func appsRouter(h *AppsHandler) chi.Router {
	r := chi.NewRouter()
	RegisterAppRoutes(r, h)
	return r
}

// --- POST /apps ---

func TestAppsCreate_201(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	body, _ := json.Marshal(createAppRequest{Slug: "demo", Name: "Demo App"})
	req := adminReq(http.MethodPost, "/apps", bytes.NewBuffer(body))
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
	if resp["slug"] != "demo" {
		t.Errorf("slug: got %v, want %q", resp["slug"], "demo")
	}
	if resp["name"] != "Demo App" {
		t.Errorf("name: got %v, want %q", resp["name"], "Demo App")
	}
	respUUID, ok := resp["uuid"].(string)
	if !ok || respUUID == "" {
		t.Fatalf("uuid missing or not a string: %v", resp["uuid"])
	}
	if resp["created_at"] == nil {
		t.Errorf("created_at: missing")
	}

	// The created uuid must be the entity's uuid, and a subsequent GET by
	// that uuid must return the same app (manual reasoning check from the
	// task doc's Validation section).
	getReq := adminReq(http.MethodGet, "/apps/"+respUUID, nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET after create: status got %d, want %d — body: %s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var getResp map[string]any
	if err := json.NewDecoder(getRec.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	if getResp["uuid"] != respUUID {
		t.Errorf("GET uuid: got %v, want %v", getResp["uuid"], respUUID)
	}
}

func TestAppsCreate_400_BadJSON(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	req := adminReq(http.MethodPost, "/apps", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAppsCreate_400_MissingSlug(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	body, _ := json.Marshal(createAppRequest{Slug: "  ", Name: "Demo App"})
	req := adminReq(http.MethodPost, "/apps", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAppsCreate_400_MissingName(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	body, _ := json.Marshal(createAppRequest{Slug: "demo", Name: " "})
	req := adminReq(http.MethodPost, "/apps", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAppsCreate_401_Unauthenticated(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	body, _ := json.Marshal(createAppRequest{Slug: "demo", Name: "Demo App"})
	req := httptest.NewRequest(http.MethodPost, "/apps", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAppsCreate_403_Forbidden(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{err: apiresp.ErrForbidden})
	router := appsRouter(h)

	body, _ := json.Marshal(createAppRequest{Slug: "demo", Name: "Demo App"})
	req := adminReq(http.MethodPost, "/apps", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// --- GET /apps ---

func TestAppsList_200(t *testing.T) {
	q := newAppsFakeQuerier()
	q.seedApp("alpha", "Alpha")
	q.seedApp("beta", "Beta")
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	req := adminReq(http.MethodGet, "/apps", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Apps []map[string]any `json:"apps"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Apps) != 2 {
		t.Errorf("apps count: got %d, want 2", len(resp.Apps))
	}
}

func TestAppsList_ExcludesArchived(t *testing.T) {
	q := newAppsFakeQuerier()
	appUUID := q.seedApp("gamma", "Gamma")
	q.seedApp("delta", "Delta")
	if err := q.ArchiveEntity(context.Background(), appUUID); err != nil {
		t.Fatalf("seed archive: %v", err)
	}
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	req := adminReq(http.MethodGet, "/apps", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp struct {
		Apps []map[string]any `json:"apps"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Apps) != 1 {
		t.Fatalf("apps count: got %d, want 1 (archived app should be excluded)", len(resp.Apps))
	}
	if resp.Apps[0]["slug"] != "delta" {
		t.Errorf("remaining app slug: got %v, want %q", resp.Apps[0]["slug"], "delta")
	}
}

// --- GET /apps/{uuid} ---

func TestAppsGetApp_200(t *testing.T) {
	q := newAppsFakeQuerier()
	appUUID := q.seedApp("demo", "Demo App")
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	req := adminReq(http.MethodGet, "/apps/"+appUUID.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["uuid"] != appUUID.String() {
		t.Errorf("uuid: got %v, want %v", resp["uuid"], appUUID.String())
	}
}

func TestAppsGetApp_404_NotFound(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	req := adminReq(http.MethodGet, "/apps/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAppsGetApp_400_BadUUID(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	req := adminReq(http.MethodGet, "/apps/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- PUT /apps/{uuid} ---

func TestAppsUpdateApp_200(t *testing.T) {
	q := newAppsFakeQuerier()
	appUUID := q.seedApp("demo", "Demo App")
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	body, _ := json.Marshal(updateAppRequest{Name: "Renamed App"})
	req := adminReq(http.MethodPut, "/apps/"+appUUID.String(), bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["name"] != "Renamed App" {
		t.Errorf("name: got %v, want %q", resp["name"], "Renamed App")
	}
	if resp["slug"] != "demo" {
		t.Errorf("slug should be unchanged: got %v, want %q", resp["slug"], "demo")
	}
}

func TestAppsUpdateApp_404_NotFound(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	body, _ := json.Marshal(updateAppRequest{Name: "Renamed App"})
	req := adminReq(http.MethodPut, "/apps/"+uuid.New().String(), bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- DELETE /apps/{uuid} ---

func TestAppsDeleteApp_204_ArchivesAndDropsFromList(t *testing.T) {
	q := newAppsFakeQuerier()
	appUUID := q.seedApp("demo", "Demo App")
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	req := adminReq(http.MethodDelete, "/apps/"+appUUID.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	// Validation criterion: archive sets entities.archived_at and the app
	// drops out of ListApps.
	listReq := adminReq(http.MethodGet, "/apps", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	var listResp struct {
		Apps []map[string]any `json:"apps"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Apps) != 0 {
		t.Errorf("apps after archive: got %d, want 0", len(listResp.Apps))
	}
}

func TestAppsDeleteApp_404_NotFound(t *testing.T) {
	q := newAppsFakeQuerier()
	h := newTestAppsHandler(q, &appsFakeAuthorizer{})
	router := appsRouter(h)

	req := adminReq(http.MethodDelete, "/apps/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
