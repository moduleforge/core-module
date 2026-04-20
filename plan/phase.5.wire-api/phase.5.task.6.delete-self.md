# Phase 5, Task 6 — Delete handlers/self.go

## Context
Core router now serves `/v1/self`. users-module's self.go is dead code.

## Acceptance
- `users-module/api/internal/handlers/self.go` deleted.
- All references in main.go to `NewSelfHandler` / `selfHandler.Get` / `selfHandler.Put` removed.
- `go build ./...` exits 0.

## How to verify
- `grep -r "selfHandler\|NewSelfHandler" users-module/api/` returns nothing.
- Build clean.

## Notes
- If any test file imports from self.go, move the relevant test coverage into core-module/api/httpapi/self_test.go or delete if redundant.
