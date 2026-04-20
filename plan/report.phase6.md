# Phase 6 Report — Extract UI components into core-module/gui

## What was migrated

### core-module/gui (new package `@moduleforge/core-gui@0.0.0`)

**Components created:**
- `src/ProfileEditor.tsx` — presentational profile-edit form; exports `ProfileData`, `ProfileEditorProps`, `ProfileEditor`. No fetch, no Next.js imports. Accepts `initial`, `onSave`, `readOnly` props.
- `src/NaturalPersonForm.tsx` — controlled `given_name` + `family_name` inputs per `natural_persons` schema.
- `src/CorporationForm.tsx` — controlled `legal_name` + `jurisdiction` inputs per `corporations` schema.
- `src/ServiceAccountForm.tsx` — controlled `label` input per `service_accounts` schema.

**Shadcn primitives copied into `src/ui/`:**
- `button.tsx`, `input.tsx`, `label.tsx`, `card.tsx`, `badge.tsx`, `alert.tsx`
- Imports changed from `@/lib/utils` to `../lib/utils` (relative).

**Supporting files:**
- `src/lib/utils.ts` — `cn()` helper.
- `src/ui/index.ts` — barrel export for all primitives.
- `src/index.ts` — top-level barrel.

**Runtime deps added:**
- `class-variance-authority@^0.7.1`, `clsx@^2.1.1`, `lucide-react@^0.468.0`, `radix-ui@^1.4.3`, `tailwind-merge@^3.5.0`

### users-module/gui

- `src/app/profile/page.tsx` reduced to thin mount importing `<ProfileEditor>` from `@moduleforge/core-gui`.
- All 17 files importing `button/input/label/card/badge/alert` from `@/components/ui/*` updated to `@moduleforge/core-gui`.
- `dialog.tsx` updated to import `Button` from `@moduleforge/core-gui` (was blocking build).
- Deleted: `button.tsx`, `input.tsx`, `label.tsx`, `card.tsx`, `badge.tsx`, `alert.tsx` from `src/components/ui/`.
- Kept: `dialog.tsx`, `separator.tsx`, `switch.tsx`, `table.tsx`.
- `@source "../../.yalc/@moduleforge/core-gui/dist";` added to `globals.css`.
- `.yalc/` and `yalc.lock` added to `.gitignore`.

## Build status

| Repo | Command | Result |
|------|---------|--------|
| `core-module/gui` | `npm run build` | PASS |
| `core-module/gui` | `npm run typecheck` | PASS |
| `users-module/gui` | `pnpm build` | PASS — 16 routes, 0 errors |

## Package manager

`users-module/gui`: **pnpm** (pnpm workspace at `users-module/`).

## Commits

**core-module (main):**
1. `Phase 6a+6b: core-gui components (ProfileEditor, entity forms, shadcn primitives) with tsup build passing`

**users-module (main):**
2. `Phase 6c: users-module/gui consumes @moduleforge/core-gui via yalc`
3. `Phase 6d: Tailwind picks up core-gui dist in content glob`

## Deviations

1. **`dialog.tsx` also needed updating.** It internally imported `Button` from the local file; fixed to use `@moduleforge/core-gui`.
2. **`is_email_verified` not on `UserSelf`.** `UserSelf` has no `email_verified_at`. Profile wrapper passes `is_email_verified: false` as a stub.
3. **Entity forms not yet wired into admin pages.** Per spec: admin `[uuid]/page.tsx` was left with its own inline form; only its primitive imports were updated. Wiring `NaturalPersonForm` is deferred.

## Remaining manual verification steps

1. Run `pnpm dev` in `users-module/gui`, visit `/profile` — verify form renders identically and saves via `PUT /v1/self`.
2. Verify dark mode renders correctly on `/profile`.
3. Visit `/admin/users/<uuid>` — confirm edit form still functions.
4. Confirm `yalc push @moduleforge/core-gui` in `users-module/gui` picks up future component changes.
5. Verify `@source` path resolves correctly from your build environment (path is relative to `src/app/globals.css`).
