# Phase 3, Task 8 — Regenerate sqlc in users-module/model

## Context
After 3.5 + 3.7, `make build` regenerates users-module's db package minus the 5 core-table entries.

## Acceptance
- `cd users-module/model && make build` exits 0.
- `users-module/model/db/models.go` no longer has core-table structs.
- `users-module/api` still builds (because call sites already point at coredb from 3.2).

## How to verify
- `grep -l "type Entity struct\|type NaturalPerson struct\|type LegalEntity struct\|type Corporation struct\|type ServiceAccount struct" users-module/model/db/*.go` returns nothing.
- `go build ./...` in users-module/api exits 0.

## Notes
- If users-module/api build fails after regen, a call site was missed in 3.2. Re-run the grep from 3.2's notes.
