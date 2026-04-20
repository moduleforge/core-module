# Phase 6 — Extract UI components into core-module/gui

## Goal
Move the profile-editing UI (and any entity-form components used across the app) out of users-module/gui into core-module/gui as a presentational component library. Re-export the shadcn primitives the components need so users-module (and future consumers) don't need a matching shadcn setup. users-module/gui consumes via yalc.

## Preconditions
- Phase 1 complete (gui scaffold exists).
- Phase 5 complete (API routes `/v1/self`, `/v1/entities/*` served by core, so component API calls can go to stable endpoints).

## Outputs
- `core-module/gui/src/ProfileEditor.tsx` — presentational profile-edit form; props only, no data fetching.
- `core-module/gui/src/NaturalPersonForm.tsx`, `CorporationForm.tsx`, `ServiceAccountForm.tsx` — primitive entity forms used inside ProfileEditor and admin views.
- `core-module/gui/src/ui/` — bundled shadcn primitives: `button.tsx`, `input.tsx`, `label.tsx`, `card.tsx`, `badge.tsx`, `alert.tsx`.
- `core-module/gui/src/index.ts` — barrel exports.
- `core-module/gui/dist/` — tsup output (ESM + CJS + dts).
- `core-module/gui/package.json` — full dep list including `@radix-ui/*`, `lucide-react`, `class-variance-authority`, `clsx`, `tailwind-merge`.
- `users-module/gui/.yalc/@moduleforge/core-gui/` (gitignored) present after `make link-core`.
- `users-module/gui/package.json` has `"@moduleforge/core-gui": "file:.yalc/@moduleforge/core-gui"`.
- `users-module/gui/tailwind.config.ts` (or equivalent) includes `.yalc/@moduleforge/core-gui/dist/**/*.{js,mjs}` in `content`.
- `users-module/gui/src/app/profile/page.tsx` reduced to a thin page that mounts `<ProfileEditor>`.
- `users-module/gui/src/app/admin/users/[uuid]/page.tsx` uses core forms where applicable.

## Hard rules
- **Presentational only.** Components don't call `fetch` or `api`; they accept `initialValue` / `onSave` props.
- **No Next.js imports.** No `"use client"` hardcoded inside core components (consumer adds the directive at the page level). No imports from `next/*` or `@/*` path aliases.
- **Consistent styling.** Components use Tailwind class names; consumer's Tailwind config picks up the dist glob.
- **No breaking visual change.** Before/after screenshots at `/profile` match.

## Tasks
- 6.1 Extract ProfileEditor
- 6.2 Extract NaturalPerson/Corporation/ServiceAccount forms
- 6.3 Extract shadcn primitives
- 6.4 tsup build
- 6.5 yalc publish
- 6.6 users-module/gui consumes core-gui
- 6.7 Tailwind content glob
- 6.8 Visual smoke

## How to verify
- `cd core-module/gui && npm run build && npm run typecheck` clean.
- `make link-core` succeeds.
- `cd users-module/gui && npm run build` succeeds with components rendered from `.yalc/@moduleforge/core-gui`.
- Visiting `/profile` in dev renders identical UI; edits save via the same `PUT /v1/self` endpoint.
- `grep -R "ProfileEditor\|NaturalPersonForm" users-module/gui/src/` shows imports from `@moduleforge/core-gui`, not local paths.

## Notes
- The current profile page (`users-module/gui/src/app/profile/page.tsx`) uses `useAuth()` and `api.self.update()`. After extraction, the page owns those concerns; ProfileEditor receives the resolved data.
- Shadcn primitives in users-module currently live at `users-module/gui/src/components/ui/*.tsx`. Copy these into core-module/gui/src/ui/ (same file names) and re-export. users-module/gui keeps its own copies during migration and deletes them once imports are switched.
