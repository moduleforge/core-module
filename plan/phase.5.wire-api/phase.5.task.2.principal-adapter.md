# Phase 5, Task 2 — Principal adapter

## Context
core-module's `service.PrincipalExtractor` must be satisfied by a users-module adapter that reads the existing `auth.UserContext` from ctx and maps it to core's `Principal`.

## Acceptance
- File `users-module/api/internal/auth/core_adapter.go`:
  ```go
  package auth

  import (
      "context"
      "github.com/moduleforge/core-api/service"
  )

  // CorePrincipalAdapter implements service.PrincipalExtractor
  // by reading the users-module UserContext from ctx.
  type CorePrincipalAdapter struct{}

  func (CorePrincipalAdapter) FromContext(ctx context.Context) (*service.Principal, bool) {
      uc, ok := FromContext(ctx)  // non-panicking variant (not MustFromContext)
      if !ok {
          return nil, false
      }
      return &service.Principal{
          UserID:   uc.UserID,
          EntityID: uc.EntityID,
          IsAdmin:  uc.IsAdmin,
      }, true
  }
  ```
- A non-panicking `FromContext(ctx) (UserContext, bool)` function exists in users-module's `auth` package. If only `MustFromContext` exists, add the non-panicking variant — don't change `MustFromContext`.

## How to verify
- `go build ./...` in users-module/api.
- A simple test: `var _ service.PrincipalExtractor = CorePrincipalAdapter{}` compiles.

## Notes
- The `AssumedUser` concept in `UserContext` is not mapped to core's Principal (core doesn't care about assumption — it just sees the effective identity). If `UserID` + `EntityID` in `UserContext` already reflect the assumed user, that's correct; verify in `auth/principal.go`.
