package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/moduleforge/core-api/service"
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

	// The service manages its own transaction internally via txhelper.Run.
	corp, entityUUID, err := h.d.Services.Corporation.Create(r.Context(), h.d.Services.Querier(), *p, in)
	if err != nil {
		writeServiceErr(w, err)
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
