# Phase 1, Task 4 — Scaffold core-module/gui

## Context
`core-module/gui` is a React component library built with tsup, published locally via yalc. Consumers pick up the components as an npm package import.

## Acceptance
- `core-module/gui/package.json`:
  - `"name": "@moduleforge/core-gui"`
  - `"version": "0.0.0"`
  - `"main": "./dist/index.js"`, `"module": "./dist/index.mjs"`, `"types": "./dist/index.d.ts"`
  - `"exports"` map: `"."` → mjs/js/d.ts; `"./styles.css"` → `"./dist/index.css"` (if any CSS emitted).
  - `"files": ["dist"]`
  - Scripts: `"build": "tsup"`, `"typecheck": "tsc --noEmit"`, `"clean": "rm -rf dist .yalc yalc.lock"`
  - `devDependencies`: `tsup`, `typescript`, `@types/react`, `@types/react-dom` (pin to same versions as users-module/gui)
  - `peerDependencies`: `react: "^19"`, `react-dom: "^19"` (match users-module/gui's actual versions)
  - `dependencies`: `lucide-react`, `class-variance-authority`, `clsx`, `tailwind-merge`, `@radix-ui/*` — copy from users-module/gui/package.json only the ones needed by the moved components (defer until Phase 6; scaffold with empty deps block for now).
- `core-module/gui/tsconfig.json` — `jsx: "react-jsx"`, `target: "ES2022"`, `module: "ESNext"`, `moduleResolution: "bundler"`, `strict: true`, `declaration: true`, `emitDeclarationOnly: false` (tsup handles emission).
- `core-module/gui/tsup.config.ts`:
  ```ts
  import { defineConfig } from 'tsup';
  export default defineConfig({
    entry: ['src/index.ts'],
    format: ['cjs', 'esm'],
    dts: true,
    sourcemap: true,
    clean: true,
    external: ['react', 'react-dom'],
  });
  ```
- `core-module/gui/src/index.ts` — empty export file: `export {};`
- `core-module/gui/.gitignore` — `node_modules/`, `dist/`, `.yalc/`, `yalc.lock`.
- `core-module/gui/README.md` — describes:
  - How to build: `npm run build`
  - How to link into a consumer: `yalc publish` here, `yalc add @moduleforge/core-gui` in consumer.
  - **Consumer Tailwind config must add** `"./node_modules/@moduleforge/core-gui/dist/**/*.{js,mjs}"` (or the `.yalc` path) to `content` globs so Tailwind picks up class names.

## How to verify
- `cd core-module/gui && npm install && npm run build` produces `dist/index.js`, `dist/index.mjs`, `dist/index.d.ts` (all effectively empty).
- `npm run typecheck` exits 0.
- `yalc publish` produces a `.yalc` package payload.

## Notes
- Leave dependency versions as `peerDependencies` only where possible — consumer brings React. Use `devDependencies` for build-time tools.
- Bundled shadcn primitives (Phase 6) will add real deps.
