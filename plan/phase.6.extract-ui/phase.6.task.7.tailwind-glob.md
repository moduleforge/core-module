# Phase 6, Task 7 — Tailwind content glob

## Context
Tailwind scans `content` globs for class names. After the migration, components come from `.yalc/@moduleforge/core-gui/dist/`, so that path must be in the globs.

## Acceptance
- `users-module/gui/tailwind.config.ts` (or `.js` / `.mjs` — whatever exists) adds to `content`:
  ```ts
  './.yalc/@moduleforge/core-gui/dist/**/*.{js,mjs,cjs}',
  ```
  in addition to existing paths.

## How to verify
- `npm run build` in users-module/gui.
- Visiting `/profile` in dev and in production build — all Tailwind classes applied (no "missing class" regressions).
- `grep -c "bg-primary" users-module/gui/.next/static/css/*.css` returns > 0 (proves core-gui's classes made it into the bundle).

## Notes
- If users-module/gui is on Tailwind v4 (config-less mode via CSS `@source`), add `@source "./.yalc/@moduleforge/core-gui/dist";` to the main CSS file instead.
