# Phase 5, Task 7 — Update handlers/auditlog.go UUID resolution

## Context
`auditlog.go:77` currently calls `GetEntityByUUID` against users-module/model/db. After Phase 3, that call was moved to coredb. Optionally clean up by going through the core service layer for consistency.

## Acceptance
Choose one:
- **(a)** Keep the coredb direct call (simplest). Confirm it still builds after Phase 4/5 changes.
- **(b)** Switch to `h.coreSvcs.Entity.GetByUUID(ctx, uuid)` for consistency with the rest of the codebase.

Preferred: **(b)** — consistent service-layer usage.

## How to verify
- `go build` clean.
- `GET /v1/audit/{object_uuid}` returns expected behavior.

## Notes
- The `object_uuid` path parameter can resolve to any entity (natural_person, corporation, service_account). The core Entity service's `GetByUUID` already handles that.
