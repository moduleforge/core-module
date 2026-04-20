# Phase 6, Task 4 — tsup build

## Context
core-module/gui compiles to ESM + CJS + type declarations via tsup. Config was scaffolded in Phase 1; this task fills in the real build.

## Acceptance
- `npm run build` in core-module/gui produces:
  - `dist/index.js` (CJS)
  - `dist/index.mjs` (ESM)
  - `dist/index.d.ts`
  - sourcemaps
- No warnings about unbundled externals; react / react-dom stay external.
- `dist/` file size sanity check: <~100kb for the gzipped output.

## How to verify
- `npm run build` exits 0.
- `node -e "console.log(Object.keys(require('./dist/index.js')))"` lists all exports.
- `npm run typecheck` clean.

## Notes
- If `"use client"` directive is needed for Next.js consumers, tsup's `banner` option can inject it at the top of each output file. Add only if users-module/gui hits runtime errors without it.
