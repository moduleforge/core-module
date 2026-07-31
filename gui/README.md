# @moduleforge/core-gui

React component library for entity editing UIs. Provides `<ProfileEditor>` and related entity-form components for use in consumer Next.js applications.

## Build

```bash
npm run build
```

Outputs `dist/index.js` (CJS), `dist/index.mjs` (ESM), and `dist/index.d.ts` (types) via tsup, plus `dist/index.css` (compiled Tailwind theme + utilities via the Tailwind v4 CLI) for consumers importing `@moduleforge/core-gui/styles.css`. `@tailwindcss/cli`'s pinned version must be bumped in lockstep whenever `tailwindcss` or `@tailwindcss/vite` is upgraded, or the publish-build CSS compiler and the Ladle dev-preview compiler can drift to different Tailwind engine versions.

`dist/index.css` is compiled from the DTCG design-token sources under [`tokens/`](tokens/README.md) via `style-dictionary/build-tokens.mjs`; see that directory's README for the token tiering model before editing any source file under `tokens/`, and see [`tokens/CONTRACT.md`](tokens/CONTRACT.md) for the fixed `--mf-*` consumption + scoping contract that governs how the compiled tokens are consumed. See [`tokens/STYLE-PACKAGE-CONTRACT.md`](tokens/STYLE-PACKAGE-CONTRACT.md) for the runtime **style-package** artifact contract — how a brand's compiled override bundle + brand-asset manifest is built and loaded (`src/lib/theme-loader.ts`) at runtime, without rebuilding the app or `mod-core`.

## Consuming this package from an app

An app that composes this module (e.g. `app-mftodo`) wires `@moduleforge/core-gui` in through
a **bun workspace** it owns — a root `package.json` listing this repo's `gui/` among its
`workspaces`, with the app's own `gui/package.json` referring to it as `workspace:*`. This repo
does not declare or manage that workspace; see
[`docs/mf-standards/building-applications.md`'s First-time setup
section](./docs/mf-standards/building-applications.md#first-time-setup) for the mechanism, and
[`docs/mf-standards/building-modules.md`'s Cross-module GUI dependencies
section](./docs/mf-standards/building-modules.md#cross-module-gui-dependencies) for what a
consumer's `gui/package.json` must satisfy to be workspace-consumable. A single `bun install` at
the composing app's workspace root links this package in; there is no publish/link step here.

## Tailwind content glob requirement

This library ships pre-built JS that contains Tailwind class names as strings. For Tailwind's content scanner to detect those classes, consumers must add the dist path to their Tailwind `content` configuration (or, for Tailwind v4's `@source` directive, the equivalent scan path):

```js
// tailwind.config.js (or equivalent), or an @source directive under Tailwind v4
content: [
  // ... existing globs
  './node_modules/@moduleforge/core-gui/dist/**/*.{js,mjs}',
]
```

Without this glob, Tailwind will purge the component styles and the UI will render unstyled.
