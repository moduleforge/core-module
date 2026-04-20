# Phase 6, Task 8 — Visual smoke test

## Context
Before/after comparison confirms no visual regression from the extraction.

## Acceptance
Manual checklist:
1. `make dev.start` in users-module.
2. Log in as the local admin user.
3. Navigate to `/profile`. Confirm:
   - Layout matches pre-extraction (header, card, inputs, buttons).
   - Fields pre-filled with current user's given_name, family_name.
   - Edit given_name → Save → success badge appears → reload → value persists.
4. Navigate to `/admin/users/[some uuid]`. Confirm:
   - Entity fields render via core forms (if migrated).
   - Edit + save flows work.
5. Visual diff: if screenshot tooling exists, capture baseline pre-migration and compare. Otherwise, take screenshots manually.

## How to verify
All checklist items pass without regression. Attach screenshots in a `report.phase6.md` if desired.

## Notes
- Any visual regression in this task is a blocker for phase sign-off. Most likely cause: Tailwind glob missing a class. Re-check Phase 6.7.
