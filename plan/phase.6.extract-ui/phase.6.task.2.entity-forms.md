# Phase 6, Task 2 — Entity form components

## Context
Admin user-edit and future entity admin screens need form components for each entity subtype. Extract minimal forms now; grow later.

## Acceptance
Files:
- `core-module/gui/src/NaturalPersonForm.tsx` — inputs for given_name, family_name; callbacks `onSave`.
- `core-module/gui/src/CorporationForm.tsx` — inputs for legal_name (and other corporation fields from the schema); `onSave`.
- `core-module/gui/src/ServiceAccountForm.tsx` — inputs for name, description; `onSave`.

Each follows the same presentational pattern as ProfileEditor (props in, onSave out; no fetch).

All exported from `core-module/gui/src/index.ts`.

## How to verify
- `npm run build` succeeds.
- `npm run typecheck` clean.

## Notes
- Check current admin user-edit (`users-module/gui/src/app/admin/users/[uuid]/page.tsx`) to see which fields are currently exposed; match its surface so Phase 6.6 is a drop-in replacement.
- If an entity subtype has no admin form today, create a minimal skeleton — we'll fill it out as needed.
