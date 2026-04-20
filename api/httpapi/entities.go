package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// getEntity handles GET /entities/{uuid} — resolves any entity UUID to its
// profile, picking the appropriate subtype automatically.
func (h *handlers) getEntity(w http.ResponseWriter, r *http.Request) {
	_, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	entityUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid uuid")
		return
	}

	profile, err := h.d.Services.Entity.ResolveProfile(r.Context(), h.d.Services.Querier(), entityUUID)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusOK, profileResponse(profile))
}

// archiveEntity handles DELETE /entities/{uuid} (admin only) — soft-deletes
// an entity by setting archived_at.
func (h *handlers) archiveEntity(w http.ResponseWriter, r *http.Request) {
	p, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	entityUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid uuid")
		return
	}

	if err := h.d.Services.Entity.Archive(r.Context(), h.d.Services.Querier(), entityUUID, *p); err != nil {
		writeServiceErr(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
