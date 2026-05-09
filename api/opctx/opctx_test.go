package opctx_test

import (
	"context"
	"testing"

	"github.com/moduleforge/core-api/opctx"
)

// TestActorEntityID verifies round-trip set/get and the missing-value case.
func TestActorEntityID(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		ctx := opctx.WithActor(context.Background(), 42)
		id, ok := opctx.ActorEntityID(ctx)
		if !ok {
			t.Fatal("expected ok=true, got false")
		}
		if id != 42 {
			t.Fatalf("expected 42, got %d", id)
		}
	})

	t.Run("not set returns zero and false", func(t *testing.T) {
		id, ok := opctx.ActorEntityID(context.Background())
		if ok {
			t.Fatal("expected ok=false, got true")
		}
		if id != 0 {
			t.Fatalf("expected 0, got %d", id)
		}
	})

	t.Run("overwrite replaces previous value", func(t *testing.T) {
		ctx := opctx.WithActor(context.Background(), 1)
		ctx = opctx.WithActor(ctx, 2)
		id, ok := opctx.ActorEntityID(ctx)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if id != 2 {
			t.Fatalf("expected 2, got %d", id)
		}
	})
}

// TestSudoActorEntityID verifies round-trip set/get and the missing-value case.
func TestSudoActorEntityID(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		ctx := opctx.WithSudoActor(context.Background(), 99)
		id, ok := opctx.SudoActorEntityID(ctx)
		if !ok {
			t.Fatal("expected ok=true, got false")
		}
		if id != 99 {
			t.Fatalf("expected 99, got %d", id)
		}
	})

	t.Run("not set returns zero and false", func(t *testing.T) {
		id, ok := opctx.SudoActorEntityID(context.Background())
		if ok {
			t.Fatal("expected ok=false, got true")
		}
		if id != 0 {
			t.Fatalf("expected 0, got %d", id)
		}
	})
}

// TestRequestID verifies round-trip set/get and the missing-value case.
func TestRequestID(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		ctx := opctx.WithRequestID(context.Background(), "req-abc-123")
		id := opctx.RequestID(ctx)
		if id != "req-abc-123" {
			t.Fatalf("expected %q, got %q", "req-abc-123", id)
		}
	})

	t.Run("not set returns empty string", func(t *testing.T) {
		id := opctx.RequestID(context.Background())
		if id != "" {
			t.Fatalf("expected empty string, got %q", id)
		}
	})

	t.Run("empty string round trips", func(t *testing.T) {
		ctx := opctx.WithRequestID(context.Background(), "")
		// An explicitly set empty string is indistinguishable from "not set"
		// by design — RequestID returns "" in both cases.
		id := opctx.RequestID(ctx)
		if id != "" {
			t.Fatalf("expected empty string, got %q", id)
		}
	})
}

// TestKeysDoNotCollide confirms that setting one key does not affect the others.
func TestKeysDoNotCollide(t *testing.T) {
	ctx := context.Background()
	ctx = opctx.WithActor(ctx, 10)
	ctx = opctx.WithSudoActor(ctx, 20)
	ctx = opctx.WithRequestID(ctx, "req-xyz")

	actorID, ok := opctx.ActorEntityID(ctx)
	if !ok || actorID != 10 {
		t.Errorf("ActorEntityID: want (10, true), got (%d, %v)", actorID, ok)
	}

	sudoID, ok := opctx.SudoActorEntityID(ctx)
	if !ok || sudoID != 20 {
		t.Errorf("SudoActorEntityID: want (20, true), got (%d, %v)", sudoID, ok)
	}

	reqID := opctx.RequestID(ctx)
	if reqID != "req-xyz" {
		t.Errorf("RequestID: want %q, got %q", "req-xyz", reqID)
	}
}

// TestActorDoesNotLeakToSudoActor confirms actor and sudo-actor keys are independent.
func TestActorDoesNotLeakToSudoActor(t *testing.T) {
	// Set only actor; sudo-actor must remain absent.
	ctx := opctx.WithActor(context.Background(), 55)

	_, ok := opctx.SudoActorEntityID(ctx)
	if ok {
		t.Fatal("SudoActorEntityID should not be set when only actor was set")
	}
}

// TestSudoActorDoesNotLeakToActor confirms sudo-actor does not bleed into actor.
func TestSudoActorDoesNotLeakToActor(t *testing.T) {
	// Set only sudo-actor; actor must remain absent.
	ctx := opctx.WithSudoActor(context.Background(), 77)

	_, ok := opctx.ActorEntityID(ctx)
	if ok {
		t.Fatal("ActorEntityID should not be set when only sudo-actor was set")
	}
}
