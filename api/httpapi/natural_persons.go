package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// createNaturalPersonRequest is the body for POST /entities/natural-persons.
type createNaturalPersonRequest struct {
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	SSN        string `json:"ssn,omitempty"` // optional plaintext; "" means not recorded
}

// createNaturalPerson handles POST /entities/natural-persons (admin only).
func (h *handlers) createNaturalPerson(w http.ResponseWriter, r *http.Request) {
	p, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if !p.IsAdmin {
		jsonErr(w, http.StatusForbidden, "forbidden", "admin required")
		return
	}

	var req createNaturalPersonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	in := service.CreateNaturalPersonInput{
		GivenName:  req.GivenName,
		FamilyName: req.FamilyName,
		SSN:        req.SSN,
	}

	// Multi-table create: open a tx, pass tx-scoped querier to the service.
	tx, err := h.d.txBeginner().Begin(r.Context())
	if err != nil {
		h.d.Logger.ErrorContext(r.Context(), "createNaturalPerson: begin tx", "error", err)
		jsonErr(w, http.StatusInternalServerError, "internal_error", "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	txQ := coredb.New(tx)
	np, entityUUID, err := h.d.Services.NaturalPerson.Create(r.Context(), txQ, *p, in)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		h.d.Logger.ErrorContext(r.Context(), "createNaturalPerson: commit", "error", err)
		jsonErr(w, http.StatusInternalServerError, "internal_error", "failed to commit transaction")
		return
	}

	resp := map[string]any{
		"uuid":        entityUUID.String(),
		"kind":        "natural_person",
		"given_name":  np.GivenName.String,
		"family_name": np.FamilyName.String,
	}
	// Admin callers always see the tax_id they just wrote, if one was supplied.
	if req.SSN != "" {
		resp["tax_id"] = req.SSN
		resp["tax_id_type"] = "SSN"
	}
	jsonOK(w, http.StatusCreated, resp)
}

// getNaturalPerson handles GET /entities/natural-persons/{uuid}.
func (h *handlers) getNaturalPerson(w http.ResponseWriter, r *http.Request) {
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

	profile, err := h.d.Services.NaturalPerson.GetByEntityUUID(r.Context(), h.d.Services.Querier(), entityUUID)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	// Non-admin callers may only retrieve their own profile.
	if !p.IsAdmin && p.EntityID != profile.Entity.ID {
		jsonErr(w, http.StatusForbidden, "forbidden", "access denied")
		return
	}

	jsonOK(w, http.StatusOK, profileResponseFor(*p, profile))
}

// updateNaturalPersonRequest is the body for PUT /entities/natural-persons/{uuid}.
// SSN uses three-state semantics: nil (field absent) = unchanged, pointer-to-""
// = clear, non-empty pointer = set.
type updateNaturalPersonRequest struct {
	GivenName  *string `json:"given_name"`
	FamilyName *string `json:"family_name"`
	SSN        *string `json:"ssn"` // nil = unchanged, "" = clear, else set
}

// updateNaturalPerson handles PUT /entities/natural-persons/{uuid}.
func (h *handlers) updateNaturalPerson(w http.ResponseWriter, r *http.Request) {
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

	var req updateNaturalPersonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	in := service.UpdateNaturalPersonInput{
		GivenName:  req.GivenName,
		FamilyName: req.FamilyName,
		SSN:        req.SSN,
	}

	if err := h.d.Services.NaturalPerson.UpdateByEntityUUID(
		r.Context(),
		h.d.Services.Querier(),
		entityUUID,
		in,
		*p,
	); err != nil {
		writeServiceErr(w, err)
		return
	}

	profile, err := h.d.Services.NaturalPerson.GetByEntityUUID(r.Context(), h.d.Services.Querier(), entityUUID)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusOK, profileResponseFor(*p, profile))
}
