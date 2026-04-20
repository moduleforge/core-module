# Phase 4, Task 1 — audit.Writer interface

## Context
core-module services emit audit entries via an injected interface. The signature mirrors users-module's existing `audit.Writer` so users-module's concrete writer satisfies it structurally (no adapter code).

## Acceptance
- File `core-module/api/audit/audit.go`:
  ```go
  // Package audit defines the Writer interface consumer-provided audit
  // implementations must satisfy. core-module services never write to
  // audit_log directly; that table lives in the consumer module.
  package audit

  import "context"

  type Writer interface {
      Write(ctx context.Context, op, resource string, targetEntityID *int64, before, after any) error
  }
  ```
- Package godoc explains the contract (actor resolved from ctx by the implementation).

## How to verify
- `go build ./audit` exits 0.
- `go vet ./audit` exits 0.

## Notes
- Do not add helper constructors or mocks in this file — mocks belong in `audit_test.go` or a `audittest` subpackage if needed later.
