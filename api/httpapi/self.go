package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// getSelf handles GET /entities/self — returns the caller's entity profile.
func (h *handlers) getSelf(w http.ResponseWriter, r *http.Request) {
	p, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	profile, err := h.d.Services.Entity.GetSelf(r.Context(), h.d.Services.Querier(), *p)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusOK, profileResponse(profile))
}

// putSelfRequest is the request body for PUT /entities/self.
type putSelfRequest struct {
	GivenName  *string `json:"given_name"`
	FamilyName *string `json:"family_name"`
}

// putSelf handles PUT /entities/self — updates the caller's natural_person fields.
func (h *handlers) putSelf(w http.ResponseWriter, r *http.Request) {
	p, ok := h.d.Principal.FromContext(r.Context())
	if !ok {
		jsonErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req putSelfRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	// Resolve entity UUID from the caller's entity ID. We need it to call
	// UpdateByEntityUUID. Resolve the profile first to get the entity row.
	profile, err := h.d.Services.Entity.GetSelf(r.Context(), h.d.Services.Querier(), *p)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	if profile.Kind != "natural_person" {
		jsonErr(w, http.StatusBadRequest, "bad_request", "self update only supported for natural_person entities")
		return
	}

	in := service.UpdateNaturalPersonInput{
		GivenName:  req.GivenName,
		FamilyName: req.FamilyName,
	}

	if err := h.d.Services.NaturalPerson.UpdateByEntityUUID(
		r.Context(),
		h.d.Services.Querier(),
		profile.Entity.Uuid,
		in,
		*p,
	); err != nil {
		writeServiceErr(w, err)
		return
	}

	// Return the updated profile.
	updated, err := h.d.Services.Entity.GetSelf(r.Context(), h.d.Services.Querier(), *p)
	if err != nil {
		writeServiceErr(w, err)
		return
	}

	jsonOK(w, http.StatusOK, profileResponse(updated))
}

// profileResponse converts a service.Profile into a JSON-serialisable map.
func profileResponse(p service.Profile) map[string]any {
	resp := map[string]any{
		"kind": p.Kind,
	}

	if p.Entity.Uuid != (coredb.Entity{}).Uuid {
		resp["uuid"] = p.Entity.Uuid.String()
	}

	switch p.Kind {
	case "natural_person":
		if np := p.NaturalPerson; np != nil {
			resp["given_name"] = np.GivenName.String
			resp["family_name"] = np.FamilyName.String
		}
	case "corporation":
		if corp := p.Corporation; corp != nil {
			resp["legal_name"] = corp.LegalName
			resp["jurisdiction"] = corp.Jurisdiction.String
		}
	case "service_account":
		if sa := p.ServiceAccount; sa != nil {
			resp["label"] = sa.Label
		}
	}

	return resp
}
