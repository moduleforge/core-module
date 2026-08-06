#!/usr/bin/env node
/**
 * Compiles the DTCG design-token sources under `mod-core/gui/tokens/` into a single
 * generated CSS bundle at `mod-core/gui/tokens/dist/tokens.css`.
 *
 * The bundle carries:
 *   1. `@property` typed declarations for every semantic/typography/radius/layout token whose
 *      DTCG `$type` maps to a CSS syntax component (color, dimension, number, fontWeight).
 *      Registered on the token's baked-default custom property (`--mf-x-default`), never on
 *      the bare `--mf-x` — see the "Why register on -default, not on the bare var" note below.
 *   2. A Tailwind v4 `@theme inline` block mapping Tailwind's color/font/radius/container theme
 *      keys to the fallback-chained semantic vars (`var(--mf-x, var(--mf-x-default))`), so
 *      existing cva utility classes (`bg-primary`, etc.) resolve through the semantic contract.
 *   3. Baked-default declarations (`--mf-x-default`) for every semantic token, emitted as
 *      mode/scope variable sets keyed on the unified `data-mf-theme` scoping attribute (see
 *      `tokens/CONTRACT.md` for the full contract):
 *        - `:root` carries the full LIGHT default set (colors + typography + radius). Light is
 *          the baseline, active whenever `data-mf-theme` is absent.
 *        - `[data-mf-theme="light"]` re-asserts the light COLOR defaults for a nested subtree
 *          (so a light island inside a dark region flips back to light).
 *        - `[data-mf-theme="dark"]` (with the legacy `.dark` class bridged alongside it for
 *          back-compat) carries the DARK color defaults.
 *        - `[data-mf-theme="inverse"]` is a relative flip to the opposite surface of its
 *          surrounding mode: in a light context it resolves to the dark color set; nested under
 *          a dark scope (`[data-mf-theme="dark"] [data-mf-theme="inverse"]`, `.dark …`) it
 *          resolves to the light color set via the higher-specificity compound selector.
 *      Radius/typography/font-family/layout tokens are mode-independent, so only colors are
 *      re-emitted in the scoped selectors; the mode-independent tokens live once in `:root` and
 *      inherit down unchanged. Sourced from `tokens/semantic/color.light.json` and
 *      `tokens/semantic/color.dark.json` respectively.
 *   4. An `@utility container` block (emitted after the `@theme inline` block) wiring the
 *      `tokens/semantic/layout.json` roles into Tailwind's container idiom: `margin-inline: auto`,
 *      a `max-width` from `--mf-max-content-width`, and a per-breakpoint-band gutter ladder on
 *      both axes. Each per-band declaration is
 *      `var(--mf-content-margins-<axis>-<band>, calc(var(--mf-content-margins-<axis>, …) * k))`,
 *      so a style package's explicit per-band override wins for that band alone while the base
 *      lever still rescales every band that carries no override. Bands are spelled `@variant
 *      <band>`, not a literal `@media`, so only the band NAMES are baked in here and the values
 *      resolve against the consuming build's own `--breakpoint-*` theme. The band list and every
 *      multiplier are read from the sources' `$extensions["com.moduleforge.breakpoint"]`.
 *
 *      Two deliberate behavior changes follow for any consumer already using `.container`, and
 *      are restated in the emitted CSS so a reader of the bundle is not surprised:
 *        - `@utility container` EXTENDS Tailwind's built-in `.container`; it does not replace it.
 *          Tailwind emits the built-in rule first and this one second, and media queries add no
 *          specificity, so this block's unconditional `max-width` overrides the built-in's entire
 *          per-breakpoint max-width ladder (40rem through 96rem), rendering it inert. That is
 *          intended — `--mf-max-content-width` takes ownership of the container's width.
 *        - `.container` gains `padding-block`, which Tailwind's built-in container never had.
 *          Also intended: it is exactly what `--mf-content-margins-tb` is for. Ordinary utilities
 *          (`py-0`, `py-8`) are emitted after both container rules and still win, so the escape
 *          hatch is intact.
 *
 * Regenerate with `npm run build:tokens` (or `bun run build:tokens`) after editing any file
 * under `tokens/`. This script produces no manual post-edits and is deterministic: token lists
 * are always sorted by their compiled CSS custom-property name before being rendered.
 *
 * --- Why three separate Style Dictionary instances, not one merged source list ---
 * `tokens/semantic/color.{light,dark}.json` define a `mf.text-body` COLOR ROLE, while
 * `tokens/typography/scale.json` defines a `mf.text-body` GROUP (its `size` / `line-height` /
 * `weight` / `tracking` sub-tokens). If both files are given to the same Style Dictionary
 * `source` array, Style Dictionary's deep-merge combines them into one object that has both a
 * `$value` (the color) AND nested children (the scale sub-tokens) at the same `mf.text-body`
 * path. Style Dictionary's token flattener stops descending as soon as it finds `$value` on an
 * object, so the four typography sub-tokens are silently dropped with no warning or error. This
 * is a latent naming collision between the color-role tier and the typography-scale tier in the
 * task-001 token sources (both legitimately use the name "text-body" for their own concern) —
 * flagged to the manager in the task-002 report, not fixed by editing the token sources here.
 * Keeping the color/radius tier and the typography tier in separate Style Dictionary instances
 * (only combined afterwards, as plain JS arrays, once each side's references are already
 * resolved) avoids the collision entirely without touching the committed token sources.
 *
 * --- Why register on -default, not on the bare `--mf-x` var ---
 * `@property` gives a custom property a real initial value and inheritance behavior. If nothing
 * ever explicitly sets a *registered* custom property, every element resolves it to that fixed
 * initial-value (inherited down from the root) — it is no longer the CSS "guaranteed-invalid"
 * value, so `var(--mf-x, var(--mf-x-default))`'s fallback would never fire. Since this build
 * intentionally never assigns the bare `--mf-x` (only style packages are meant to, later, per
 * the fallback-chaining contract), registering `@property` on `--mf-x` itself would make every
 * `--mf-x` permanently resolve to one fixed light-or-dark initial value regardless of the
 * `.dark` scope, breaking dark-mode rendering. `--mf-x-default` does not have this problem: this
 * build always explicitly assigns it in both `:root` and `.dark`, so its registered
 * `initial-value` is never actually relied upon in normal operation.
 */

