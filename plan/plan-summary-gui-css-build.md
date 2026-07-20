# gui-css-build — Session Summary

## What was planned and why

`mod-core/gui`'s publish build never produced a working `dist/index.css`.
`gui/package.json` exports `"./styles.css": "./dist/index.css"`, but the publish build
(tsup, entry `src/index.ts`) only ever emitted JS/TS/`.d.ts` artifacts — the real Tailwind
v4 stylesheet lived at `gui/.ladle/styles.css` and was wired only into the Ladle
dev-preview build via `@tailwindcss/vite`, never into the publish path. As a result,
`dist/index.css` did not exist, and any consumer importing
`@moduleforge/core-gui/styles.css` (e.g. mod-users' `gui/`) resolved to nothing.

The goal was to add a Tailwind v4 CSS compile step (via `@tailwindcss/cli`) to the gui
publish build so `bun run build` alone produces both the existing tsup JS/TS output **and**
a non-empty, correct `dist/index.css`, then verify the fix end-to-end — locally, in a real
consumer (mod-users' `gui/`) via yalc, and through the full regression chain (gui typecheck
+ test, root `make build`). Scope was confined to `mod-core/gui`; `model/` and `api/` were
untouched, and no tracked mod-users files were to be modified or committed.

## What shipped

**Phase 01 — CSS Build** (single phase, two sequential tasks; task 002 depended on task
001's `dist/index.css` output):

- **001 — Add Tailwind CSS compile step to the gui publish build**
  (merge `383ecb2`, commit `a2ddd26`). Added a Tailwind v4 CSS compile step to gui's publish
  build so `dist/index.css` (already promised by `gui/package.json`'s `"./styles.css"`
  export) is actually produced. Kept `gui/.ladle/styles.css` as the single Tailwind source
  (Option B), added `@tailwindcss/cli` pinned to `4.3.1`, and wired the build script to
  `"tsup && bun run build:css"` so the CSS step survives tsup's `clean: true`. Verified
  `bun run build` emits both the five original tsup artifacts and a genuine ~29KB minified
  `index.css` with expected theme vars, dark-mode overrides, and component-sourced utility
  classes. Confirmed Ladle's preview build still succeeds untouched. All changes confined to
  `gui/`.

- **002 — Verify CSS output end-to-end in a real consumer and full regression**
  (merge `4292986`, commit `efaf632`). Re-ran mod-core/gui's build and confirmed
  `dist/index.css` is a real, non-empty (29,973-byte) compiled Tailwind stylesheet with
  expected theme CSS variables and utility classes. Published via yalc from mod-core/gui and
  linked into mod-users/gui per mod-users/AGENTS.md §4, then used Bun's
  `import.meta.resolve` to prove `@moduleforge/core-gui/styles.css` resolves through the
  package's real exports map to the yalc-linked `dist/index.css`, byte-identical to both the
  yalc store copy and mod-core's freshly built artifact. Fully reverted mod-users afterward
  (yalc remove + git checkout on `bun.lock`); `git -C mod-users status` showed zero
  tracked-file changes attributable to this task and no commits were made. Ran the full
  mod-core regression — gui typecheck, gui tests (15/15 passing), and root `make build`
  (model + api + gui) — all green.

- **Post-review fix (no task doc)** (commit `e86a06d`). After the phase-boundary review,
  the manager applied a trivial one-line fix directly to the plan branch: a note added to
  `gui/README.md` stating that `@tailwindcss/cli`'s pinned version must be bumped in
  lockstep with `tailwindcss`/`@tailwindcss/vite` whenever either changes, to avoid
  CSS-compiler version drift between the publish build and the Ladle dev-preview.

## Key decisions

- **Kept `.ladle/styles.css` as the single Tailwind source (Option B)**, rather than
  relocating the stylesheet to `gui/src/styles.css`. This avoided disturbing the existing
  Ladle dev-preview wiring (`@tailwindcss/vite` in `gui/vite.config.ts`) while still letting
  the new CLI step target the same source of truth for the publish build.
- **Exact-pinned `@tailwindcss/cli` to `4.3.1`**, matching the already-installed
  `tailwindcss@4.3.1` / `@tailwindcss/vite@4.3.1`, to avoid CSS-compiler version drift
  between the two build paths (publish build vs. Ladle preview). This constraint is now
  documented in `gui/README.md` (see the post-review fix above) so future dependency bumps
  keep all three packages in lockstep.
- **Sequenced the build script as `tsup && bun run build:css`** rather than running the two
  steps in parallel or in the reverse order, specifically because tsup's `clean: true`
  option wipes `dist/` on each run — running the CSS compile step after tsup ensures
  `dist/index.css` survives.

## Follow-up items

No open follow-up items from this plan were recorded in `plan/followups.yaml` — the entries
currently present there belong to other, unrelated plans (`gui-test-infrastructure`,
`apiresp-action-required`).

Two additional items were identified during phase review but could not be recorded to
`followups.yaml` (the file is mid-schema-migration v1→v2 in a separate concurrent process),
and are carried forward here as freestanding notes:

- **Pre-existing documentation link-chain gap.** `gui/README.md`, `model/README.md`,
  `next-steps.md`, and `stories-next.md` are not reachable from the project's root
  `README.md`. This gap predates this plan and is unrelated to this plan's diff — not fixed
  here.
- **Pre-existing dependency vulnerabilities.** `bun audit` on `gui/`'s lockfile surfaced 5
  pre-existing moderate/high-severity vulnerabilities in `vite` and `esbuild` (via `tsup`).
  Confirmed present before this plan's changes and unaffected by them.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — CSS Build

- [x] [001-add-css-compile-step.md](./phase-01-css-build/001-add-css-compile-step.md) — tier `sonnet-high` · branch `phase-01-task-01-add-css-compile-step` · commit `a2ddd26` · merge `383ecb2`
- [x] [002-verify-consumer-and-regression.md](./phase-01-css-build/002-verify-consumer-and-regression.md) — tier `sonnet-high` · branch `phase-01-task-02-verify-consumer-and-regression` · commit `efaf632` · merge `4292986`
