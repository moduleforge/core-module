# mod-core/gui — deferred component workbench follow-ups

The initial Ladle setup (`gui/.ladle/`, `make preview`) covers the minimum: every exported component has at least one story, Tailwind + the shadcn theme tokens render correctly, and HMR works. Items below were intentionally left out of the first pass.

## Story coverage

- Any component added to `gui/src/` after the initial scaffold needs a matching `*.stories.tsx` next to it. Keep stories co-located with their component.
- Variants we punted on:
  - `ProfileEditor` — loading state (`onSave` pending for > 250ms), server-error state with a retryable failure from `onSave`.
  - `NaturalPersonForm` / `CorporationForm` / `ServiceAccountForm` — an `isSubmitting={true}` snapshot to verify the disabled-button styling independently of timing.
  - Dark-mode variants — see "Dark mode toggle" below.

## Storybook migration path

Stories are written in CSF, so the swap is config-only:
1. `npm remove @ladle/react` and `npm add -D @storybook/react-vite @storybook/addon-essentials`.
2. Rename `.ladle/` → `.storybook/` and swap `config.mjs` for Storybook's `main.ts` + `preview.ts` (import the same `styles.css`).
3. Keep `vite.config.ts` as-is; Storybook's `react-vite` framework consumes it.
4. Port the `dev` script: `ladle serve --port 61000` → `storybook dev -p 61000`.

Consider this only once the libraries are published externally or grow into a documented design system. For an internal niche app, Ladle's speed is the right trade.

## Addons to turn on when useful

- **a11y**: Ladle 5 ships axe-core integration. Flip `addons.a11y.enabled = true` in `.ladle/config.mjs`; stories then show a violations panel.
- **Theme toggle**: add `addons.theme` for a light/dark switcher that sets the `.dark` class on the story root. Pairs with the `.dark` CSS vars already defined in `.ladle/styles.css`.
- **MSW decorator**: once stories need realistic API fixtures, register `msw` in `.ladle/components.tsx` via the `Provider` export and define per-story handlers.

## Visual regression

No built-in visual-regression story for Ladle. Options:
- **Playwright snapshot on `ladle build` output**: `npm run preview:build` produces a static `build/`; serve it with `http-server` and drive Playwright to screenshot each story URL (`/?story=<id>`). This is the cheapest path.
- **Chromatic** requires Storybook — only worth it if we're migrating anyway.

## Typecheck story coverage

Current `npm run typecheck` (`tsc --noEmit`) includes stories, which gives us basic type safety for free. It assumes `@ladle/react` is installed — `make preview` runs `npm install` first, so this is normally fine. If we add a fresh-clone lint flow, add `npm install` ahead of `typecheck` there too.

## Aggregated multi-module index

If someone wants a single URL covering both core and tags previews, the simplest approach is a tiny `index.html` served from a parent directory that links to each module's `ladle build` static output. Not worth the plumbing yet.

## Composite stories that use core-gui primitives

No tag stories currently compose core-gui; if one does in the future, yalc-link core-gui into `mod-tags/gui` (mirror the root `link-core` pattern under a module-level `preview-link` target) so the story can resolve `@moduleforge/core-gui` at dev time.