import { fileURLToPath } from 'node:url';
import path from 'node:path';
import fs from 'node:fs/promises';
import StyleDictionary from 'style-dictionary';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const GUI_ROOT = path.resolve(__dirname, '..');
const OUT_DIR = path.join(GUI_ROOT, 'tokens', 'dist');
const OUT_FILE = path.join(OUT_DIR, 'tokens.css');

/** DTCG `$type` -> CSS `@property` `syntax` component. Types with no entry here (fontFamily —
 * CSS has no `@property`-syntax component for an arbitrary comma-separated family-name list)
 * still get a baked `-default` custom property, just no typed `@property` registration. */
const SYNTAX_BY_TYPE = {
  color: '<color>',
  dimension: '<length>',
  number: '<number>',
  fontWeight: '<number>',
};

/** Tailwind v4 `@theme` color keys the pre-migration `.ladle/styles.css` already mapped, now
 * repointed at the fallback-chained semantic contract instead of the old bare shadcn vars. */
const COLOR_THEME_MAP = [
  ['mf-background', '--color-background'],
  ['mf-text-body', '--color-foreground'],
  ['mf-surface', '--color-card'],
  ['mf-surface-foreground', '--color-card-foreground'],
  ['mf-popover', '--color-popover'],
  ['mf-popover-foreground', '--color-popover-foreground'],
  ['mf-primary', '--color-primary'],
  ['mf-primary-foreground', '--color-primary-foreground'],
  ['mf-secondary', '--color-secondary'],
  ['mf-secondary-foreground', '--color-secondary-foreground'],
  ['mf-surface-variant', '--color-muted'],
  ['mf-surface-variant-foreground', '--color-muted-foreground'],
  ['mf-accent', '--color-accent'],
  ['mf-accent-foreground', '--color-accent-foreground'],
  ['mf-error', '--color-destructive'],
  ['mf-border', '--color-border'],
  ['mf-input', '--color-input'],
  ['mf-ring', '--color-ring'],
  // Added by task-002 (primitive audit): ProfileEditor's success banner used raw
  // `green-*` Tailwind palette classes because `--mf-success`/`--mf-success-foreground`
  // were defined (tokens/semantic/color.{light,dark}.json) but never wired to a Tailwind
  // color utility. Wiring them here — mirroring every other semantic color role — lets the
  // banner consume `bg-success`/`text-success`/`border-success` through the same
  // fallback-chained `@theme` contract instead of a hardcoded, non-token color.
  ['mf-success', '--color-success'],
  ['mf-success-foreground', '--color-success-foreground'],
];

