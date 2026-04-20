# Phase 6, Task 6 — users-module/gui consumes core-gui

## Context
Add the yalc link, replace local imports with `@moduleforge/core-gui`, delete the now-redundant local components.

## Acceptance
- `cd users-module/gui && yalc add @moduleforge/core-gui` — records `file:.yalc/@moduleforge/core-gui` in package.json.
- `npm install` in users-module/gui so the link resolves.
- Update imports:
  - `src/app/profile/page.tsx` — replace inline form with `<ProfileEditor initial={profile} onSave={api.self.update} />`. Keep the page's use of `useAuth` to resolve `initial`.
  - `src/app/admin/users/[uuid]/page.tsx` — use `<NaturalPersonForm>` where applicable.
  - Replace `import { Button } from '@/components/ui/button'` with `import { Button } from '@moduleforge/core-gui'` across the codebase (grep + sed).
- Delete `users-module/gui/src/components/ui/{button,input,label,card,badge,alert}.tsx` and any others now sourced from core-gui.
- Delete `users-module/gui/src/lib/utils.ts` if it only contained `cn()` (now in core-gui) — or keep if other things live there.

## How to verify
- `cd users-module/gui && npm run build` succeeds.
- `grep -R "from '@/components/ui/" users-module/gui/src/` returns nothing (all migrated).
- `grep -R "ProfileEditor" users-module/gui/src/` shows only the import from `@moduleforge/core-gui`.

## Notes
- If any local file depended on a primitive that WASN'T migrated to core-gui, either (a) copy that primitive into core-gui too (if it's truly shared) or (b) leave it in users-module.
- Do this migration in one PR to avoid a half-migrated state.
