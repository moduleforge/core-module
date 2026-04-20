// Package httpapi exposes a mountable chi subrouter serving core entity routes.
// Consumers call NewRouter(deps) and mount the returned router under their
// preferred path prefix (typically /v1).
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moduleforge/core-api/service"
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
