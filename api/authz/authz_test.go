// Package authz_test confirms that the Authorizer interface is implementable
// from outside the authz package and that the interface signature compiles as
// specified.
//
// There is no behavior to test on the interface itself; these tests exist to
// catch any future accidental mutation of the interface signature and to serve
// as a compilation-time contract check.
package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moduleforge/core-api/authz"
)

// stubAuthorizer is a minimal, outside-the-package implementation of authz.Authorizer.
// It is used only to confirm that the interface can be satisfied by consumer code.
type stubAuthorizer struct {
	// denyOperation, if non-empty, causes Authorize to return an error for that operation.
	denyOperation string
}

// Authorize implements authz.Authorizer.
func (s *stubAuthorizer) Authorize(_ context.Context, operation string, _ *int64) error {
	if s.denyOperation != "" && operation == s.denyOperation {
		return errors.New("authz: denied")
	}
	return nil
}

// Compile-time assertion: *stubAuthorizer must satisfy authz.Authorizer.
var _ authz.Authorizer = (*stubAuthorizer)(nil)

// TestStubAuthorizerAllows verifies the stub permits operations that are not denied.
func TestStubAuthorizerAllows(t *testing.T) {
	a := &stubAuthorizer{denyOperation: "delete"}
	id := int64(1)

	if err := a.Authorize(context.Background(), "read", &id); err != nil {
		t.Fatalf("expected nil error for allowed operation, got: %v", err)
	}
}

// TestStubAuthorizerDenies verifies the stub rejects the configured operation.
func TestStubAuthorizerDenies(t *testing.T) {
	a := &stubAuthorizer{denyOperation: "delete"}
	id := int64(1)

	if err := a.Authorize(context.Background(), "delete", &id); err == nil {
		t.Fatal("expected error for denied operation, got nil")
	}
}

// TestAuthorizerWithNilTarget confirms the interface handles a nil target
// (e.g. create / list / search) without panicking.
func TestAuthorizerWithNilTarget(t *testing.T) {
	a := &stubAuthorizer{}

	if err := a.Authorize(context.Background(), "create", nil); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}
