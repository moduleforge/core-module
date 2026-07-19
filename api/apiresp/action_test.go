package apiresp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moduleforge/core-api/apiresp"
)

// actionEnvelope mirrors apiresp.ActionEnvelope for decoding responses in
// tests, using map[string]any for the action object so we can distinguish
// an absent "data" key from a present-but-empty/null one.
type actionEnvelope struct {
	Action map[string]any `json:"action"`
}

// TestWriteActionRequired_RegisteredFlows verifies each of the three
// registered mod-users action codes writes the documented status verbatim
// (in particular that 503 is emitted, not remapped), the action envelope
// shape, Content-Type, and that no top-level "error" member is present.
func TestWriteActionRequired_RegisteredFlows(t *testing.T) {
	cases := []struct {
		name       string
		action     apiresp.ActionCode
		message    string
		path       string
		wantStatus int
	}{
		{
			name:       "email_unverified",
			action:     apiresp.ActionCode{Code: "users.email_unverified", Status: http.StatusForbidden},
			message:    "Verify your email address before continuing.",
			path:       "/verify-email",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "step_up_required",
			action:     apiresp.ActionCode{Code: "users.step_up_required", Status: http.StatusConflict},
			message:    "Confirm your identity to remove this login method.",
			path:       "/step-up?return=%2Fself%2Fidentities",
			wantStatus: http.StatusConflict,
		},
		{
			name:       "oidc_not_confirmed",
			action:     apiresp.ActionCode{Code: "users.oidc_not_confirmed", Status: http.StatusServiceUnavailable},
			message:    "Single sign-on is not finished configuring.",
			path:       "/setup/oidc",
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/whatever", nil)

			apiresp.WriteActionRequired(rec, req, tc.action, tc.message, tc.path, nil)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d", tc.wantStatus, rec.Code)
			}
			if rec.Code != tc.action.Status {
				t.Fatalf("status: want ActionCode.Status %d exactly, got %d", tc.action.Status, rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type: want application/json, got %q", ct)
			}

			var env actionEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
			}
			if env.Action == nil {
				t.Fatal("expected top-level \"action\" object, got none")
			}
			if code, _ := env.Action["code"].(string); code != tc.action.Code {
				t.Fatalf("action.code: want %q, got %q", tc.action.Code, code)
			}
			if msg, _ := env.Action["message"].(string); msg != tc.message {
				t.Fatalf("action.message: want %q, got %q", tc.message, msg)
			}
			if path, _ := env.Action["path"].(string); path != tc.path {
				t.Fatalf("action.path: want %q, got %q", tc.path, path)
			}

			var raw map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatalf("decode raw body: %v", err)
			}
			if _, present := raw["error"]; present {
				t.Fatalf("response unexpectedly carries top-level \"error\" member: %s", rec.Body.String())
			}
		})
	}
}

// TestWriteActionRequired_Data verifies action.data is present and
// correctly nested when supplied, and that the "data" key is absent from
// the raw JSON body — not null, not {} — when data is nil.
func TestWriteActionRequired_Data(t *testing.T) {
	action := apiresp.ActionCode{Code: "users.oidc_not_confirmed", Status: http.StatusServiceUnavailable}

	t.Run("with data", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/self", nil)

		data := map[string]any{"state": "awaiting_oidc_config"}
		apiresp.WriteActionRequired(rec, req, action, "Single sign-on is not finished configuring.", "/setup/oidc", data)

		var env actionEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
		}
		got, ok := env.Action["data"].(map[string]any)
		if !ok {
			t.Fatalf("action.data: expected object, got %T (%v)", env.Action["data"], env.Action["data"])
		}
		if got["state"] != "awaiting_oidc_config" {
			t.Fatalf("action.data.state: want %q, got %v", "awaiting_oidc_config", got["state"])
		}
	})

	t.Run("without data", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/self", nil)

		apiresp.WriteActionRequired(rec, req, action, "Single sign-on is not finished configuring.", "/setup/oidc", nil)

		var env actionEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
		}
		if _, present := env.Action["data"]; present {
			t.Fatalf("action.data: expected absent, got present: %v", env.Action["data"])
		}

		// Belt-and-suspenders: confirm the raw body text has no "data" key
		// at all, ruling out both a null and an empty-object encoding.
		if strings.Contains(rec.Body.String(), "data") {
			t.Fatalf("response body unexpectedly contains \"data\": %s", rec.Body.String())
		}
	})

	t.Run("with typed-nil pointer data", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/self", nil)

		// A typed-nil (non-nil interface holding a nil *struct{}) must be
		// treated the same as an untyped nil: omitted from the body, not
		// serialized as "data":null. This exercises the classic Go
		// nil-interface gotcha where data != nil evaluates true even though
		// the underlying pointer is nil.
		var p *struct{ X int }
		apiresp.WriteActionRequired(rec, req, action, "Single sign-on is not finished configuring.", "/setup/oidc", p)

		var env actionEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
		}
		if _, present := env.Action["data"]; present {
			t.Fatalf("action.data: expected absent for typed-nil pointer, got present: %v", env.Action["data"])
		}

		if strings.Contains(rec.Body.String(), "data") {
			t.Fatalf("response body unexpectedly contains \"data\" for typed-nil pointer: %s", rec.Body.String())
		}
	})
}

// TestWriteActionRequired_PathGuard verifies the action.path structural
// guard: WriteActionRequired panics for a path carrying a URL scheme, a
// "//"-prefixed (protocol-relative) authority, a backslash-based host
// confusion (browsers treat "\" as interchangeable with "/" when entering
// the authority-slashes state), or multi-slash authority collapsing (2+
// leading "/" or "\" characters, not just a literal "//" prefix) — and
// does not panic for ordinary application-relative paths, including one
// carrying a query string.
func TestWriteActionRequired_PathGuard(t *testing.T) {
	action := apiresp.ActionCode{Code: "users.email_unverified", Status: http.StatusForbidden}

	cases := []struct {
		name      string
		path      string
		wantPanic bool
	}{
		{"absolute http URL", "http://evil.example/x", true},
		{"protocol-relative authority", "//evil.example/x", true},
		{"javascript scheme", "javascript:alert(1)", true},
		{"empty path", "", true},
		{"backslash host confusion (leading slash-backslash)", "/\\evil.example/x", true},
		{"backslash host confusion (double backslash)", "\\\\evil.example/x", true},
		{"backslash host confusion (backslash-slash)", "\\/evil.example/x", true},
		{"triple-slash authority collapsing", "///evil.example", true},
		{"quadruple-slash authority collapsing", "////evil.example", true},
		{"ordinary relative path", "/verify-email", false},
		{"relative path with query", "/step-up?return=%2Fself%2Fidentities", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/whatever", nil)

			gotPanic := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						gotPanic = true
					}
				}()
				apiresp.WriteActionRequired(rec, req, action, "message", tc.path, nil)
			}()

			if gotPanic != tc.wantPanic {
				t.Fatalf("panic: want %v, got %v (path=%q)", tc.wantPanic, gotPanic, tc.path)
			}
		})
	}
}
