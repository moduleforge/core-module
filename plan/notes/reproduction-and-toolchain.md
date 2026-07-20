# Reproduction And Toolchain Findings

## Purpose and scope

Investigation notes captured while planning the `gui-css-build` change. Records the
confirmed reproduction of the missing `dist/index.css`, the current CSS/Tailwind wiring,
the available toolchain, and the design options for where the publishable Tailwind source
should live. These findings ground the task documents in `plan/phase-01-css-build/`.

## Reproduction (confirmed)

- `gui/package.json` declares `"./styles.css": "./dist/index.css"` in `exports`, and lists
  `dist` in `files`, so consumers can `import '@moduleforge/core-gui/styles.css'`.
- The publish build is `"build": "tsup"` → `gui/tsup.config.ts` with `entry: ['src/index.ts']`,
  `format: ['cjs','esm']`, `dts: true`. tsup processes JS/TS only — it never emits CSS.
- Current `gui/dist/` contents: `index.js`, `index.mjs`, `index.d.ts`, `index.d.mts`, and
  their `.map` files. There is **no `index.css`**. So `./styles.css` resolves to a
  non-existent file for consumers. Reproduction confirmed by direct inspection of
  `gui/dist/` (no rebuild needed; the missing artifact is structural, not stale).
- This matches the mod-users followup `JFnc` (mod-users/plan/followups.yaml), surfaced by
  the `<ErrorBanner>` adoption task, which never chased it down.

## Current Tailwind wiring (Ladle-only)

- The real Tailwind v4 stylesheet source is `gui/.ladle/styles.css`:
  `@import "tailwindcss"`, `@import "tw-animate-css"`, `@import "shadcn/tailwind.css"`,
  `@custom-variant dark`, a full `@theme inline { … }` block, `:root` / `.dark` token
  definitions, and an `@layer base` block. Crucially it has `@source "../src";` — relative
  to `gui/.ladle/`, this resolves to `gui/src`, so Tailwind scans the component source
  (including classes inside `cva()` calls) and emits utilities for every class used.
- It is wired into the **Ladle dev-preview build only**: `gui/.ladle/components.tsx` does
  `import './styles.css';`, and `gui/vite.config.ts` registers `@tailwindcss/vite`. Ladle's
  Vite build is the only thing that ever compiles this stylesheet. The publish build (tsup)
  never touches it.

## Toolchain (verified in gui/node_modules)

- `tailwindcss` — v4.3.1 (installed).
- `@tailwindcss/vite` — v4.3.1 (installed; used by Ladle).
- `@tailwindcss/cli` — **NOT installed.** This is the Tailwind v4 standalone CLI
  (`tailwindcss -i <input.css> -o <output.css>`), the natural fit for compiling CSS
  alongside tsup. It must be added as a `devDependency` (v4, matching `tailwindcss`).
- `bun` 1.3.14; `yalc` 1.0.0-pre.53 available on PATH.

Note: tsup itself cannot process Tailwind — it has no PostCSS/Tailwind pipeline for a
standalone `.css` entry. A separate CSS compile step is required; `@tailwindcss/cli` is the
idiomatic v4 tool for this.

## Design options — where the publishable Tailwind source lives

Tailwind v4 resolves `@source` relative to the CSS file that declares it, not the CLI's
cwd, so `@source` correctness depends only on the file's own location.

- **Option B (recommended): keep `.ladle/styles.css` as the single source of truth.**
  Point the new CLI build step at it directly:
  `tailwindcss -i .ladle/styles.css -o dist/index.css`. `@source "../src"` already resolves
  correctly (`.ladle/` → `gui/src`) regardless of the CLI's cwd. Ladle preview is left
  completely untouched. Zero duplication, lowest risk. The only quibble is that the publish
  build now sources from a conventionally Ladle-owned directory.

- **Option A: relocate to `gui/src/styles.css`.** Move the stylesheet into `src/`, change
  its `@source` to `.` (the file would sit in the dir it scans), and repoint Ladle's
  `.ladle/components.tsx` import to `../src/styles.css`. Both the CLI build and Ladle then
  read the same relocated file (still single source, no duplication). Extra care needed:
  (1) `@source` must be updated to keep scanning `gui/src`; (2) confirm tsup does **not**
  pick up `src/styles.css` — tsup's only entry is `src/index.ts`, and `index.ts` does not
  import the stylesheet, so tsup will ignore it (verify this holds).

Either option satisfies "avoid duplication + keep Ladle working." Option B is the smaller,
lower-risk change and is the recommended default; the implementer may choose A if keeping a
dev-tool directory out of the publish path is judged worth the extra moves. The choice must
be verified by confirming `@source` still resolves to `gui/src` and Ladle preview still
builds.

## Consumer-verification notes (mod-users, read-only)

- mod-users' `gui/` is the live consumer with the yalc workflow (mod-users/AGENTS.md
  "First-time setup" §4; `gui/yalc.lock` lists `@moduleforge/core-gui`).
- Current state: `mod-users/gui/.yalc/` is **not populated** in this checkout, and
  `mod-users/gui/package.json` has `@moduleforge/core-gui` only as an optional peer
  dependency (`"*"`) — no `file:.yalc/` link present. So verification must run
  `yalc publish` (in core-gui) then link into mod-users' gui.
- mod-users does **not** currently `import '@moduleforge/core-gui/styles.css'` anywhere; its
  Ladle preview uses its own `mod-users/gui/.ladle/styles.css`. So the consumer smoke-check
  cannot rely on an existing import — it must resolve the export explicitly (e.g. resolve
  `@moduleforge/core-gui/styles.css` from mod-users/gui and content-check the resolved file),
  and/or drive a throwaway bundle that imports it, **without editing tracked mod-users files**.
- Hard constraint: no commits in mod-users, and any transient edits the yalc workflow makes
  to `mod-users/gui/package.json` (the `file:.yalc/` link) plus the gitignored `.yalc/` dir
  must be left in a clean state — restore `package.json` to its committed state when done.
  The separate mod-users followup `JFnc` is closed out from the consumer side later, not here.
