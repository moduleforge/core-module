package service

import (
	"context"
	"testing"
)

// fakePrincipalExtractor is a minimal PrincipalExtractor for testing.
type fakePrincipalExtractor struct {
	p  *Principal
	ok bool
}

func (f *fakePrincipalExtractor) FromContext(_ context.Context) (*Principal, bool) {
	return f.p, f.ok
}

// Compile-time: fakePrincipalExtractor must satisfy PrincipalExtractor.
var _ PrincipalExtractor = (*fakePrincipalExtractor)(nil)

func TestPrincipalExtractor_Interface(t *testing.T) {
	t.Run("authenticated", func(t *testing.T) {
		want := &Principal{UserID: 1, EntityID: 2, IsAdmin: true}
		ext := &fakePrincipalExtractor{p: want, ok: true}

		got, ok := ext.FromContext(context.Background())
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got.UserID != want.UserID || got.EntityID != want.EntityID || got.IsAdmin != want.IsAdmin {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		ext := &fakePrincipalExtractor{ok: false}
		got, ok := ext.FromContext(context.Background())
		if ok {
			t.Fatal("expected ok=false")
		}
		if got != nil {
			t.Fatalf("expected nil principal, got %+v", got)
		}
	})
}
