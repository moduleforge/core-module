package apiresp

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

// ActionBody is the nested "action" object carried by an action-required
// response. Code, Message, and Path are always present. Data is omitted —
// never null, never an empty object — when the action carries no auxiliary
// state; see the design doc's field-semantics table for the action object.
type ActionBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path"`
	Data    any    `json:"data,omitempty"`
}

// ActionEnvelope is the top-level action-required response body: a single
// "action" member wrapping ActionBody. It is the flow-control parallel to
// Envelope{Error: ErrorBody} — mutually exclusive with it, never both on
// the same response. WriteActionRequired is the only place that constructs
// and writes an ActionEnvelope.
type ActionEnvelope struct {
	Action ActionBody `json:"action"`
}

// ActionCode is a registered, module-namespaced action code (e.g.
// "users.email_unverified") together with the single HTTP status bound to
// it (403, 409, or 503). Pairing the code with its bound status makes
// unregistered codes and off-status writes unrepresentable at the call
// site: a caller supplies a registered ActionCode value rather than a bare
// string plus a separately-chosen status.
//
// Ownership boundary: this package does not maintain a registry of valid
// codes and does not validate Status against the {403, 409, 503}
// action-required status set. The closed set of action codes and their
// bound statuses is owned per-module, in each module's own API reference
// (see the design doc's "Action-code vocabulary"). This package's job is
// only to carry and emit the code and status the caller supplies.
type ActionCode struct {
	Code   string // namespaced, e.g. "users.email_unverified"
	Status int    // the code's bound status: 403, 409, or 503
}

// WriteActionRequired writes the action-required envelope for a
// flow-control signal that is neither success nor error — a peer of
// WriteJSON and WriteError, not a variant of WriteError. Its normal path
// never calls WriteError and never emits the error envelope; it always
// emits the action envelope via WriteJSON.
//
// It writes action.Status verbatim — including 503 for
// "users.oidc_not_confirmed" — and MUST NOT remap it onto an error status
// (403/409/500) under any circumstance; 503 is a first-class, reserved
// action-required status per the design doc's "Action-required status
// set".
//
// Action.Data is set only when data is non-nil, so a nil data omits the
// "data" member from the response body via omitempty rather than emitting
// null. This also covers the typed-nil case (e.g. a nil *T or nil map
// passed through the any parameter): such a value is non-nil as an
// interface but carries no useful state, so it is detected via reflection
// and omitted the same as an untyped nil, rather than serializing as
// "data":null.
//
// Caller obligations enforced/documented at this boundary:
//   - path MUST be application-relative (no URL scheme, no "//"-prefixed
//     authority) — WriteActionRequired panics if it is not, treating a
//     violation as a programmer error / open-redirect guard rather than
//     writing an unsafe navigation target.
//   - data MUST contain only minimal, non-sensitive state the destination
//     needs to render the action; it MUST NOT carry internal identifiers,
//     tokens, or other server-internal state. This constraint is not
//     mechanically checked — it is a caller-discipline obligation.
//
// Panic-recovery precondition: callers' HTTP server MUST install
// panic-recovery middleware around any handler that can reach this
// function. This package does not recover the guard-violation panic
// itself, and an unrecovered panic crashes the process instead of failing
// one request. Additionally, a guard-violation panic is not routed through
// this package's structured logServerError path (used by WriteError's 5xx
// path); it instead surfaces as a bare, unstructured panic/stack-trace with
// no request-ID correlation. Callers relying on request-ID-correlated 5xx
// logs for this failure mode should recover and log it themselves.
func WriteActionRequired(w http.ResponseWriter, r *http.Request, action ActionCode, message, path string, data any) {
	if !isAppRelativePath(path) {
		panic("apiresp: WriteActionRequired: action.path is not application-relative (open-redirect guard): " + path)
	}

	env := ActionEnvelope{Action: ActionBody{Code: action.Code, Message: message, Path: path}}
	if data != nil && !isNilValue(data) {
		env.Action.Data = data
	}
	WriteJSON(w, action.Status, env)
}

// isNilValue reports whether data holds a typed-nil value (e.g. a nil *T,
// nil map, nil slice, nil chan, nil func, or nil interface boxed inside the
// any parameter). Such a value is non-nil when compared as an interface
// (data != nil is true) because the interface still carries a concrete
// type descriptor, but it carries no useful state and should be treated
// like an untyped nil for the purposes of omitting Action.Data. Kinds that
// are not nilable are reported as not-nil without calling IsNil, which
// would otherwise panic.
func isNilValue(data any) bool {
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

// isAppRelativePath reports whether p is a valid application-relative
// navigation target for action.path: no URL scheme (e.g. "http://…",
// "javascript:…") and no "//"-prefixed authority (protocol-relative, e.g.
// "//evil.example/x"). An empty path is also rejected — it is not a valid
// navigation target. Ordinary relative paths, including those carrying a
// query string, are accepted.
//
// Beyond what net/url.Parse alone reports, two additional bypass classes
// are rejected because browsers (WHATWG URL parsing, used for
// window.location, <a href>, and most JS routers) resolve them off-origin
// even though Go's RFC-3986-based net/url.Parse does not flag them as
// scheme- or authority-bearing:
//
//   - Backslash-based host confusion: browsers treat "\" as interchangeable
//     with "/" when entering the authority-slashes state for special
//     schemes. "/\evil.example/x" and "\\evil.example/x" both parse via
//     net/url.Parse with an empty Scheme and Host, but a browser navigates
//     them off-origin. Any literal backslash in p is rejected outright.
//   - Multi-slash authority collapsing: WHATWG's state machine treats a run
//     of 2+ leading "/" or "\" characters (in any combination) as
//     introducing an authority, not just a literal "//" prefix.
//     "///evil.example" and "////evil.example" also parse via
//     net/url.Parse with an empty Host, but browsers resolve them
//     off-origin. p is checked directly for 2+ leading slashes rather than
//     relying on net/url.Parse's Host field, which does not flag this case.
func isAppRelativePath(p string) bool {
	if p == "" {
		return false
	}
	if strings.ContainsRune(p, '\\') {
		return false
	}
	if strings.HasPrefix(p, "//") {
		return false
	}
	u, err := url.Parse(p)
	if err != nil {
		return false
	}
	return u.Scheme == "" && u.Host == ""
}
