#!/usr/bin/env node
/**
 * Compiles the DTCG design-token sources under `mod-core/gui/tokens/` into a single
 * generated CSS bundle at `mod-core/gui/tokens/dist/tokens.css`.
 *
 * The bundle carries:
 *   1. `@property` typed declarations for every semantic/typography/radius token whose
 *      DTCG `$type` maps to a CSS syntax component (color, dimension, number, fontWeight).
 *      Registered on the token's baked-default custom property (`--mf-x-default`), never on
 *      the bare `--mf-x` — see the "Why register on -default, not on the bare var" note below.
 *   2. A Tailwind v4 `@theme inline` block mapping Tailwind's color/font/radius theme keys to
 *      the fallback-chained semantic vars (`var(--mf-x, var(--mf-x-default))`), so existing cva
 *      utility classes (`bg-primary`, etc.) resolve through the semantic contract.
 *   3. `:root` (light) and `.dark` baked-default declarations (`--mf-x-default`) for every
 *      semantic token, sourced from `tokens/semantic/color.light.json` and
 *      `tokens/semantic/color.dark.json` respectively. Radius/typography/font-family tokens are
 *      mode-independent and are emitted in `:root` only.
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

async function main() {
  const colorLightTokens = (
    await resolveTokens([
      'tokens/base/color.json',
      'tokens/base/radius.json',
      'tokens/semantic/color.light.json',
      'tokens/semantic/radius.json',
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

  const lightTokens = [...colorLightTokens, ...typographyTokens].sort(byName);

  // Sanity check: every dark color role must also exist as a light color role (same `--mf-*`
  // contract, mode-appropriate value) — guards against a future token-source edit silently
  // dropping or renaming a role in one mode file but not the other.
  const lightColorNames = new Set(colorLightTokens.filter((t) => t.$type === 'color').map((t) => t.name));
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
  ];

  const rootLines = lightTokens.map((t) => `  --${t.name}-default: ${formatValue(t)};`);
  const darkLines = colorDarkTokens.map((t) => `  --${t.name}-default: ${formatValue(t)};`);

  const output = `/* AUTO-GENERATED by mod-core/gui/style-dictionary/build-tokens.mjs — do not edit by hand.
 * Regenerate with \`npm run build:tokens\` (or \`bun run build:tokens\`) after editing any file
 * under mod-core/gui/tokens/. Compiled from the DTCG token sources via Style Dictionary. */

${propertyBlocks}

@theme inline {
${themeLines.join('\n')}
}

:root {
${rootLines.join('\n')}
}

.dark {
${darkLines.join('\n')}
}
`;

  await fs.mkdir(OUT_DIR, { recursive: true });
  await fs.writeFile(OUT_FILE, output);
  console.log(`Wrote ${path.relative(GUI_ROOT, OUT_FILE)} (${lightTokens.length} light tokens, ${colorDarkTokens.length} dark color overrides).`);
}

await main();
