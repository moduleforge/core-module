# @moduleforge/core-gui

React component library for entity editing UIs. Provides `<ProfileEditor>` and related entity-form components for use in consumer Next.js applications.

## Build

```bash
npm run build
```

Outputs `dist/index.js` (CJS), `dist/index.mjs` (ESM), and `dist/index.d.ts` (types) via tsup.

## Linking into a consumer (yalc workflow)

In this package:

```bash
yalc publish
```

In the consumer app (e.g. `users-module/gui`):

```bash
yalc add @moduleforge/core-gui
npm install
```

To push updates after rebuilding:

```bash
# in core-module/gui
npm run build && yalc push
```

## Tailwind content glob requirement

This library ships pre-built JS that contains Tailwind class names as strings. For Tailwind's content scanner to detect those classes, consumers must add the dist path to their Tailwind `content` configuration.

**When linked via yalc:**

```js
// tailwind.config.js (or equivalent)
content: [
  // ... existing globs
  './.yalc/@moduleforge/core-gui/dist/**/*.{js,mjs}',
]
```

**When installed via npm:**

```js
content: [
  // ... existing globs
  './node_modules/@moduleforge/core-gui/dist/**/*.{js,mjs}',
]
```

Without this glob, Tailwind will purge the component styles and the UI will render unstyled.
