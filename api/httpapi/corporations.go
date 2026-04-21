package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// createCorporationRequest is the body for POST /entities/corporations.
type createCorporationRequest struct {
	LegalName    string `json:"legal_name"`
	Jurisdiction string `json:"jurisdiction"`
	EIN          string `json:"ein,omitempty"` // optional plaintext; "" means not recorded
}

// createCorporation handles POST /entities/corporations (admin only).
func (h *handlers) createCorporation(w http.ResponseWriter, r *http.Request) {
	p, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if !p.IsAdmin {
		jsonErr(w, http.StatusForbidden, "forbidden", "admin required")
		return
	}

	var req createCorporationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	in := service.CreateCorporationInput{
		LegalName:    req.LegalName,
		Jurisdiction: req.Jurisdiction,
		EIN:          req.EIN,
	}

	tx, err := h.d.txBeginner().Begin(r.Context())
	if err != nil {
		h.d.Logger.ErrorContext(r.Context(), "createCorporation: begin tx", "error", err)
		jsonErr(w, http.StatusInternalServerError, "internal_error", "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	txQ := coredb.New(tx)
	corp, entityUUID, err := h.d.Services.Corporation.Create(r.Context(), txQ, *p, in)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		h.d.Logger.ErrorContext(r.Context(), "createCorporation: commit", "error", err)
		jsonErr(w, http.StatusInternalServerError, "internal_error", "failed to commit transaction")
		return
	}

	resp := map[string]any{
		"uuid":         entityUUID.String(),
		"kind":         "corporation",
		"legal_name":   corp.LegalName,
		"jurisdiction": corp.Jurisdiction.String,
	}
	// Admin callers always see the tax_id they just wrote, if one was supplied.
	if req.EIN != "" {
		resp["tax_id"] = req.EIN
		resp["tax_id_type"] = "EIN"
	}
	jsonOK(w, http.StatusCreated, resp)
}

// getCorporation handles GET /entities/corporations/{uuid}.
func (h *handlers) getCorporation(w http.ResponseWriter, r *http.Request) {
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

	profile, err := h.d.Services.Corporation.GetByEntityUUID(r.Context(), h.d.Services.Querier(), entityUUID)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusOK, profileResponseFor(*p, profile))
}

// updateCorporationRequest is the body for PUT /entities/corporations/{uuid}.
// EIN uses three-state semantics: nil (field absent) = unchanged, pointer-to-""
// = clear, non-empty pointer = set.
type updateCorporationRequest struct {
	LegalName    *string `json:"legal_name"`
	Jurisdiction *string `json:"jurisdiction"`
	EIN          *string `json:"ein"` // nil = unchanged, "" = clear, else set
}

// updateCorporation handles PUT /entities/corporations/{uuid} (admin only).
func (h *handlers) updateCorporation(w http.ResponseWriter, r *http.Request) {
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

	var req updateCorporationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	in := service.UpdateCorporationInput{
		LegalName:    req.LegalName,
		Jurisdiction: req.Jurisdiction,
		EIN:          req.EIN,
	}

	if err := h.d.Services.Corporation.UpdateByEntityUUID(
		r.Context(),
		h.d.Services.Querier(),
		entityUUID,
		in,
		*p,
	); err != nil {
		writeServiceErr(w, err)
		return
	}

	profile, err := h.d.Services.Corporation.GetByEntityUUID(r.Context(), h.d.Services.Querier(), entityUUID)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusOK, profileResponseFor(*p, profile))
}
