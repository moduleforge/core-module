package apiresp

import "errors"

// invalidInputError wraps ErrInvalidInput and carries the field-level
// details supplied to InvalidInput. Its Unwrap method makes
// errors.Is(err, ErrInvalidInput) succeed for values built by
// InvalidInput; WriteError's fieldErrors accessor uses errors.As to
// recover the concrete type and surface Details in the response envelope.
type invalidInputError struct {
	details []FieldError
}

// Error implements the error interface. It returns the underlying
// sentinel's text; the safe, client-facing message is decided separately
// by publicMessage in WriteError, not by this method.
func (e *invalidInputError) Error() string {
	return ErrInvalidInput.Error()
}

// Unwrap makes errors.Is(err, ErrInvalidInput) succeed for values built by
// InvalidInput.
func (e *invalidInputError) Unwrap() error {
	return ErrInvalidInput
}

// fieldDetails implements fieldDetailCarrier, letting WriteError's
// fieldErrors accessor recover the details carried by an
// invalidInputError.
func (e *invalidInputError) fieldDetails() []FieldError {
	return e.details
}

// InvalidInput builds an error that satisfies errors.Is(err,
// ErrInvalidInput) and carries the supplied field-level details so
// WriteError can surface them in the response envelope's error.details
// member. Calling InvalidInput() with no details is valid and yields a
// plain invalid-input error with no details — WriteError then omits
// error.details entirely (omitempty), rather than emitting null or [].
func InvalidInput(details ...FieldError) error {
	return &invalidInputError{details: details}
}

// fieldDetailCarrier is implemented by any error type that carries
// field-level details for WriteError to surface in the response
// envelope's error.details member — invalidInputError and conflictError
// both implement it. Declared at the point of use so a future
// detail-carrying error type needs no further edits here beyond
// implementing fieldDetails.
type fieldDetailCarrier interface {
	fieldDetails() []FieldError
}

// fieldErrors recovers the field-level details carried by err, if any, via
// errors.As against fieldDetailCarrier. It returns (nil, false) for any
// error that does not implement fieldDetailCarrier — including one that
// merely wraps ErrInvalidInput or ErrConflict directly (e.g. via
// fmt.Errorf), which carries no structured details to recover.
func fieldErrors(err error) ([]FieldError, bool) {
	var carrier fieldDetailCarrier
	if errors.As(err, &carrier) {
		return carrier.fieldDetails(), true
	}
	return nil, false
}