const FONT_THEME_MAP = [
  ['mf-font-sans', '--font-sans'],
  ['mf-font-mono', '--font-mono'],
];

/** Derived radius steps. Emitted via `calc(var(--mf-radius, var(--mf-radius-default)) * k)`
 * (per tokens/semantic/radius.json's guidance) rather than through each step's own
 * fallback-chained var, so a runtime override of the single base `--mf-radius` cascades to every
 * derived Tailwind radius utility — matching the pre-migration `calc(var(--radius) * k)` shape. */
const RADIUS_THEME_MAP = [
  ['mf-radius-sm', '--radius-sm'],
  ['mf-radius-md', '--radius-md'],
  ['mf-radius-lg', '--radius-lg'],
  ['mf-radius-xl', '--radius-xl'],
];

/** Tailwind v4's `--container-*` namespace backs the `max-w-*` utilities, so this single key
 * yields a `max-w-content` utility — a cheap adoption affordance for a consumer that wants the
 * width token without adopting the whole `@utility container` block below. */
const LAYOUT_THEME_MAP = [
  ['mf-max-content-width', '--container-content'],
];

/** Ascending Tailwind breakpoint band order. This list is the ORDERING authority only — *which*
 * bands are emitted, and every multiplier, are read from the token sources'
 * `com.moduleforge.breakpoint` extensions, never restated here. Order is load-bearing and cannot
 * be derived from the sources: every band is min-width, so at any viewport the LAST matching
 * declaration wins, and emitting in source order or by the `byName` comparator (which sorts
 * `2xl` first) would silently invert the ladder. `base` is the unprefixed, below-`sm` band; its
 * declarations are emitted unwrapped, outside any `@variant`. A band the sources name but this
 * list does not throws, rather than landing in an arbitrary position. */
const BAND_ORDER = ['base', 'sm', 'md', 'lg', 'xl', '2xl'];

/** The content-margin ladder's two axes: the `--mf-content-margins-<axis>` token-name segment and
 * the CSS property the `@utility container` block declares it on. `padding-*` rather than
 * `margin-*` because Tailwind's preflight sets `box-sizing: border-box`, which puts the gutter
 * inside `--mf-max-content-width` and keeps it from fighting the `margin-inline: auto` centering
 * at narrow viewports. */
const CONTAINER_AXES = [
  ['lr', 'padding-inline'],
  ['tb', 'padding-block'],
];

/** Strips the component-override tier's inert `$example` illustration group (recursively, by
 * key name) before tokens are flattened, so `component/overrides.json` can be included in the
 * source list today (contributing zero real tokens, since it is intentionally empty per its own
 * convention) without ever accidentally emitting anything from the `$example` group. */
function stripExample(tokens) {
  const clone = structuredClone(tokens);
  (function walk(node) {
    if (!node || typeof node !== 'object') return;
    if (Object.prototype.hasOwnProperty.call(node, '$example')) {
      delete node.$example;
    }
    for (const key of Object.keys(node)) walk(node[key]);
  })(clone);
  return clone;
}

StyleDictionary.registerPreprocessor({
  name: 'strip-example',
  preprocessor: (tokens) => stripExample(tokens),
});

/** @param {string[]} source */
async function resolveTokens(source) {
  const sd = new StyleDictionary({
    source,
    preprocessors: ['strip-example'],
    // The token source files each carry their own top-level "$description" (file-level
    // documentation). Style Dictionary's collision detector flags the resulting root-level
    // description overwrite as a "token collision" on every multi-file build; it is cosmetic
    // (no token $value/$type data is affected) so warnings are disabled here. Broken references
    // and transform errors are unaffected by this and still throw with full detail (verbosity
    // stays 'verbose' so a genuine broken-reference error prints the offending token, not just
    // a "re-run with --verbose" pointer).
    // Trade-off: Style Dictionary v5's `warnings` option is on/off only (no finer granularity
    // to silence just the description collision above), so disabling it here also silences
    // genuine future token-VALUE collisions between the source files in this instance's
    // `source` array. If those source arrays are ever restructured, temporarily re-enable
    // warnings (`warnings: 'warn'`) to check for real collisions before disabling them again.
    log: { verbosity: 'verbose', warnings: 'disabled', errors: { brokenReferences: 'throw' } },
    platforms: {
      css: { transformGroup: 'css' },
    },
  });
  const dictionary = await sd.getPlatformTokens('css');
  return dictionary.allTokens;
}

