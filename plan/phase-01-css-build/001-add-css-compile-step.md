# Add Tailwind CSS Compile Step To The Gui Publish Build

## Purpose and scope

Make `mod-core/gui`'s publish build emit a real, non-empty `dist/index.css` so the existing
`"./styles.css": "./dist/index.css"` export resolves to compiled Tailwind CSS. This is a
build-tooling change confined to `mod-core/gui/`: decide where the publishable Tailwind
source lives, add the Tailwind v4 CLI, and wire a CSS compile step into `gui`'s `build`
script alongside the existing tsup step — without disturbing tsup's JS/TS output.

No standard skill covers this; follow the `## Procedure` below. Scope is `mod-core/gui/`
only — do not touch `model/`, `api/`, or anything in mod-users.

## Requirements

1. **Confirm the reproduction first.** From `gui/`, run `bun run build` (tsup) and confirm
   `gui/dist/` has no `index.css` before making changes. (Expected per the
   [investigation notes](../notes/reproduction-and-toolchain.md); confirm it still holds.)

2. **Decide where the publishable Tailwind source lives.** The stylesheet source is
   `gui/.ladle/styles.css` (full Tailwind v4 theme: `@import "tailwindcss"` /
   `"tw-animate-css"` / `"shadcn/tailwind.css"`, `@theme inline`, `:root`/`.dark` tokens,
   `@layer base`, and `@source "../src"` scanning `gui/src`). Pick an approach that avoids
   duplication and keeps Ladle's preview working — see the two options in the
   [notes](../notes/reproduction-and-toolchain.md#design-options--where-the-publishable-tailwind-source-lives):
   - **Recommended (Option B):** keep `.ladle/styles.css` as the single source of truth and
     point the new CLI step at it directly. Ladle is left untouched. `@source "../src"`
     already resolves to `gui/src` (Tailwind resolves `@source` relative to the CSS file, not
     the CLI cwd).
   - **Alternative (Option A):** relocate the stylesheet to `gui/src/styles.css`, update its
     `@source` so it still scans `gui/src`, and repoint `gui/.ladle/components.tsx`'s
     `import './styles.css'` to the new location so Ladle uses the same single source. If you
     choose this, confirm tsup does not pick up `src/styles.css` (tsup's only entry is
     `src/index.ts`, which does not import the stylesheet).

   Whichever you choose, **verify `@source` still resolves to `gui/src`** so utilities for
   every class used across the component source (including classes inside `cva()` calls) are
   emitted. Do not create a second copy of the stylesheet.

3. **Add `@tailwindcss/cli` as a `devDependency`** in `gui/package.json` at a version
   matching the installed `tailwindcss` (v4; currently `4.3.1`). `@tailwindcss/cli` is the
   Tailwind v4 standalone CLI and is not currently installed. Install so the lockfile
   (`gui/bun.lock`) updates. Do not add it to `dependencies` — it is a build-time tool.

4. **Wire the CSS compile step into `gui/package.json`'s `build` script** so a single
   `bun run build` produces both the tsup JS/TS output and `dist/index.css`. Use the
   Tailwind v4 CLI, e.g. `tailwindcss -i <source.css> -o dist/index.css --minify` run
   alongside/before tsup (compose with tsup via the `build` script — a `build:css` +
   `build:js` split invoked by `build`, or a direct `&&`/parallel composition, is fine).
   Requirements on the wiring:
   - `bun run build` alone must emit `dist/index.css`; no extra manual step.
   - The existing tsup step, its config, and its JS/TS/`.d.ts`/sourcemap outputs must be
     unchanged. Note tsup runs with `clean: true` (it wipes `dist/` on run) — order the CSS
     compile so its output survives (e.g. run the CSS step after tsup, or ensure the CSS
     step is not clobbered by tsup's clean). Confirm both artifacts coexist in `dist/` after
     a single `bun run build`.

5. **Update the gui build docs.** Add a brief note (one line is enough) that the gui build
   now also emits `dist/index.css`, in whichever of `gui/README.md` / `mod-core/AGENTS.md`
   currently describes the gui build. Do not over-document; a single accurate sentence.

## Validation

- `cd gui && bun run build` succeeds and, after it, `gui/dist/index.css` **exists and is
  non-empty**.
- `gui/dist/index.css` contains genuine compiled Tailwind output — verify by grep:
  - theme CSS variables from the source (e.g. `--background`, `--foreground`, `--primary`,
    `--radius`, and `.dark` overrides) are present;
  - real utility class rules used by the components are present (e.g. classes such as
    `bg-background`, `text-foreground`, `border-border`, and utilities emitted from the
    components under `gui/src/` — spot-check a few actually used in the source).
- `gui/dist/` still contains the unchanged tsup outputs (`index.js`, `index.mjs`,
  `index.d.ts`, `index.d.mts`, and `.map` files) after `bun run build`.
- Ladle preview still builds: `cd gui && bun run preview:build` (`ladle build`) succeeds,
  confirming the source-location decision did not break the dev preview.
- If Option A was chosen: confirm no duplicate stylesheet exists and
  `gui/.ladle/components.tsx` imports the relocated file; confirm `src/styles.css` did not
  leak into tsup output.
- `@tailwindcss/cli` appears in `gui/package.json` `devDependencies` and `gui/bun.lock` is
  updated.
- `git status` in `mod-core` shows changes only under `gui/` (and the plan task doc); no
  changes to `model/`, `api/`, or mod-users.

## Assumptions

- `tailwindcss@4.3.1` and `@tailwindcss/vite@4.3.1` are already installed in `gui/`; only
  `@tailwindcss/cli` needs adding.
- `gui/dist/` currently has no `index.css` (confirmed during planning; re-confirm at start).
- Full end-to-end consumer verification (yalc into mod-users) and the full regression chain
  (typecheck, test, root `make build`) are handled by the follow-on task 002, which depends
  on this task's `dist/index.css` output.

## References

- [Reproduction and toolchain findings](../notes/reproduction-and-toolchain.md) — confirmed
  reproduction, current Ladle-only wiring, toolchain versions, and the source-location
  design options.
- `gui/tsup.config.ts` — existing publish build (must stay unchanged); note `clean: true`.
- `gui/package.json` — `build` script, `exports["./styles.css"]`, `files`, devDependencies.
- `gui/.ladle/styles.css` — the Tailwind v4 stylesheet source (`@source "../src"`).
- `gui/.ladle/components.tsx` — Ladle's `import './styles.css'` (repoint only under Option A).
- `gui/vite.config.ts` — `@tailwindcss/vite` (Ladle preview; leave untouched).

## Checkpoint hints

- After adding `@tailwindcss/cli` and confirming a bare CLI compile produces expected CSS.
- After wiring the `build` script and confirming `bun run build` emits both JS and CSS.
- After updating the build docs and confirming Ladle preview still builds.
