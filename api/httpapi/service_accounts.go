package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/core-api/service"
)

// createServiceAccountRequest is the body for POST /entities/service-accounts.
type createServiceAccountRequest struct {
	Label string `json:"label"`
}

// createServiceAccount handles POST /entities/service-accounts (admin only).
func (h *handlers) createServiceAccount(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req createServiceAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	in := service.CreateServiceAccountInput{Label: req.Label}

	// The service manages its own transaction internally via txhelper.Run.
	sa, entityUUID, err := h.d.Services.ServiceAccount.Create(r.Context(), h.d.Services.Querier(), in)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusCreated, map[string]any{
		"uuid":  entityUUID.String(),
		"label": sa.Label,
	})
}

// getServiceAccount handles GET /entities/service-accounts/{uuid} (admin only).
func (h *handlers) getServiceAccount(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
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
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
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
