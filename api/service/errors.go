package service

import "errors"

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrForbidden is returned when the caller lacks permission for the operation.
var ErrForbidden = errors.New("forbidden")

// ErrInvalidInput is returned when the caller supplies invalid or missing input.
var ErrInvalidInput = errors.New("invalid input")
