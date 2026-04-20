package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// createServiceAccountRequest is the body for POST /entities/service-accounts.
type createServiceAccountRequest struct {
	Label string `json:"label"`
}

// createServiceAccount handles POST /entities/service-accounts (admin only).
func (h *handlers) createServiceAccount(w http.ResponseWriter, r *http.Request) {
	p, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if !p.IsAdmin {
		jsonErr(w, http.StatusForbidden, "forbidden", "admin required")
		return
	}

	var req createServiceAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	in := service.CreateServiceAccountInput{Label: req.Label}

	tx, err := h.d.txBeginner().Begin(r.Context())
	if err != nil {
		h.d.Logger.ErrorContext(r.Context(), "createServiceAccount: begin tx", "error", err)
		jsonErr(w, http.StatusInternalServerError, "internal_error", "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	txQ := coredb.New(tx)
	sa, entityUUID, err := h.d.Services.ServiceAccount.Create(r.Context(), txQ, *p, in)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		h.d.Logger.ErrorContext(r.Context(), "createServiceAccount: commit", "error", err)
		jsonErr(w, http.StatusInternalServerError, "internal_error", "failed to commit transaction")
		return
	}

	jsonOK(w, http.StatusCreated, map[string]any{
		"uuid":  entityUUID.String(),
		"label": sa.Label,
	})
}

// getServiceAccount handles GET /entities/service-accounts/{uuid} (admin only).
func (h *handlers) getServiceAccount(w http.ResponseWriter, r *http.Request) {
	p, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if !p.IsAdmin {
		jsonErr(w, http.StatusForbidden, "forbidden", "admin required")
		return
	}

	entityUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid uuid")
		return
	}

	profile, err := h.d.Services.ServiceAccount.GetByEntityUUID(r.Context(), h.d.Services.Querier(), entityUUID)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusOK, profileResponse(profile))
}

// updateServiceAccountRequest is the body for PUT /entities/service-accounts/{uuid}.
type updateServiceAccountRequest struct {
	Label *string `json:"label"`
}

// updateServiceAccount handles PUT /entities/service-accounts/{uuid} (admin only).
func (h *handlers) updateServiceAccount(w http.ResponseWriter, r *http.Request) {
	p, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if !p.IsAdmin {
		jsonErr(w, http.StatusForbidden, "forbidden", "admin required")
		return
	}

	entityUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid uuid")
		return
	}

	var req updateServiceAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	in := service.UpdateServiceAccountInput{Label: req.Label}

	if err := h.d.Services.ServiceAccount.UpdateByEntityUUID(
		r.Context(),
		h.d.Services.Querier(),
		entityUUID,
		in,
		*p,
	); err != nil {
		writeServiceErr(w, err)
		return
	}

	profile, err := h.d.Services.ServiceAccount.GetByEntityUUID(r.Context(), h.d.Services.Querier(), entityUUID)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusOK, profileResponse(profile))
}
