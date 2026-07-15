package apiresp_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moduleforge/core-api/apiresp"
)

// envelope mirrors apiresp.Envelope for decoding responses in tests,
// using map[string]any for the error object so we can distinguish an
// absent "details" key from a present-but-empty/null one.
type envelope struct {
	Error map[string]any `json:"error"`
}

// TestWriteError_SentinelMapping verifies each sentinel maps to its
// documented (status, code) pair via WriteError, asserting the nested
// envelope shape and Content-Type.
func TestWriteError_SentinelMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"unauthenticated", apiresp.ErrUnauthenticated, http.StatusUnauthorized, "unauthenticated"},
		{"forbidden", apiresp.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"not_found", apiresp.ErrNotFound, http.StatusNotFound, "not_found"},
		{"invalid_input", apiresp.ErrInvalidInput, http.StatusBadRequest, "invalid_input"},
		{"conflict", apiresp.ErrConflict, http.StatusConflict, "conflict"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/whatever", nil)

			apiresp.WriteError(rec, req, tc.err)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d", tc.wantStatus, rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type: want application/json, got %q", ct)
			}

			var env envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
			}
			if env.Error == nil {
				t.Fatal("expected top-level \"error\" object, got none")
			}
			if code, _ := env.Error["code"].(string); code != tc.wantCode {
				t.Fatalf("error.code: want %q, got %q", tc.wantCode, code)
			}
			if msg, _ := env.Error["message"].(string); msg == "" {
				t.Fatal("error.message: expected non-empty message")
			}
			if _, present := env.Error["details"]; present {
				t.Fatalf("error.details: expected absent, got present: %v", env.Error["details"])
			}
		})
	}
}

// TestWriteError_UnmappedError verifies an error not matching any sentinel
// maps to 500 internal_error with the fixed generic message, and that the
// raw error text never appears in the response body.
func TestWriteError_UnmappedError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)

	rawText := "super secret internal detail: connection refused to db-primary-7"
	err := errors.New(rawText)

	apiresp.WriteError(rec, req, err)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, rawText) {
		t.Fatalf("response body leaked raw error text: %s", body)
	}

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, body)
	}
	if code, _ := env.Error["code"].(string); code != "internal_error" {
		t.Fatalf("error.code: want internal_error, got %q", code)
	}
	msg, _ := env.Error["message"].(string)
	if msg == "" {
		t.Fatal("error.message: expected non-empty generic message")
	}
	if strings.Contains(msg, rawText) {
		t.Fatalf("error.message leaked raw error text: %q", msg)
	}
	if _, present := env.Error["details"]; present {
		t.Fatalf("error.details: expected absent, got present: %v", env.Error["details"])
	}
}

// TestWriteError_WrappedSentinel verifies a sentinel wrapped via
// fmt.Errorf("...: %w", ...) still classifies correctly through errors.Is.
func TestWriteError_WrappedSentinel(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)

	wrapped := fmt.Errorf("looking up widget 42: %w", apiresp.ErrNotFound)

	apiresp.WriteError(rec, req, wrapped)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want %d, got %d", http.StatusNotFound, rec.Code)
	}

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if code, _ := env.Error["code"].(string); code != "not_found" {
		t.Fatalf("error.code: want not_found, got %q", code)
	}
}

// TestInvalidInput_WithDetails verifies InvalidInput(fe...) maps to 400
// invalid_input with the supplied details populated in the envelope.
func TestInvalidInput_WithDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/whatever", nil)

	details := []apiresp.FieldError{
		{Field: "email", Code: "users.email_taken", Message: "That email is already registered."},
		{Field: "password", Code: "users.too_short", Message: "Password must be at least 12 characters."},
	}
	err := apiresp.InvalidInput(details...)

	if !errors.Is(err, apiresp.ErrInvalidInput) {
		t.Fatal("expected errors.Is(err, ErrInvalidInput) to be true")
	}

	apiresp.WriteError(rec, req, err)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want %d, got %d", http.StatusBadRequest, rec.Code)
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
	if env.Error.Code != "invalid_input" {
		t.Fatalf("error.code: want invalid_input, got %q", env.Error.Code)
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

// TestInvalidInput_NoDetails verifies InvalidInput() with no arguments
// still maps to 400 invalid_input, and that error.details is absent from
// the JSON body — not null, not [] — per the omitempty contract.
func TestInvalidInput_NoDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/whatever", nil)

	err := apiresp.InvalidInput()

	if !errors.Is(err, apiresp.ErrInvalidInput) {
		t.Fatal("expected errors.Is(err, ErrInvalidInput) to be true")
	}

	apiresp.WriteError(rec, req, err)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if code, _ := env.Error["code"].(string); code != "invalid_input" {
		t.Fatalf("error.code: want invalid_input, got %q", code)
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

// TestWriteJSON verifies WriteJSON writes a bare body — no "error"
// wrapper — with the given status and Content-Type.
func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	body := map[string]string{"uuid": "6f1c-example", "name": "urgent"}
	apiresp.WriteJSON(rec, http.StatusCreated, body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: want %d, got %d", http.StatusCreated, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: want application/json, got %q", ct)
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
	}
	if got["uuid"] != "6f1c-example" || got["name"] != "urgent" {
		t.Fatalf("body: want %+v, got %+v", body, got)
	}

	// No "error" wrapper: the top-level value is the resource itself.
	if _, present := got["error"]; present {
		t.Fatal("WriteJSON body unexpectedly wrapped in \"error\"")
	}
}

// TestWriteError_5xxWithRealRequestContext exercises WriteError with a
// non-nil *http.Request carrying a real context, verifying the slog call
// in the 5xx path does not panic and still produces the expected response.
func TestWriteError_5xxWithRealRequestContext(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/widgets/42", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("WriteError panicked on 5xx path: %v", r)
		}
	}()

	apiresp.WriteError(rec, req, errors.New("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