const isSemanticToken = (t) => Array.isArray(t.path) && t.path[0] === 'mf';
const byName = (a, b) => a.name.localeCompare(b.name);

function formatValue(t) {
  return String(t.$value);
}

function radiusMultiplierOf(tokens, name) {
  const token = tokens.find((t) => t.name === name);
  const ext = token?.$extensions?.['com.moduleforge.radius'];
  if (!ext || typeof ext.multiplier !== 'number') {
    throw new Error(`Expected a com.moduleforge.radius multiplier extension on token "${name}"`);
  }
  return ext.multiplier;
}

/** Asserts that a token the emitted CSS names outright exists in the resolved set, so a renamed
 * or dropped source token fails the build rather than emitting a `var()` chain whose baked
 * default is never declared anywhere. */
function requireToken(tokens, name) {
  if (!tokens.some((t) => t.name === name)) {
    throw new Error(`Expected a token named "--${name}" in the resolved token set`);
  }
}

/** Reads one content-margin axis's per-band multiplier ladder out of the resolved tokens'
 * `com.moduleforge.breakpoint` extensions, as a `band -> multiplier` map. Follows
 * `radiusMultiplierOf`'s precedent: a token source missing or malforming the extension throws
 * with a clear message, so the build fails loudly instead of emitting silently-wrong CSS. The
 * band list and the multipliers both come from the sources — neither is restated here.
 * @param {{name: string, $extensions?: Record<string, any>}[]} tokens
 * @param {string} axis
 * @returns {Map<string, number>} */
function contentMarginLadder(tokens, axis) {
  const prefix = `mf-content-margins-${axis}-`;
  /** @type {Map<string, number>} */
  const multiplierByBand = new Map();
  for (const token of tokens) {
    if (!token.name.startsWith(prefix)) continue;
    const ext = token.$extensions?.['com.moduleforge.breakpoint'];
    if (!ext || typeof ext.band !== 'string' || typeof ext.multiplier !== 'number') {
      throw new Error(
        `Expected a com.moduleforge.breakpoint { band, multiplier } extension on token "--${token.name}"`,
      );
    }
    if (!BAND_ORDER.includes(ext.band)) {
      throw new Error(
        `Token "--${token.name}" names breakpoint band "${ext.band}", which is outside the known ascending band order (${BAND_ORDER.join(', ')}). Add it to BAND_ORDER, in its correct ascending position, before adding it to the token sources.`,
      );
    }
    if (multiplierByBand.has(ext.band)) {
      throw new Error(`Two --mf-content-margins-${axis}-* tokens both claim breakpoint band "${ext.band}"`);
    }
    multiplierByBand.set(ext.band, ext.multiplier);
  }
  if (multiplierByBand.size === 0) {
    throw new Error(`Found no --mf-content-margins-${axis}-<band> tokens; the ${axis} content-margin ladder is empty.`);
  }
  return multiplierByBand;
}

/** Renders the compiled `@utility container` block: Tailwind's container idiom wired to the
 * layout tokens, with the per-band gutter ladder read from the token sources. Each per-band
 * declaration is `var(<per-band lever>, calc(<base lever> * <k>))` — the outer `var()` takes the
 * per-band lever and the `calc()` is only its FALLBACK, which is what lets a style package's
 * explicit per-band override win for that band alone while the base lever
 * `--mf-content-margins-<axis>` still rescales every band that carries no explicit override. */
