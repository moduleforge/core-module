# Verify CSS Output End To End In A Real Consumer And Full Regression

## Purpose and scope

Verify the `dist/index.css` produced by task 001 actually works for a real consumer and that
nothing else in `mod-core/gui` broke. Two parts: (1) a cross-repo yalc smoke-check that
`@moduleforge/core-gui/styles.css` resolves to real compiled CSS from mod-users' `gui/`, and
(2) the full mod-core regression chain (gui typecheck + test, root `make build`).

**Hard dependency:** runs after task 001 — it consumes task 001's `gui/dist/index.css`.

No standard skill covers this; follow the `## Procedure` below. mod-users is **read-only**:
used purely as a live yalc consumer. Do not commit anything in mod-users and do not edit any
tracked mod-users file; restore any transient yalc state when done.

## Requirements

1. **Rebuild and re-confirm the artifact.** From `mod-core/gui`, run `bun run build` and
   confirm `gui/dist/index.css` exists, is non-empty, and contains real compiled Tailwind
   output (theme CSS vars + utility classes actually used in `gui/src/`). This is the package
   that will be published to the consumer.

2. **yalc-publish and link into mod-users' gui.** Per mod-users/AGENTS.md "First-time
   setup" §4:
   - From `mod-core/gui`, run `yalc publish` (publishes `@moduleforge/core-gui` — including
     the new `dist/index.css` — to the local yalc store).
   - Link it into mod-users' gui. `mod-users/gui/.yalc/` is not currently populated in this
     checkout and `mod-users/gui/package.json` has core-gui only as an optional peer (`"*"`),
     so run the link from `mod-users/gui` (`yalc add @moduleforge/core-gui` then
     `bun install`, or `yalc update` if a `file:.yalc/` link is already present). Note that
     `yalc add` will transiently modify `mod-users/gui/package.json` (adds a `file:.yalc/`
     link) and populate the gitignored `mod-users/gui/.yalc/` dir.

