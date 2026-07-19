package apiresp

// conflictError wraps ErrConflict and carries the field-level details
// supplied to Conflict. Its Unwrap method makes errors.Is(err,
// ErrConflict) succeed for values built by Conflict; WriteError's
// fieldErrors accessor uses errors.As to recover the concrete type and
// surface Details in the response envelope.
type conflictError struct {
	details []FieldError
}

// Error implements the error interface. It returns the underlying
// sentinel's text; the safe, client-facing message is decided separately
// by publicMessage in WriteError, not by this method.
func (e *conflictError) Error() string {
	return ErrConflict.Error()
}

// Unwrap makes errors.Is(err, ErrConflict) succeed for values built by
// Conflict.
func (e *conflictError) Unwrap() error {
	return ErrConflict
}

// fieldDetails implements fieldDetailCarrier, letting WriteError's
// fieldErrors accessor recover the details carried by a conflictError.
func (e *conflictError) fieldDetails() []FieldError {
	return e.details
}

// Conflict builds an error that satisfies errors.Is(err, ErrConflict) and
// carries the supplied field-level details so WriteError can surface them
// in the response envelope's error.details member. WriteError maps it to
// 409 conflict and surfaces any supplied details. Calling Conflict() with
// no details is valid and yields a plain conflict error with no details —
// WriteError then omits error.details entirely (omitempty), rather than
// emitting null or [].
func Conflict(details ...FieldError) error {
	return &conflictError{details: details}
}