function containerUtilityBlock(tokens) {
  requireToken(tokens, 'mf-max-content-width');
  for (const [axis] of CONTAINER_AXES) requireToken(tokens, `mf-content-margins-${axis}`);

  const ladders = new Map(CONTAINER_AXES.map(([axis]) => [axis, contentMarginLadder(tokens, axis)]));
  const bands = BAND_ORDER.filter((band) => [...ladders.values()].some((ladder) => ladder.has(band)));

  // Both axes must cover the same bands, so every emitted band carries a complete declaration set.
  for (const [axis, ladder] of ladders) {
    for (const band of bands) {
      if (!ladder.has(band)) {
        throw new Error(
          `The --mf-content-margins-${axis} ladder has no "${band}" band but another axis does; the content-margin axes must cover the same bands.`,
        );
      }
    }
  }

  const [baseBand, ...variantBands] = bands;
  if (baseBand !== BAND_ORDER[0]) {
    throw new Error(
      `The content-margin ladder is missing its "${BAND_ORDER[0]}" band. That band's declarations are emitted unwrapped, outside any @variant, so it cannot be omitted.`,
    );
  }

  const declarations = (band, indent) =>
    CONTAINER_AXES.map(([axis, property]) => {
      const baseLever = `var(--mf-content-margins-${axis}, var(--mf-content-margins-${axis}-default))`;
      const derived = `calc(${baseLever} * ${ladders.get(axis).get(band)})`;
      return `${indent}${property}: var(--mf-content-margins-${axis}-${band}, ${derived});`;
    }).join('\n');

  return [
    '@utility container {',
    '  margin-inline: auto;',
    '  max-width: var(--mf-max-content-width, var(--mf-max-content-width-default));',
    declarations(baseBand, '  '),
    ...variantBands.map((band) => `  @variant ${band} {\n${declarations(band, '    ')}\n  }`),
    '}',
  ].join('\n');
}

