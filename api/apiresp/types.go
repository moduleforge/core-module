package apiresp

// FieldError is a single field-level or sub-error detail attached to an
// ErrorBody. Field binds it to a rendered input by name; Code is a
// machine-readable code namespaced by the owning module or resource (e.g.
// "users.email_taken"); Message is a human-readable summary safe to
// display to an end user.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorBody is the nested "error" object carried by every non-2xx
// response. Details is omitted — never null, never an empty array — when
// there is no per-field detail; see the design doc's field-semantics
// table.
type ErrorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// Envelope is the top-level error response body: a single "error" member
// wrapping ErrorBody. WriteError is the only place that constructs and
// writes an Envelope.
type Envelope struct {
	Error ErrorBody `json:"error"`
}
