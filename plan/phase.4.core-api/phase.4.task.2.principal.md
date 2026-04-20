# Phase 4, Task 2 — Principal + PrincipalExtractor

## Context
core-module handlers need caller identity to authorize requests. They get it via a consumer-provided `PrincipalExtractor` — no auth middleware in core.

## Acceptance
- File `core-module/api/service/principal.go`:
  ```go
  package service

  import "context"

  // Principal is the normalized caller identity.
  type Principal struct {
      UserID   int64
      EntityID int64
      IsAdmin  bool
  }

  // PrincipalExtractor lifts a Principal from request context. Consumers
  // implement this against their own auth middleware's context keys.
  // Returns (nil, false) if unauthenticated.
  type PrincipalExtractor interface {
      FromContext(ctx context.Context) (*Principal, bool)
  }
  ```
- No implementation provided in core-module; consumers implement.
- Unit test asserts the interface is satisfied by a minimal test impl.

## How to verify
- `go build ./service` exits 0.
- `go test ./service -run TestPrincipalExtractor` passes (trivial test with fake impl).

## Notes
- `Principal` intentionally does not carry email, roles, or assumed-user info — those belong in the consumer's richer context type. core-module only needs these three fields to authorize entity operations.
