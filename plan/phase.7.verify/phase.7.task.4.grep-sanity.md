# Phase 7, Task 4 — grep sanity checks

## Acceptance
All of the following return empty (no matches):
```sh
grep -R "natural_persons" users-module/api/internal/handlers/
grep -R "legal_entities"  users-module/api/internal/handlers/
grep -R "service_accounts" users-module/api/internal/handlers/
grep -R "from '@/components/ui/" users-module/gui/src/
grep -R "MustFromContext" core-module/
```

## How to verify
Run the greps.

## Notes
- `users-module/api/internal/handlers/assume.go` may still use `users` operations and that's fine — it doesn't touch natural_persons etc.
- A match in `auditlog.go` is acceptable only if it's going through `h.coreSvcs.Entity.GetByUUID` and the grep matches a comment — inspect context.
