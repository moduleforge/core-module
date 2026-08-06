# Emit Spacing Tokens And Container Utility

## Purpose and scope

Extend `gui/style-dictionary/build-tokens.mjs` so the compiled bundle
(`gui/tokens/dist/tokens.css`) carries the new spacing / container-width surface: typed `@property`
registrations and `:root` baked defaults for every new token, one new `@theme inline` key, and a
compiler-generated `@utility container` block wiring the tokens into Tailwind's established container
idiom.

This is the architecturally novel task in the plan. The mechanism is fully designed and empirically
verified — do not redesign it; implement the shape specified below and in
[`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md). If the emitted output
disagrees with what that note predicts, **halt and report** rather than adjusting the design.

Only `gui/style-dictionary/build-tokens.mjs` is edited. No standard skill covers this; follow the
`## Procedure` below.

## Requirements

### 1. Wire the new sources into the correct Style Dictionary instance

The compiler runs **three** separate instances for the reason its header comment explains (the
`mf.text-body` colour/typography collision). The new spacing sources are mode-independent and belong
to the **first** instance only — the one currently resolving `base/color.json`, `base/radius.json`,
`semantic/color.light.json`, `semantic/radius.json`, `component/overrides.json` into
`colorLightTokens`.

Add `tokens/base/spacing.json` and `tokens/semantic/layout.json` to that instance's `source` array.

**Do not** add them to the dark instance or the typography instance. Adding them to the dark instance
would be harmless only by accident (the dark set is filtered to `$type === 'color'`), and adding them
to the typography instance would produce duplicate tokens in `lightTokens`.

The variable name `colorLightTokens` is now a misnomer — it has always in fact held "mode-independent
tokens plus the light colour set" (it already carries radius). Renaming it to something accurate
(e.g. `lightAndModeIndependentTokens`) is welcome but optional; if you rename, update every use site
including the `lightColorNames` sanity check.

Once wired, the existing generic machinery gives you two of the three deliverables for free:

- `propertyBlocks` emits `@property --mf-<name>-default { syntax: "<length>"; … }` for every new
  token, since `SYNTAX_BY_TYPE.dimension === '<length>'` already.
- `rootBlock` emits `--mf-<name>-default: <literal>;` for every new token in `:root`.

Neither needs new code. Confirm both actually happen rather than assuming it.

**The new tokens must not appear in the scoped `data-mf-theme` selectors.** They are
mode-independent, exactly like radius and typography. The existing `$type === 'color'` filters on
`lightColorTokens` and `colorDarkTokens` already exclude them; verify this in the emitted output
rather than trusting it.

### 2. Add one `@theme inline` key

Append to the `themeLines` construction:

```css
  --container-content: var(--mf-max-content-width, var(--mf-max-content-width-default));
```

Tailwind v4's `--container-*` namespace backs the `max-w-*` utilities, so this yields a
`max-w-content` utility — a cheap adoption affordance for a consumer that wants the width token
without the whole container utility. Follow the existing `COLOR_THEME_MAP` / `FONT_THEME_MAP` /
`RADIUS_THEME_MAP` structure: add a small named map rather than a one-off inline string, so the
file's existing "declare the mapping as data, render it uniformly" shape is preserved.

### 3. Emit the `@utility container` block

This is the new emission. Generate it from the token sources — the band list and multipliers must be
**read from `$extensions["com.moduleforge.breakpoint"]`**, never hardcoded a second time in the
compiler. Follow `radiusMultiplierOf`'s precedent: a small lookup helper that throws with a clear
message when the expected extension is missing, so a malformed token source fails the build loudly
rather than emitting silently-wrong CSS.

The emitted block, with `<k>` values pulled from the sources:

```css
@utility container {
  margin-inline: auto;
  max-width: var(--mf-max-content-width, var(--mf-max-content-width-default));
  padding-inline: var(--mf-content-margins-lr-base, calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * 1));
  padding-block: var(--mf-content-margins-tb-base, calc(var(--mf-content-margins-tb, var(--mf-content-margins-tb-default)) * 1.5));
  @variant sm {
    padding-inline: var(--mf-content-margins-lr-sm, calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * 1.5));
    padding-block: var(--mf-content-margins-tb-sm, calc(var(--mf-content-margins-tb, var(--mf-content-margins-tb-default)) * 1.5));
  }
  @variant md { /* … md multipliers … */ }
  @variant lg { /* … lg multipliers … */ }
  @variant xl { /* … xl multipliers … */ }
  @variant 2xl { /* … 2xl multipliers … */ }
}
```

Points that are load-bearing and must not be varied:

- **`@variant <band>`, not a literal `@media`.** `@variant` resolves against whatever `--breakpoint-*`
  theme the *consuming* build has, so only the band **names** are baked into mod-core, never the
  values. A downstream app that customizes `--breakpoint-lg` gets a container whose gutter step moves
  with it. (`@variant 2xl` is valid — the leading digit is fine; this was verified.)
- **The `base` band's declarations are unwrapped**, at the top of the block, not inside a variant.
- **Band order must be ascending** (`base`, `sm`, `md`, `lg`, `xl`, `2xl`). All bands are min-width,
  so at any viewport the *last matching* declaration wins; emitting them out of order silently
  inverts the ladder. Sort explicitly by breakpoint order rather than relying on source order or on
  the `byName` comparator (which would put `2xl` first alphabetically).
- **The outer `var()` takes the per-band lever; the `calc()` is its fallback.** This is what gives
  a style package's per-band override precedence over the derived value for that band alone, while
  the base lever `--mf-content-margins-lr` still rescales every band that has no explicit override.
- **Place the block after `@theme inline`** in the emitted file, with an explanatory comment in the
  same voice as the file's existing block comments — covering both consequences in the section below,
  so a reader of the generated CSS is not surprised.

### 4. Document the two container behavior changes in the emitted comment and the compiler header

The compiler's header comment enumerates what the bundle carries (items 1–3). Add a fourth item for
the `@utility container` block, and state both consequences:

- **`@utility container` extends, not replaces.** Tailwind emits the built-in `.container` rule
  *and* this one, in that order. Because the custom rule comes second and media queries add no
  specificity, its unconditional `max-width` **overrides Tailwind's entire built-in per-breakpoint
  max-width ladder**. That is intended — `--mf-max-content-width` takes ownership of the width — but
  it is a behavior change for any consumer already using `.container`.
- **`.container` gains `padding-block`,** which Tailwind's built-in container never had. Intended;
  it is what `--mf-content-margins-tb` is for. Ordinary utilities (`py-0`, `py-8`) are emitted after
  the container rules and still win, so the escape hatch is intact.

### Do not

- Do not edit any file under `gui/tokens/*.json` — those are task `001`'s output. If a source is
  wrong, halt and report.
- Do not edit `gui/tokens/CONTRACT.md`, `README.md`, or `STYLE-PACKAGE-CONTRACT.md` — Phase 2.
- Do not edit `gui/.ladle/styles.css`. The `@utility` block belongs in the compiled bundle, which
  that file already `@import`s; an `@utility` block reached through an `@import` works (verified).
- Do not commit `gui/tokens/dist/tokens.css` — the directory is gitignored.
- Do not change any existing `@theme` key, `@property` registration, or scope selector.

## Validation

**Environment first.** `gui/node_modules` may be broken in this checkout (its `tailwindcss` entries
symlink into a `bounded-width-task-editor/…` path that no longer exists). Run `bun install` inside
`gui/` before any build step. If `bun install` fails, halt and report rather than working around it.

1. `cd gui && bun run build:tokens` exits 0 and its console line reports a light-token count 15
   higher than before this task (3 scalars + 12 per-band).
2. `gui/tokens/dist/tokens.css` contains an `@property --mf-max-content-width-default` block with
   `syntax: "<length>"` and `initial-value: 80rem`, and 14 further `@property` blocks for the other
   new tokens.
3. Its `:root` block contains `--mf-max-content-width-default: 80rem;` and all 14 other new
   `-default` declarations.
4. **The new tokens appear nowhere in the scoped selectors.** Confirm that no
   `[data-mf-theme="light"]`, `[data-mf-theme="dark"]`, `.dark`, or `[data-mf-theme="inverse"]` block
   contains `max-content-width` or `content-margins`.
5. Its `@theme inline` block contains exactly one new line,
   `--container-content: var(--mf-max-content-width, var(--mf-max-content-width-default));`, and no
   existing line changed.
6. The `@utility container` block is present, declares `margin-inline`, `max-width`,
   `padding-inline`, and `padding-block` at the base band, and carries five `@variant` blocks in the
   order `sm`, `md`, `lg`, `xl`, `2xl`, each with a `padding-inline` and a `padding-block`.
7. Every per-band declaration has the exact three-level shape
   `var(--mf-content-margins-<axis>-<band>, calc(var(--mf-content-margins-<axis>, var(--mf-content-margins-<axis>-default)) * <k>))`,
   and each `<k>` matches its token's `$extensions` multiplier.
8. **Compile through Tailwind and inspect the result** — this is the check that actually proves the
   mechanism, and it must not be skipped: `cd gui && bun run build:css` exits 0. Then compile a probe
   that exercises the class (the class appears nowhere in `gui/src`, so the production `build:css`
   will legitimately purge it):

   ```sh
   cd gui
   printf '<div class="container py-0"></div>' > /tmp/container-probe.html
   ./node_modules/.bin/tailwindcss -i .ladle/styles.css -o /tmp/container-probe.css --content /tmp/container-probe.html
   ```

   In `/tmp/container-probe.css`, confirm all four: (a) **two** `.container` rules are emitted; (b)
   the built-in one comes **first** and the token-backed one **second** — if this order is ever
   inverted the design breaks, so assert it explicitly; (c) each `@variant` became a `@media (width >= …)`
   with the Tailwind default value for its band (`sm`→`40rem`, `md`→`48rem`, `lg`→`64rem`,
   `xl`→`80rem`, `2xl`→`96rem`); (d) `.py-0` is emitted *after* both `.container` rules.
9. `cd gui && bun run typecheck` and `cd gui && bun run test` both pass (no regression; neither
   should be affected).
10. `git status` shows exactly one modified file, `gui/style-dictionary/build-tokens.mjs`, and no
    new tracked files.

## Metadata

architectural_impact: true

## Assumptions

- Task `001` has landed and both new token source files are present and well-formed.
- The three Tailwind mechanics this design rests on hold at the pinned `@tailwindcss/cli` 4.3.1 —
  `@utility container` extends rather than replaces, `@utility` works from an `@import`ed file, and
  `@variant <band>` (including `2xl`) compiles inside an `@utility` body. All three were verified
  empirically during planning; validation step 8 re-verifies them against the real bundle.
- Style Dictionary's `allTokens` exposes `$extensions` on resolved tokens — `radiusMultiplierOf`
  already relies on this, so the same access pattern works.

## References

- [`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md) — the design record:
  precedence semantics, why the hybrid was chosen over discrete-per-breakpoint and fluid-`clamp()`,
  and the exact value tables. Read first.
- [`plan/notes/tailwind-container-mechanics.md`](../notes/tailwind-container-mechanics.md) — the
  measured Tailwind behavior, including the verified probe input and output, the built-in container's
  exact emitted shape, and the utility-ordering findings.
- `gui/style-dictionary/build-tokens.mjs` — the file being edited. Its header comment and the
  `radiusMultiplierOf` / `RADIUS_THEME_MAP` pattern are the precedents to follow.
- `gui/tokens/CONTRACT.md` — the fallback-chaining contract and the settable-vs-internal rule this
  emission must satisfy.
- `gui/.ladle/styles.css` — the Tailwind entry point that `@import`s the compiled bundle. Read to
  understand the build graph; do not edit.

## Checkpoint hints

- After wiring the new sources into the first Style Dictionary instance and confirming the
  `@property` blocks and `:root` defaults appear.
- After adding the `--container-content` `@theme inline` key.
- After emitting the `@utility container` block and confirming validation step 8's Tailwind compile.
- After updating the compiler header comment.

## Status

**Outcome: succeeded.** Implemented 2026-08-06 on branch `plan/gui-spacing-tokens-01-002`.

### Affected source files

- `gui/style-dictionary/build-tokens.mjs` — the only file changed. `git status` is otherwise clean;
  no new tracked files. `gui/tokens/dist/tokens.css` and `gui/dist/index.css` are both regenerated
  build products and both gitignored (`gui/.gitignore` lines 7 and 2), so neither is committed.

### What was implemented

1. `tokens/base/spacing.json` and `tokens/semantic/layout.json` joined the **first** Style
   Dictionary instance only. The dark and typography instances are untouched. The existing generic
   `propertyBlocks` / `rootBlock` machinery picked up all 15 new tokens with no new code, and the
   existing `$type === 'color'` filters kept them out of every scoped selector — all three
   confirmed against the emitted bundle rather than assumed.
2. A new `LAYOUT_THEME_MAP` (mirroring `COLOR_THEME_MAP` / `FONT_THEME_MAP` / `RADIUS_THEME_MAP`)
   appends the single `--container-content` key to `@theme inline`.
3. A new `containerUtilityBlock()` renders the `@utility container` block after `@theme inline`,
   preceded by an explanatory block comment in the file's existing voice covering both behavior
   changes. Band list and multipliers are read from `$extensions["com.moduleforge.breakpoint"]`;
   `BAND_ORDER` is the ordering authority only, and a band the sources name but it does not throws.
4. The compiler header comment gained item 4 for the `@utility container` block, stating both
   consequences; items 1–3 were touched only to add "layout"/"container" to their enumerations.

### Validation summary — all 10 checks passed

| # | Check | Result |
|---|-------|--------|
| 1 | `bun run build:tokens` exits 0, light-token count +15 | passed — 79 → **94** |
| 2 | `@property --mf-max-content-width-default` (`<length>`, `80rem`) + 14 more | passed |
| 3 | `:root` carries all 15 new `-default` declarations | passed |
| 4 | New tokens absent from all 4 scoped `data-mf-theme` / `.dark` blocks | passed |
| 5 | `@theme inline` gained exactly one line; no existing line changed | passed |
| 6 | `@utility container` shape + 5 `@variant` blocks in order `sm,md,lg,xl,2xl` | passed |
| 7 | Every per-band declaration has the exact three-level shape and source multiplier | passed |
| 8 | `bun run build:css` exits 0; probe compile confirms (a)–(d) | passed |
| 9 | `bun run typecheck` and `bun run test` | passed — 53 tests, 0 fail |
| 10 | `git status` shows only `gui/style-dictionary/build-tokens.mjs` | passed |

Checks 2–7 were additionally asserted mechanically, including a byte-comparison proving the rest of
the bundle is identical to the pre-task baseline — so no existing `@property` registration, `@theme`
key, or scope selector changed. Check 8's four sub-assertions were scripted rather than eyeballed:
exactly two `.container` rules; the built-in first (offset 7675) and the token-backed one second
(offset 8011); the five `@variant`s compiled to `@media (width >= 40rem/48rem/64rem/80rem/96rem)`;
and `.py-0` emitted after both, inside the same `@layer utilities`. The new helpers' nine
failure paths were each exercised in isolation and confirmed to throw with their intended message.

`bun install` was not needed — the worktree's `gui/node_modules` was already provisioned and healthy
(the broken-symlink condition the `## Validation` preamble warns about did not apply here).

### Decisions made

- Renamed `colorLightTokens` to `lightAndModeIndependentTokens` (the optional rename the task
  sanctions), updating all four use sites including the `lightColorNames` sanity check.
- `BAND_ORDER` is declared as the *ordering authority only*. Which bands are emitted, and every
  multiplier, come from the token sources; an ordering could not be derived from them because the
  `com.moduleforge.breakpoint` extension carries no ordinal. A band present in the sources but
  absent from `BAND_ORDER` throws rather than landing in an arbitrary position.
- The unwrapped band is `BAND_ORDER[0]`, asserted present, rather than a second `'base'` literal.
- Added four guard rails beyond the one the task named, all in the same fail-loudly spirit:
  `requireToken` for the three scalars the block names outright, a duplicate-band check, an
  empty-ladder check, and a both-axes-cover-the-same-bands check.

### Notes for later phases

`gui/dist/index.css` (the published bundle) **does** already contain a compiled `.container` rule —
both before and after this task. `plan/notes/token-shape-decision.md`'s "Distribution gap" section
and this task doc's validation-8 parenthetical both predict the class is purged; it is not, because
Tailwind's candidate extractor picks up the identifier `container` from `const { container } =
render(...)` in `gui/src/*.test.tsx`. Baseline `dist/index.css` had 6 `.container{` rules (the
built-in, split by minification); it now has 12. This changes nothing about the design or this
task's output — `tokens/dist/tokens.css` matches the note's specified shape exactly — but the
distribution gap's *conclusion* still holds for a different reason: `dist/index.css` carries zero
`@utility` at-rules, so a downstream consumer's own Tailwind pass still cannot define the utility
from the published bundle, and the incidental `.container` rule that is there today depends on an
unrelated test-file identifier.