async function main() {
  // Mode-independent tokens (radius, spacing/layout) plus the LIGHT color set. Light is the
  // baseline mode, so this one instance backs both `:root` and the light-scoped re-assertion;
  // the mode-independent members are filtered back out before the scoped selectors are rendered.
  const lightAndModeIndependentTokens = (
    await resolveTokens([
      'tokens/base/color.json',
      'tokens/base/radius.json',
      'tokens/base/spacing.json',
      'tokens/semantic/color.light.json',
      'tokens/semantic/radius.json',
      'tokens/semantic/layout.json',
      'tokens/component/overrides.json',
    ])
  )
    .filter(isSemanticToken)
    .sort(byName);

  const colorDarkTokens = (
    await resolveTokens([
      'tokens/base/color.json',
      'tokens/semantic/color.dark.json',
      'tokens/component/overrides.json',
    ])
  )
    .filter(isSemanticToken)
    .filter((t) => t.$type === 'color')
    .sort(byName);

  const typographyTokens = (
    await resolveTokens([
      'tokens/base/font.json',
      'tokens/typography/families.json',
      'tokens/typography/scale.json',
    ])
  )
    .filter(isSemanticToken)
    .sort(byName);

  const lightTokens = [...lightAndModeIndependentTokens, ...typographyTokens].sort(byName);

  // Sanity check: every dark color role must also exist as a light color role (same `--mf-*`
  // contract, mode-appropriate value) — guards against a future token-source edit silently
  // dropping or renaming a role in one mode file but not the other.
  const lightColorNames = new Set(
    lightAndModeIndependentTokens.filter((t) => t.$type === 'color').map((t) => t.name),
  );
  for (const t of colorDarkTokens) {
    if (!lightColorNames.has(t.name)) {
      throw new Error(
        `Dark-mode color token "--${t.name}" has no matching light-mode token; the light and dark semantic color sources have diverged.`,
      );
    }
  }

  const propertyBlocks = lightTokens
    .filter((t) => SYNTAX_BY_TYPE[t.$type])
    .map(
      (t) =>
        `@property --${t.name}-default {\n  syntax: "${SYNTAX_BY_TYPE[t.$type]}";\n  inherits: true;\n  initial-value: ${formatValue(t)};\n}`,
    )
    .join('\n\n');

  const themeLines = [
    ...COLOR_THEME_MAP.map(([mfName, twVar]) => `  ${twVar}: var(--${mfName}, var(--${mfName}-default));`),
    ...FONT_THEME_MAP.map(([mfName, twVar]) => `  ${twVar}: var(--${mfName}, var(--${mfName}-default));`),
    ...RADIUS_THEME_MAP.map(([mfName, twVar]) => {
      const multiplier = radiusMultiplierOf(lightTokens, mfName);
      return `  ${twVar}: calc(var(--mf-radius, var(--mf-radius-default)) * ${multiplier});`;
    }),
    ...LAYOUT_THEME_MAP.map(([mfName, twVar]) => `  ${twVar}: var(--${mfName}, var(--${mfName}-default));`),
  ];

  // Light COLOR-only defaults: used for the scoped light re-assertion and for the
  // inverse-inside-dark flip. Mode-independent tokens (typography, radius, spacing/layout, font
  // families) are NOT re-emitted in scoped selectors — they live once in :root and inherit down
  // unchanged; this `$type === 'color'` filter is what keeps them out.
  const lightColorTokens = lightAndModeIndependentTokens.filter((t) => t.$type === 'color').sort(byName);

  const declList = (tokens) => tokens.map((t) => `  --${t.name}-default: ${formatValue(t)};`).join('\n');

  const containerBlock = containerUtilityBlock(lightTokens);

  const rootBlock = declList(lightTokens);
  const lightColorBlock = declList(lightColorTokens);
  const darkColorBlock = declList(colorDarkTokens);

  const output = `/* AUTO-GENERATED by mod-core/gui/style-dictionary/build-tokens.mjs — do not edit by hand.
 * Regenerate with \`npm run build:tokens\` (or \`bun run build:tokens\`) after editing any file
 * under mod-core/gui/tokens/. Compiled from the DTCG token sources via Style Dictionary.
 *
 * The mode/scope blocks below are keyed on the unified \`data-mf-theme\` scoping attribute (the
 * one mechanism serving light/dark, inverse sections, and runtime brand selection). See
 * mod-core/gui/tokens/CONTRACT.md for the token-consumption + scoping contract. */

${propertyBlocks}

@theme inline {
${themeLines.join('\n')}
}

/* Tailwind's container idiom, wired to the layout tokens. \`@variant <band>\` rather than a
   literal \`@media\` so the breakpoint VALUES resolve against whatever \`--breakpoint-*\` theme the
   CONSUMING build has — only the band names are baked in here. Bands are min-width and emitted in
   ascending order, so at any viewport the last matching declaration wins; a per-band
   \`--mf-content-margins-<axis>-<band>\` override therefore governs its band's span, from that
   breakpoint up to the next declared one, rather than every width above it.

   Two deliberate behavior changes for anyone already using \`.container\`:
     - \`@utility container\` EXTENDS Tailwind's built-in \`.container\`; it does not replace it.
       Tailwind emits the built-in rule first and this one second, and media queries add no
       specificity, so the unconditional \`max-width\` below overrides the built-in's entire
       per-breakpoint max-width ladder (40rem through 96rem). That is intended —
       \`--mf-max-content-width\` takes ownership of the container's width — but it is a behavior
       change for a consumer already relying on that ladder.
     - \`.container\` gains \`padding-block\`, which Tailwind's built-in never had. Also intended: it
       is what \`--mf-content-margins-tb\` is for. Ordinary utilities (\`py-0\`, \`py-8\`) are emitted
       after the container rules and still win, so the escape hatch is intact. */
${containerBlock}

/* Light is the baseline: the full default set (colors + mode-independent typography/radius),
   active whenever data-mf-theme is absent. */
:root {
${rootBlock}
}

/* Explicit light scope: re-asserts the light COLOR defaults so a light island nested inside a
   dark region flips back to light. Typography/radius are inherited unchanged. */
[data-mf-theme="light"] {
${lightColorBlock}
}

/* Dark scope. The legacy .dark class is bridged alongside the unified attribute for back-compat
   (Ladle's theme addon and any existing consumer still toggling .dark keep working). */
[data-mf-theme="dark"],
.dark {
${darkColorBlock}
}

/* Inverse section — a relative flip to the opposite surface of its surrounding mode.
   In a light context (the default), an inverse subtree resolves to the DARK color set. */
[data-mf-theme="inverse"] {
${darkColorBlock}
}

/* Inverse nested under a dark scope resolves to the LIGHT color set. The two-attribute compound
   selector outranks the single-attribute inverse rule above by specificity, so the flip is
   relative to the surrounding mode. */
[data-mf-theme="dark"] [data-mf-theme="inverse"],
.dark [data-mf-theme="inverse"] {
${lightColorBlock}
}
`;

  await fs.mkdir(OUT_DIR, { recursive: true });
  await fs.writeFile(OUT_FILE, output);
  console.log(`Wrote ${path.relative(GUI_ROOT, OUT_FILE)} (${lightTokens.length} light tokens, ${colorDarkTokens.length} dark color overrides).`);
}

await main();
