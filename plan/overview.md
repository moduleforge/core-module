# gui-css-build — Plan Overview

## Purpose and scope

Make `mod-core/gui`'s publish build actually produce a working `dist/index.css` so that
consumers importing `@moduleforge/core-gui/styles.css` receive real compiled Tailwind CSS
(theme tokens plus every utility class used across `gui/src/`), instead of resolving to a
file that is never generated.

**Problem.** `gui/package.json` exports `"./styles.css": "./dist/index.css"`, but the
publish build is tsup (`gui/tsup.config.ts`, entry `src/index.ts`, JS/TS only) and never
emits any CSS. The real Tailwind v4 stylesheet lives at `gui/.ladle/styles.css` and is wired
only into the Ladle dev-preview build (`@tailwindcss/vite` in `gui/vite.config.ts`), never
through the publish build. Result: `dist/index.css` does not exist and `./styles.css`
resolves to nothing for consumers. See
[reproduction and toolchain findings](./notes/reproduction-and-toolchain.md).

**Goal.** Add a Tailwind v4 CSS compile step (via `@tailwindcss/cli`) to the gui publish
build so `bun run build` alone produces both the existing tsup JS/TS output **and** a
non-empty, correct `dist/index.css`, then verify the fix end-to-end — locally, in a real
consumer (mod-users' `gui/`) via yalc, and through the full regression chain (gui typecheck
+ test, root `make build`).

**Scope / hard constraints.**
- mod-core `gui/` only. `model/` and `api/` are untouched.
- Do **not** disturb existing tsup JS/TS output or behavior.
- Do **not** commit anything in mod-users, and do **not** edit tracked mod-users files.
  mod-users' `gui/` is used purely as a live yalc consumer to verify against; any transient
  yalc edits to `mod-users/gui/package.json` and its gitignored `.yalc/` dir must be
  restored to a clean state afterward. The mod-users-side followup (`JFnc`) is closed out
  separately, not in this plan.
- No `docs/*-spec.md` or `docs/architecture.md` exist in this project; there is no spec or
  architecture doc to reconcile. There are no architectural implications (the `./styles.css`
  export boundary already exists — this only makes it resolve to real content), so no
  documentation-updates phase is warranted. A one-line note in `gui`'s build docs that the
  build now also emits CSS is folded into the implementation task.

## Current status

Reproduction is confirmed: `gui/dist/` contains the tsup JS/TS/`.d.ts` outputs but no
`index.css`. `tailwindcss@4.3.1` and `@tailwindcss/vite@4.3.1` are installed;
`@tailwindcss/cli` is **not** yet installed and must be added. `yalc` is available.
mod-users' `gui/.yalc/` is not currently populated. The plan begins at
**Phase 01 — CSS Build**, task 001. Task 002 (verification) has a hard dependency on task
001's `dist/index.css` output and runs after it.

## Overview

Single phase, two sequential tasks (task 002 depends on task 001's output — not
parallel-eligible).

### Phase 01 — CSS Build

- **001 — Add Tailwind CSS compile step to the gui publish build**
  (`plan/phase-01-css-build/001-add-css-compile-step.md`). Confirm the reproduction, decide
  where the publishable Tailwind source lives (recommended: keep `gui/.ladle/styles.css` as
  the single source of truth and point a new CLI step at it; relocating to `gui/src/styles.css`
  is an acceptable alternative — see the [design options](./notes/reproduction-and-toolchain.md)),
  add `@tailwindcss/cli` as a devDependency, and wire a Tailwind v4 CLI compile step into
  `gui/package.json`'s `build` script so `bun run build` produces both the existing tsup JS/TS
  output and a non-empty `dist/index.css`. Verify locally that `dist/index.css` contains the
  expected theme CSS variables and utility classes, that Ladle's preview build still works,
  and that existing tsup output is unchanged. Add a one-line note to the gui build docs that
  the build now emits CSS.

- **002 — Verify CSS output end-to-end in a real consumer and full regression**
  (`plan/phase-01-css-build/002-verify-consumer-and-regression.md`). yalc-publish the rebuilt
  core-gui and link it into mod-users' `gui/`, confirm `@moduleforge/core-gui/styles.css`
  resolves to the real compiled CSS (not just that a file exists) and carries genuine
  Tailwind output, then restore mod-users to a clean, uncommitted state. Finally run
  `cd gui && bun run typecheck && bun run test` and the root `make build` to confirm nothing
  else broke. Depends on task 001.
