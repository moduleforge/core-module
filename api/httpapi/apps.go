package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/authz"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/core-api/txhelper"
	coredb "github.com/moduleforge/core-model/db"
)

// AppsHandler serves the top-level /apps CRUD endpoints: Create (POST
// /apps), List (GET /apps), GetApp (GET /apps/{uuid}), UpdateApp (PUT
// /apps/{uuid}), and DeleteApp (DELETE /apps/{uuid}, archive).
//
// Apps are entities (fundamental_type = "app"): creation inserts an
// entities row and an apps row keyed by that entity id in a single
// transaction, and archival uses the generic entities.archived_at column
// rather than a per-table flag. This closes the audit gap the original
// mod-users handler carried: that handler passed a nil target_entity_id
// with a comment noting apps were not registered in core's entities table.
// Now that apps are entities, mutations carry the app's entity id as the
// observer target.
//
// Unlike the natural-person/corporation/service-account handlers in this
// package, AppsHandler does not sit on the service.Services aggregate: apps
// have no Profile-shaped read model and no encrypted fields, so a
// dedicated, lighter-weight handler (mirroring mod-users' original
// AppsHandler shape) avoids pulling in cipher/typeResolver dependencies
// apps do not need.
//
// Membership management (assigning/removing users, editing roles) is not
// part of this handler — those operations stay in mod-users (Phase 02 of
// the apps-to-core migration) since they operate on mod-users' own
// apps_user_accounts join table.
type AppsHandler struct {
	pool       txhelper.DB
	q          coredb.Querier
	newQuerier func(pgx.Tx) coredb.Querier // factory for tx-scoped querier; defaults to coredb.New
	az         authz.Authorizer
	observers  *observer.ObserverGroup
	// entityResolver is accepted so NewAppsHandler's signature lines up with
	// the rest of core's manifest-wired constructors (queries:coredb,
	// service:authorizer, service:observerGroup, service:entityResolver).
	// GetApp/UpdateApp/DeleteApp resolve {uuid} via the GetAppByUUID join
	// query instead of a separate entityResolver.Resolve call: that single
	// round trip already returns the row's id (== the entity id, per the
	// apps FK-anchor pattern) alongside slug/name, so a second lookup would
	// add a query without adding information.
	entityResolver *entity.Resolver
}

// NewAppsHandler constructs an AppsHandler. Args are ordered to match the
// module manifest's arg-source list: infra:pool, queries:coredb,
// service:authorizer, service:observerGroup, service:entityResolver.
func NewAppsHandler(
	pool txhelper.DB,
	q coredb.Querier,
	az authz.Authorizer,
	observers *observer.ObserverGroup,
	entityResolver *entity.Resolver,
) *AppsHandler {
	return &AppsHandler{
		pool:           pool,
		q:              q,
		newQuerier:     func(tx pgx.Tx) coredb.Querier { return coredb.New(tx) },
		az:             az,
		observers:      observers,
		entityResolver: entityResolver,
	}
}

// RegisterAppRoutes mounts the top-level /apps CRUD routes on r.
func RegisterAppRoutes(r chi.Router, h *AppsHandler) {
	r.Post("/apps", h.Create)
	r.Get("/apps", h.List)
	r.Get("/apps/{uuid}", h.GetApp)
	r.Put("/apps/{uuid}", h.UpdateApp)
	r.Delete("/apps/{uuid}", h.DeleteApp)
}

func (h *AppsHandler) querier(tx pgx.Tx) coredb.Querier {
	if h.newQuerier != nil {
		return h.newQuerier(tx)
	}
	return coredb.New(tx)
}

// appRowResponse builds the wire shape shared by create/list/get/update:
// uuid, slug, name, created_at. Internal ids are never exposed.
func appRowResponse(appUUID uuid.UUID, slug, name string, createdAt pgtype.Timestamptz) map[string]any {
	return map[string]any{
		"uuid":       appUUID.String(),
		"slug":       slug,
		"name":       name,
		"created_at": createdAt.Time,
	}
}

type createAppRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Create handles POST /apps (admin).
func (h *AppsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiresp.WriteError(w, r, apiresp.ErrInvalidInput)
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.Name = strings.TrimSpace(req.Name)
	if req.Slug == "" {
		apiresp.WriteError(w, r, apiresp.InvalidInput(apiresp.FieldError{
			Field: "slug", Code: "apps.slug_required", Message: "slug is required",
		}))
		return
	}
	if req.Name == "" {
		apiresp.WriteError(w, r, apiresp.InvalidInput(apiresp.FieldError{
			Field: "name", Code: "apps.name_required", Message: "name is required",
		}))
		return
	}

	// Authorize: create is admin-only.
	if err := h.az.Authorize(r.Context(), "create", nil); err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	var (
		entityUUID uuid.UUID
		entityID   int64
		createdAt  pgtype.Timestamptz
		app        coredb.App
	)

	txErr := txhelper.Run(r.Context(), h.pool, func(ctx context.Context, tx pgx.Tx) error {
		q := h.querier(tx)

		// Resolve the type id for 'app' from the registry, then create the
		// entity row and the apps row keyed by that entity id, atomically.
		t, err := q.GetTypeBySlug(ctx, "app")
		if err != nil {
			return fmt.Errorf("apps.create resolve type: %w", err)
		}

		ent, err := q.CreateEntity(ctx, t.ID)
		if err != nil {
			return fmt.Errorf("apps.create entity: %w", err)
		}

		app, err = q.InsertApp(ctx, coredb.InsertAppParams{
			ID:   ent.ID,
			Slug: req.Slug,
			Name: req.Name,
		})
		if err != nil {
			return fmt.Errorf("apps.create app: %w", err)
		}

		entityUUID = ent.Uuid
		entityID = ent.ID
		createdAt = ent.CreatedAt

		after := map[string]any{
			"uuid": entityUUID.String(),
			"slug": app.Slug,
			"name": app.Name,
		}
		return h.observers.Observe(ctx, tx, "create", "app", &entityID, nil, after)
	})
	if txErr != nil {
		apiresp.WriteError(w, r, fmt.Errorf("apps.create: %w", txErr))
		return
	}

	after := map[string]any{
		"uuid": entityUUID.String(),
		"slug": app.Slug,
		"name": app.Name,
	}
	h.observers.ObserveAfterCommit(r.Context(), "create", "app", &entityID, after)

	apiresp.WriteJSON(w, http.StatusCreated, appRowResponse(entityUUID, app.Slug, app.Name, createdAt))
}

// List handles GET /apps (admin).
func (h *AppsHandler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	// Authorize: list is admin-only.
	if err := h.az.Authorize(r.Context(), "list", nil); err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	rows, err := h.q.ListApps(r.Context())
	if err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("apps.list: %w", err))
		return
	}

	resp := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, appRowResponse(row.Uuid, row.Slug, row.Name, row.CreatedAt))
	}
	apiresp.WriteJSON(w, http.StatusOK, map[string]any{"apps": resp})
}

// GetApp handles GET /apps/{uuid} (admin).
func (h *AppsHandler) GetApp(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	app, ok := h.loadAppByUUIDParam(w, r)
	if !ok {
		return
	}

	// Authorize: read — admin only for apps.
	if err := h.az.Authorize(r.Context(), "read", &app.ID); err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	apiresp.WriteJSON(w, http.StatusOK, appRowResponse(app.Uuid, app.Slug, app.Name, app.CreatedAt))
}

type updateAppRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// UpdateApp handles PUT /apps/{uuid} (admin).
func (h *AppsHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	app, ok := h.loadAppByUUIDParam(w, r)
	if !ok {
		return
	}

	// Authorize: update — admin only for apps.
	if err := h.az.Authorize(r.Context(), "update", &app.ID); err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	var req updateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiresp.WriteError(w, r, apiresp.ErrInvalidInput)
		return
	}

	newSlug := app.Slug
	if s := strings.TrimSpace(req.Slug); s != "" {
		newSlug = s
	}
	newName := app.Name
	if n := strings.TrimSpace(req.Name); n != "" {
		newName = n
	}

	before := map[string]any{
		"uuid": app.Uuid.String(),
		"slug": app.Slug,
		"name": app.Name,
	}

	updated := app
	txErr := txhelper.Run(r.Context(), h.pool, func(ctx context.Context, tx pgx.Tx) error {
		q := h.querier(tx)
		if err := q.UpdateApp(ctx, coredb.UpdateAppParams{
			ID:   app.ID,
			Slug: newSlug,
			Name: newName,
		}); err != nil {
			return fmt.Errorf("apps.update app: %w", err)
		}

		row, err := q.GetAppByUUID(ctx, app.Uuid)
		if err != nil {
			// Best-effort: fall back to the computed values (mirrors the
			// source handler's behaviour on a re-fetch failure).
			updated.Slug = newSlug
			updated.Name = newName
		} else {
			updated = row
		}

		after := map[string]any{
			"uuid": updated.Uuid.String(),
			"slug": updated.Slug,
			"name": updated.Name,
		}
		return h.observers.Observe(ctx, tx, "update", "app", &app.ID, before, after)
	})
	if txErr != nil {
		apiresp.WriteError(w, r, fmt.Errorf("apps.update: %w", txErr))
		return
	}

	after := map[string]any{
		"uuid": updated.Uuid.String(),
		"slug": updated.Slug,
		"name": updated.Name,
	}
	h.observers.ObserveAfterCommit(r.Context(), "update", "app", &app.ID, after)

	apiresp.WriteJSON(w, http.StatusOK, appRowResponse(updated.Uuid, updated.Slug, updated.Name, updated.CreatedAt))
}

// DeleteApp handles DELETE /apps/{uuid} (admin) — archives the app's
// underlying entity row (sets entities.archived_at). Apps have no
// per-table archived_at of their own; archival is the generic entity
// archive path shared by every entity subtype.
func (h *AppsHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	app, ok := h.loadAppByUUIDParam(w, r)
	if !ok {
		return
	}

	// Authorize: delete — admin only for apps.
	if err := h.az.Authorize(r.Context(), "delete", &app.ID); err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	before := map[string]any{
		"uuid": app.Uuid.String(),
		"slug": app.Slug,
		"name": app.Name,
	}

	txErr := txhelper.Run(r.Context(), h.pool, func(ctx context.Context, tx pgx.Tx) error {
		q := h.querier(tx)
		if err := q.ArchiveEntity(ctx, app.Uuid); err != nil {
			return fmt.Errorf("apps.delete: %w", err)
		}
		return h.observers.Observe(ctx, tx, "delete", "app", &app.ID, before, nil)
	})
	if txErr != nil {
		apiresp.WriteError(w, r, fmt.Errorf("apps.delete: %w", txErr))
		return
	}

	h.observers.ObserveAfterCommit(r.Context(), "delete", "app", &app.ID, nil)
	w.WriteHeader(http.StatusNoContent)
}

// loadAppByUUIDParam extracts the {uuid} chi param and loads the app via
// the entities/apps join query, returning ErrNotFound on miss (matching
// the source mod-users handler's behaviour — apps are an admin-only
// resource with no existence-masking requirement).
func (h *AppsHandler) loadAppByUUIDParam(w http.ResponseWriter, r *http.Request) (coredb.GetAppByUUIDRow, bool) {
	rawUUID := chi.URLParam(r, "uuid")
	parsed, err := uuid.Parse(rawUUID)
	if err != nil {
		apiresp.WriteError(w, r, apiresp.ErrInvalidInput)
		return coredb.GetAppByUUIDRow{}, false
	}

	app, err := h.q.GetAppByUUID(r.Context(), parsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			apiresp.WriteError(w, r, apiresp.ErrNotFound)
		} else {
			apiresp.WriteError(w, r, fmt.Errorf("apps: load by uuid: %w", err))
		}
		return coredb.GetAppByUUIDRow{}, false
	}
	return app, true
}