3. **Confirm the export resolves to real CSS in the consumer — not just that a file exists.**
   From `mod-users/gui`, verify that `@moduleforge/core-gui/styles.css`:
   - resolves (via the package `exports` map) to the yalc-linked
     `.yalc/@moduleforge/core-gui/dist/index.css` — resolve it explicitly (e.g.
     `bun`/Node module resolution of the `@moduleforge/core-gui/styles.css` subpath, or
     inspect the linked package's `exports`), since mod-users does not currently import it
     anywhere; and
   - the resolved file is **non-empty and contains genuine compiled Tailwind CSS** — the same
     theme CSS variables and real utility-class rules verified in task 001 (grep for
     `--background`/`--foreground`/`--primary` and utilities like `bg-background` /
     `text-foreground`). "Renders styled output" is demonstrated by the resolved stylesheet
     carrying actual compiled rules, not an empty/missing file. Optionally drive a throwaway
     bundle (in a scratch dir, not inside mod-users) that imports the subpath to confirm a
     bundler resolves and includes the CSS. **Do not edit any tracked mod-users file** to
     perform this check.

4. **Restore mod-users to a clean state.** After the smoke-check, revert any transient yalc
   changes so mod-users has no uncommitted tracked changes: restore
   `mod-users/gui/package.json` to its committed state (e.g. `yalc remove @moduleforge/core-gui`
   or `git -C <mod-users> checkout -- gui/package.json`) and leave the gitignored `.yalc/`
   dir as-is or removed. Confirm `git -C <mod-users> status` shows no staged/committed
   changes and no modified tracked files. **Make no commits in mod-users.** Do not touch the
   mod-users `JFnc` followup — it is closed out separately.

5. **Full mod-core regression.** Confirm the change broke nothing else:
   - `cd gui && bun run typecheck` passes (`tsc --noEmit`).
   - `cd gui && bun run test` passes (`bun test --pass-with-no-tests`).
   - `make build` at the `mod-core` root passes (builds model, api, and gui; the gui build
     now also emits `dist/index.css`).

## Validation

- `gui/dist/index.css` exists, is non-empty, and greps clean for theme CSS vars and used
  utility classes (repeat of task 001's content check on the freshly rebuilt artifact).
- `yalc publish` (core-gui) and the link into `mod-users/gui` complete without error.
- `@moduleforge/core-gui/styles.css` resolves from `mod-users/gui` to the yalc-linked
  `dist/index.css`, and that file is non-empty real compiled CSS (theme vars + utilities
  present) — evidence captured in the task report (e.g. resolved path + a few grep'd rule
  names).
- `git -C <mod-users-root> status` is clean: no modified tracked files, no commits. Any
  transient `package.json`/`.yalc/` yalc state has been restored.
- `cd gui && bun run typecheck` passes.
- `cd gui && bun run test` passes.
- `make build` at mod-core root passes.
- `git status` in `mod-core` shows no unexpected changes outside `gui/` (build artifacts in
  `gui/dist/` are gitignored) and the plan task doc.

## Assumptions

- Task 001 has landed: `gui/package.json`'s `build` script emits `dist/index.css` via the
  Tailwind v4 CLI, and `@tailwindcss/cli` is a devDependency.
- `yalc` (available on PATH) is the sanctioned local-link mechanism per mod-users/AGENTS.md;
  the mod-core repo is the yalc *source*, mod-users the *consumer*.
- mod-users' gui may need `bun install` after `yalc add` for the `file:.yalc/` link to
  resolve; this is expected and must be reverted afterward.

## References

- [Reproduction and toolchain findings](../notes/reproduction-and-toolchain.md) — consumer
  state, yalc workflow, and the "resolve the export, don't just check the file exists"
  requirement (mod-users has no existing import of the subpath).
- `mod-users/AGENTS.md` "First-time setup" §4 — the yalc setup steps and gitignored `.yalc/`
  convention.
- `mod-users/gui/yalc.lock`, `mod-users/gui/package.json` — current consumer yalc state.
- `mod-core/Makefile` — root `make build` (delegates to model, api, gui).
- `gui/package.json` — `typecheck` and `test` scripts.

## Checkpoint hints

- After the yalc publish + link and confirming the export resolves to real CSS.
- After restoring mod-users to a clean state.
- After the full mod-core regression (typecheck, test, root make build) passes.

## Status

- **Outcome:** succeeded — 2026-07-19.
- **Artifact re-confirmed:** `gui/dist/index.css` rebuilt via `bun run build` (and again via
  root `make build`) is 29,973 bytes, non-empty, and greps clean for theme CSS vars
  (`--background:oklch(...)`, `--foreground:oklch(...)`, `--primary:oklch(...)`) and used
  utility classes (`.bg-background{...}`, `.text-foreground{...}`, `.bg-primary{...}`).
- **yalc smoke-check:** `yalc publish` from `mod-core/gui` (`@moduleforge/core-gui@0.0.0`
  published), then from `/Users/zane/playground/moduleforge/mod-users/gui`:
  `yalc add @moduleforge/core-gui` + `bun install`. Resolved
  `@moduleforge/core-gui/styles.css` from `mod-users/gui` via
  `bun -e "await import.meta.resolve(...)"`, which returned
  `.../mod-users/gui/node_modules/@moduleforge/core-gui/dist/index.css` (exports-map
  resolution, not a manual path guess). That resolved file is byte-identical (`diff` clean,
  29,973 bytes) to both the yalc-published `.yalc/@moduleforge/core-gui/dist/index.css` copy
  and to mod-core's freshly built `gui/dist/index.css`, and greps clean for the same theme
  vars and utility classes as above.
- **mod-users restored:** `yalc remove @moduleforge/core-gui` reverted
  `mod-users/gui/package.json`'s transient `file:.yalc/` dependency line; `bun install`'s
  `bun.lock` diff (3 lines, `@moduleforge/core-gui` resolution entries) was reverted via
  `git checkout -- bun.lock`; the gitignored `mod-users/gui/.yalc/` dir was removed manually.
  `git -C mod-users diff -- gui/package.json bun.lock` is empty; `git -C mod-users status`
  shows zero changes attributable to this task (pre-existing, unrelated `.flow/session-id`
  modification and untracked `.flow/tasks/*.json` files were already present before this task
  touched the repo — confirmed by checking `git -C mod-users status` before starting the yalc
  workflow — and were left untouched). No commits were made in mod-users.
- **Regression:** `cd gui && bun run typecheck` (tsc --noEmit) passes with no output.
  `cd gui && bun run test` passes (15 pass, 0 fail, 25 expect() calls). `make build` at the
  mod-core root passes (model, api, gui all build; gui build emits `dist/index.css`).
- **mod-core worktree:** `git status` clean throughout (gui/dist/ is gitignored; no source
  changes were required — this was a pure verification task).
- **Dependencies:** `bun install` was run inside `gui/` per the dispatch's
  `dependencies_installed: none` note (root-level lockfile detection gap), matching the
  supplied instruction.
