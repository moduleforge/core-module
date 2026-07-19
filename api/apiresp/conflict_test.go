package apiresp_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moduleforge/core-api/apiresp"
)

// TestConflict_WithDetails verifies Conflict(fe...) maps to 409 conflict
// with the supplied details populated in the envelope.
func TestConflict_WithDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/whatever", nil)

	details := []apiresp.FieldError{
		{Field: "identities", Code: "users.last_identity", Message: "Cannot remove the last identity."},
		{Field: "email", Code: "users.email_taken", Message: "That email is already registered."},
	}
	err := apiresp.Conflict(details...)

	if !errors.Is(err, apiresp.ErrConflict) {
		t.Fatal("expected errors.Is(err, ErrConflict) to be true")
	}

	apiresp.WriteError(rec, req, err)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status: want %d, got %d", http.StatusConflict, rec.Code)
	}

	var env struct {
		Error struct {
			Code    string               `json:"code"`
			Message string               `json:"message"`
			Details []apiresp.FieldError `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if env.Error.Code != "conflict" {
		t.Fatalf("error.code: want conflict, got %q", env.Error.Code)
	}
	if env.Error.Message == "" {
		t.Fatal("error.message: expected non-empty message")
	}
	if len(env.Error.Details) != len(details) {
		t.Fatalf("error.details: want %d entries, got %d (%+v)", len(details), len(env.Error.Details), env.Error.Details)
	}
	for i, fe := range details {
		if env.Error.Details[i] != fe {
			t.Errorf("error.details[%d]: want %+v, got %+v", i, fe, env.Error.Details[i])
		}
	}
}

// TestConflict_NoDetails verifies Conflict() with no arguments still maps
// to 409 conflict, and that error.details is absent from the JSON body —
// not null, not [] — per the omitempty contract.
func TestConflict_NoDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/whatever", nil)

	err := apiresp.Conflict()

	if !errors.Is(err, apiresp.ErrConflict) {
		t.Fatal("expected errors.Is(err, ErrConflict) to be true")
	}

	apiresp.WriteError(rec, req, err)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status: want %d, got %d", http.StatusConflict, rec.Code)
	}

	var env struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if code, _ := env.Error["code"].(string); code != "conflict" {
		t.Fatalf("error.code: want conflict, got %q", code)
	}
	if _, present := env.Error["details"]; present {
		t.Fatalf("error.details: expected absent (omitempty), got present: %v", env.Error["details"])
	}

	// Belt-and-suspenders: confirm the raw body text has no "details" key
	// at all, ruling out both a null and an empty-array encoding.
	if strings.Contains(rec.Body.String(), "details") {
		t.Fatalf("response body unexpectedly contains \"details\": %s", rec.Body.String())
	}
}
