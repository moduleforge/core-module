package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// TestGetEntity_MaskedMiss_Returns403Forbidden is an httpapi-layer test that
// wires the real service.EntityService (via service.New, with a real
// entity.NewResolver()) into the router instead of a fake*Service. Every
// other httpapi test in this package injects fake*Service implementations
// that return a caller-supplied err directly, bypassing entity.Resolver
// entirely — they cannot exercise (and could not have caught) the
// masked-lookup bug fixed by aliasing entity.ErrForbidden/ErrNotFound onto
// the canonical apiresp sentinels. This test proves the fix end-to-end
// through the real resolver -> service -> apiresp.WriteError chain.
func TestGetEntity_MaskedMiss_Returns403Forbidden(t *testing.T) {
	q := &stubQuerier{getEntityErr: pgx.ErrNoRows}
	svcs := service.New(q, nil, &stubAuthorizer{}, observer.NewObserverGroup(), nil, entity.NewResolver(), nil)
	d := Deps{Services: svcs, Logger: noopLogger()}
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String(), nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusForbidden)
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
	if body.Error.Code != "forbidden" {
		t.Errorf("error code: got %q, want %q", body.Error.Code, "forbidden")
	}
}

// --- minimal stub Querier ---
//
// stubQuerier implements coredb.Querier with controllable GetEntityByUUID
// behaviour. All other methods are no-ops; they are never invoked on the
// masked-miss code path this test exercises. Mirrors the shape of
// api/entity/resolver_test.go's resolverStubQuerier, which lives in package
// entity_test and cannot be imported directly here.
type stubQuerier struct {
	getEntityErr error
}

func (s *stubQuerier) GetEntityByUUID(_ context.Context, _ uuid.UUID) (coredb.GetEntityByUUIDRow, error) {
	return coredb.GetEntityByUUIDRow{}, s.getEntityErr
}

func (s *stubQuerier) ArchiveEntity(_ context.Context, _ uuid.UUID) error { return nil }
func (s *stubQuerier) CreateCorporation(_ context.Context, _ coredb.CreateCorporationParams) (coredb.CreateCorporationRow, error) {
	return coredb.CreateCorporationRow{}, nil
}
func (s *stubQuerier) CreateEntity(_ context.Context, _ int64) (coredb.Entity, error) {
	return coredb.Entity{}, nil
}
func (s *stubQuerier) CreateEntityWithOwner(_ context.Context, _ coredb.CreateEntityWithOwnerParams) (coredb.Entity, error) {
	return coredb.Entity{}, nil
}
func (s *stubQuerier) CreateLegalEntity(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (s *stubQuerier) CreateNaturalPerson(_ context.Context, _ coredb.CreateNaturalPersonParams) (coredb.CreateNaturalPersonRow, error) {
	return coredb.CreateNaturalPersonRow{}, nil
}
func (s *stubQuerier) CreateServiceAccount(_ context.Context, _ coredb.CreateServiceAccountParams) (coredb.ServiceAccount, error) {
	return coredb.ServiceAccount{}, nil
}
func (s *stubQuerier) GetCorporationByEntityID(_ context.Context, _ int64) (coredb.GetCorporationByEntityIDRow, error) {
	return coredb.GetCorporationByEntityIDRow{}, nil
}
func (s *stubQuerier) GetEntityByID(_ context.Context, _ int64) (coredb.GetEntityByIDRow, error) {
	return coredb.GetEntityByIDRow{}, nil
}
func (s *stubQuerier) GetLegalEntityByEntityID(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (s *stubQuerier) GetNaturalPersonByEntityID(_ context.Context, _ int64) (coredb.GetNaturalPersonByEntityIDRow, error) {
	return coredb.GetNaturalPersonByEntityIDRow{}, nil
}
func (s *stubQuerier) GetServiceAccountByEntityID(_ context.Context, _ int64) (coredb.ServiceAccount, error) {
	return coredb.ServiceAccount{}, nil
}
func (s *stubQuerier) GetTypeByID(_ context.Context, _ int64) (coredb.Type, error) {
	return coredb.Type{}, nil
}
func (s *stubQuerier) GetTypeBySlug(_ context.Context, _ string) (coredb.Type, error) {
	return coredb.Type{}, nil
}
func (s *stubQuerier) ListAllTypes(_ context.Context) ([]coredb.Type, error) {
	return nil, nil
}
func (s *stubQuerier) UnarchiveEntity(_ context.Context, _ uuid.UUID) error { return nil }
func (s *stubQuerier) UpdateCorporation(_ context.Context, _ coredb.UpdateCorporationParams) error {
	return nil
}
func (s *stubQuerier) UpdateNaturalPerson(_ context.Context, _ coredb.UpdateNaturalPersonParams) error {
	return nil
}

var _ coredb.Querier = (*stubQuerier)(nil)

// --- minimal stub Authorizer ---
//
// stubAuthorizer always allows. It is not actually reached on the
// masked-miss path exercised above, since EntityService.GetByUUID /
// ResolveProfile call the resolver before authorization, but service.New
// requires a non-nil authz.Authorizer to type-check the aggregate, and a
// real request that does resolve should not spuriously fail authz.
type stubAuthorizer struct{}

func (stubAuthorizer) Authorize(_ context.Context, _ string, _ *int64) error {
	return nil
}
