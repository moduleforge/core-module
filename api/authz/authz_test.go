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
	"github.com/moduleforge/core-api/entity"
)

// stubAuthorizer is a minimal, outside-the-package implementation of authz.Authorizer.
// It is used only to confirm that the interface can be satisfied by consumer code.
type stubAuthorizer struct {
	// denyAction, if non-empty, causes Authorize to return an error for that action.
	denyAction string
}

// Authorize implements authz.Authorizer.
func (s *stubAuthorizer) Authorize(_ context.Context, action string, _ entity.Entity) error {
	if s.denyAction != "" && action == s.denyAction {
		return errors.New("authz: denied")
	}
	return nil
}

// Compile-time assertion: *stubAuthorizer must satisfy authz.Authorizer.
var _ authz.Authorizer = (*stubAuthorizer)(nil)

// TestStubAuthorizerAllows verifies the stub permits actions that are not denied.
func TestStubAuthorizerAllows(t *testing.T) {
	a := &stubAuthorizer{denyAction: "delete"}
	target := entity.NaturalPerson{ID: ptr(int64(1))}

	if err := a.Authorize(context.Background(), "read", target); err != nil {
		t.Fatalf("expected nil error for allowed action, got: %v", err)
	}
}

// TestStubAuthorizerDenies verifies the stub rejects the configured action.
func TestStubAuthorizerDenies(t *testing.T) {
	a := &stubAuthorizer{denyAction: "delete"}
	target := entity.NaturalPerson{ID: ptr(int64(1))}

	if err := a.Authorize(context.Background(), "delete", target); err == nil {
		t.Fatal("expected error for denied action, got nil")
	}
}

// TestAuthorizerWithNilEntityID confirms the interface handles a pre-create entity
// (EntityID() returns nil) without panicking.
func TestAuthorizerWithNilEntityID(t *testing.T) {
	a := &stubAuthorizer{}
	target := entity.NaturalPerson{ID: nil} // pre-create: no ID yet

	if err := a.Authorize(context.Background(), "create", target); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }
