# Phase 7 — Verification report

## Automated checks (passed)

| # | Check | Result |
|---|---|---|
| 7.1 | `make test` in core-module/model | pass (no tests — generated code) |
| 7.1 | `make test` in core-module/api | pass (service 77.1%, httpapi 71.7% coverage) |
| 7.1 | `make test` in users-module/model | pass (no tests) |
| 7.1 | `make test` in users-module/api | pass (auth, config, handlers, handlers/auth) |
| 7.1 | `npm run typecheck` in core-module/gui | pass |
| 7.1 | `pnpm build` in users-module/gui | pass (earlier, Phase 6) |
| 7.4 | `grep natural_persons\|legal_entities\|service_accounts users-module/api/internal/handlers/` | no matches |
| 7.4 | Shadcn primitives remaining local: `dialog.tsx`, `separator.tsx`, `switch.tsx`, `table.tsx` | expected (migrated: button, input, label, card, badge, alert) |
| 7.4 | `grep "from '@moduleforge/core-gui'" users-module/gui/src/` | 17 import sites |
| 7.5 | `make clean && make build` in core-module/model — no git diff on `db/` | idempotent |
| 7.5 | `make clean && make build` in users-module/model — no git diff on `db/` | idempotent |

## Manual checks — pending user verification

These require a running dev stack (Docker + DB) which was not available during implementation:

- **7.2 `make dev.start` end-to-end smoke** — start full stack, log in, edit profile at `/profile`, reload to confirm persistence; create a user via admin; edit an existing user.
- **7.3 `atlas migrate status`** — against a freshly-migrated local DB, confirm all 16 migrations applied in order: `0000–0005` (core) then `0100–0109` (users-module).
- **7.6 Audit log** — after a profile edit, verify `SELECT op, resource, actor_user_id FROM audit_log ORDER BY at DESC LIMIT 5` shows an entry with `op='update'`, `resource='natural_person'`, `actor_user_id` matching the editing user; `before`/`after` JSON populated.

## Docs updated

- `users-module/plan/summary.md` — added "Dependencies on core-module" section describing how model, api, and gui each consume core-module.
- `user-components/README.md` — already reflects the current layout (Phase 1).
- No `users-module/CLAUDE.md` exists; the root `CLAUDE.md` was not modified (it describes the overall goals, not module-specific details).

## Schema-only plan supersession

The prior schema-only plan in `core-module/plan/summary.md` was rewritten in Phase 1 (commit 9e83838) to cover the full API + UI extraction. No separate archive step is needed.

## Known open items

1. `EntityService.GetByID` in core-api returns `ErrNotFound` unconditionally (sqlc didn't emit `GetEntityByID`; only `GetEntityByUUID`). Not blocking — no caller uses it today. Add the sqlc query if needed.
2. `ServiceAccountService.UpdateByEntityUUID` returns `ErrInvalidInput` because no `UpdateServiceAccount` query exists. Wiring is in place; add the sqlc query when service account updates become a real use case.
3. Entity forms (`<NaturalPersonForm>`, `<CorporationForm>`, `<ServiceAccountForm>`) exist in core-gui but the admin user-edit page still renders its own inline form. Consolidation was out of scope for Phase 6 ("leave admin pages alone — we can consolidate later").
4. Integration test for `PUT /v1/self` is unit-level (mocked) because users-module/api has no testcontainer harness. If a DB-backed harness is added later, upgrade the test.

## Summary

All automatable Phase 7 checks pass. The extraction is complete end-to-end: core-module owns the entity schema + service + HTTP + UI surface, users-module composes and consumes it. Manual smoke (7.2, 7.3, 7.6) is the last gate before sign-off.
