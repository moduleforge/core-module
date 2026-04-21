// Package httpapi exposes a mountable chi subrouter serving core entity routes.
// Consumers call NewRouter(deps) and mount the returned router under their
// preferred path prefix (typically /v1).
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moduleforge/core-api/service"
	coredb "github.com/moduleforge/core-model/db"
)

// jsonOK encodes body as JSON and writes status to w.
func jsonOK(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// jsonErr writes a JSON error envelope to w.
func jsonErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": msg,
	})
}

// writeServiceErr maps service sentinel errors to HTTP status codes and
// writes a JSON error response.
func writeServiceErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		jsonErr(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, service.ErrForbidden):
		jsonErr(w, http.StatusForbidden, "forbidden", "access denied")
	case errors.Is(err, service.ErrInvalidInput):
		jsonErr(w, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		jsonErr(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
	}
}

// profileResponse converts a service.Profile into a JSON-serialisable map.
// Used by entity-CRUD handlers that return the resolved Profile after a
// mutation or lookup. The "kind" JSON key carries the fundamental_type_slug.
// Tax-id fields are never included; use profileResponseFor when the caller
// principal is available.
func profileResponse(p service.Profile) map[string]any {
	return profileResponseFor(service.Principal{}, p)
}

// profileResponseFor is like profileResponse but gates tax_id / tax_id_type
// inclusion on the caller's identity:
//
//	include IFF principal.IsAdmin OR principal.EntityID == profile.Entity.ID
//
// When the gate fails the keys are omitted entirely (no nulls emitted).
func profileResponseFor(principal service.Principal, p service.Profile) map[string]any {
	resp := map[string]any{
		"kind": p.Kind,
	}

	if p.Entity.Uuid != (coredb.GetEntityByUUIDRow{}).Uuid {
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

	// Gate: include tax_id / tax_id_type only for admins or the subject.
	if p.TaxID != "" && (principal.IsAdmin || principal.EntityID == p.Entity.ID) {
		resp["tax_id"] = p.TaxID
		resp["tax_id_type"] = p.TaxIDType
	}

	return resp
}
