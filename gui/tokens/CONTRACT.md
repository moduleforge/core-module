# ModuleForge token contract

The stable interface between three parties:

- **Component authors** (mod-core primitives and any GUI package consuming `@moduleforge/core-gui`)
  — who *consume* tokens.
- **Style-package authors** (the built-in default, the Liquid Labs brand package, and future
  packages) — who *supply* token values.
- **The token compiler** (`../style-dictionary/build-tokens.mjs`) — which emits the baked defaults
  and the scoping variable sets both sides rely on.

Two mechanisms make the token system pluggable and robust: a **fallback-chaining consumption
contract** and **one unified `data-mf-theme` scoping attribute**. This document is the fixed
contract for both. It complements [README.md](./README.md) (which documents the DTCG token
*sources*); this doc governs how the *compiled* tokens are consumed and scoped.

## Fallback-chaining consumption contract

**Rule:** every consumed token resolves as

```css
var(--mf-x, var(--mf-x-default))
```

`--mf-x` is the *current* value a style package may supply; `--mf-x-default` is the mod-core baked
default. If a style package omits `--mf-x`, rendering degrades gracefully to `--mf-x-default`
rather than to the CSS guaranteed-invalid value. A style package is therefore **always partial /
sparse** and can never leave a token undefined.

### Settable vs. internal

| Property | Who sets it | Notes |
|----------|-------------|-------|
| `--mf-x` | **Style packages only** | The single runtime-settable lever. Never set by mod-core; that is what makes the fallback fire. |
| `--mf-x-default` | **The compiler only** | Baked into the compiled bundle in `:root` (light) and the scope selectors. **Not** style-package-settable — treat it as internal. Registered as a typed CSS `@property` (see [Security posture](#security-posture)). |

A style package that sets `--mf-x-default` is **outside the contract** and its behavior is
undefined (it fights the compiled bundle and breaks mode/scope switching). Supply `--mf-x` only.

**Divergence for spacing/container-width per-band levers.** The general rule that a compiler-derived
form is style-package-internal (see [Radius](#radius), below) holds for radius but not for the new
`--mf-content-margins-{lr,tb}-<band>` per-band tokens: those derived-shape names **are**
style-package-settable, by design — see
[Spacing and container width](#spacing-and-container-width). Only their `-default` twins
(`--mf-content-margins-{lr,tb}-<band>-default`) are compiler-internal, as usual.

### Where the contract is enforced

**Primary lever — the Tailwind `@theme inline` layer.** The compiled bundle
(`./dist/tokens.css`) maps every Tailwind color/font/radius theme key through the fallback chain,
e.g.:

```css
@theme inline {
  --color-primary: var(--mf-primary, var(--mf-primary-default));
  --color-background: var(--mf-background, var(--mf-background-default));
  --font-sans: var(--mf-font-sans, var(--mf-font-sans-default));
  --radius-md: calc(var(--mf-radius, var(--mf-radius-default)) * 0.8);
}
```

Because of this, existing `class-variance-authority` (cva) utility classes — `bg-primary`,
`text-secondary-foreground`, `border-border`, `outline-ring/50`, `rounded-md`, `font-sans`, … —
inherit the contract automatically, with no per-component churn. This is the mechanism task `002`
relies on when auditing the primitives.

**Residual direct `var(--…)` references.** A few sites reference a custom property directly
instead of flowing through a Tailwind color utility. These must also chain to a `-default`:

- **The `--radius-*` calc chain.** The Tailwind radius theme keys are emitted as
  `calc(var(--mf-radius, var(--mf-radius-default)) * k)`, so a single runtime `--mf-radius`
  override cascades to every derived step. A component that references `var(--radius-md)` directly
  (e.g. `rounded-[min(var(--radius-md),10px)]` in `button.tsx`) therefore already resolves through
  the `--mf-radius` chain — no per-site change needed, but do **not** reference `--mf-radius-md`
  (or its `-default`) directly; go through the Tailwind `--radius-md` theme key.
- **`outline-ring/50`, `ring-ring/50`, etc.** These are Tailwind utilities backed by
  `--color-ring: var(--mf-ring, var(--mf-ring-default))`, so they chain through the `@theme` layer
  like any other color utility.
- **Any raw `var(--mf-x)` a component author writes by hand** must be written as
  `var(--mf-x, var(--mf-x-default))` — never a bare `var(--mf-x)`, which would resolve to the
  guaranteed-invalid value whenever no style package is loaded.

**Rule of thumb for component authors:** prefer a Tailwind utility (it carries the contract for
free). If you must reference a custom property directly, always spell the full
`var(--mf-x, var(--mf-x-default))` chain.

### Tailwind container integration

The compiler emits an `@utility container` block into `./dist/tokens.css`, wiring the spacing
tokens into Tailwind's established container idiom rather than building parallel machinery:

- **`@variant <band>`, not a literal `@media`.** Each band inside `@utility container` is spelled
  `@variant sm { … }`, `@variant md { … }`, and so on, so the breakpoint *values* resolve against
  the consuming build's own `--breakpoint-*` theme and only the band *names* are baked into
  mod-core. A downstream app that customizes `--breakpoint-lg` gets a container whose gutter step
  moves with it.
- **`--container-content` / `max-w-content`.** The `@theme inline` key
  `--container-content: var(--mf-max-content-width, var(--mf-max-content-width-default))` yields a
  `max-w-content` utility — a cheap adoption path for a consumer that wants the width token without
  the whole `container` utility.
- **`@utility container` extends Tailwind's built-in container; it does not replace it.** Both
  `.container` rules are emitted into `@layer utilities` — the built-in rule first, mod-core's
  custom rule second. Media queries add no specificity, so the custom rule's unconditional
  `max-width` **overrides Tailwind's entire built-in per-breakpoint max-width ladder**
  (`40rem` → `96rem`). This is intended — the token takes ownership of the container's width — but
  it is a **behavior change for any consumer already using `.container`**: that consumer stops
  stepping through five breakpoint widths and gets a single `80rem` cap (mod-core's default)
  instead.
- **`.container` now carries `padding-block`, which Tailwind's built-in container never had.**
  Intended; it is what `--mf-content-margins-tb` is for. The escape hatch: ordinary utilities
  (`py-0`, `py-8`, `max-w-md`, …) are emitted after the container rules in source order and still
  win, so a consumer can opt out of any container declaration with a plain utility.

## The fixed `--mf-*` surface

The token names below are the **stable contract surface**. Component authors consume them (mostly
via Tailwind utilities); style-package authors may supply the `--mf-x` form of any of them. Each
has a compiler-emitted `--mf-x-default` twin (internal — see above).

### Color roles (35)

Wired into the Tailwind `@theme` color map (consumed through cva utility classes):

`--mf-background`, `--mf-text-body` (→ `foreground`), `--mf-surface` (→ `card`),
`--mf-surface-foreground`, `--mf-popover`, `--mf-popover-foreground`, `--mf-primary`,
`--mf-primary-foreground`, `--mf-secondary`, `--mf-secondary-foreground`, `--mf-surface-variant`
(→ `muted`), `--mf-surface-variant-foreground`, `--mf-accent`, `--mf-accent-foreground`,
`--mf-error` (→ `destructive`), `--mf-border`, `--mf-input`, `--mf-ring`, `--mf-success`
(→ `success`), `--mf-success-foreground` (→ `success-foreground`).

Defined and settable, consumed by direct reference or reserved for later primitives (not currently
in the Tailwind color map):

`--mf-primary-hover`, `--mf-brand-highlight`, `--mf-tertiary`, `--mf-tertiary-foreground`,
`--mf-text-heading`, `--mf-text-muted`, `--mf-link`, `--mf-link-visited`, `--mf-link-hover`,
`--mf-link-active`, `--mf-error-foreground`, `--mf-warning`, `--mf-warning-foreground`,
`--mf-info`, `--mf-info-foreground`.

### Radius

`--mf-radius` is the **single settable radius lever**. The derived Tailwind steps `--radius-sm`,
`--radius-md`, `--radius-lg`, `--radius-xl` are emitted as `calc(var(--mf-radius, …) * k)`, so
overriding `--mf-radius` rescales all of them. (The compiled `--mf-radius-{sm,md,lg,xl}-default`
values exist as typed `@property` reference values; they are **not** part of the runtime-settable
chain — override `--mf-radius`, not the derived steps.)

#### Per-component radius override tier

`--mf-radius` rescales **every** primitive together — there is no way, through the global lever
alone, for a brand to want pill-shaped buttons but square-cornered cards. The **component-override
tier** (`tokens/component/overrides.json`, `mf.component.<component>.<property>` — see
[README.md](./README.md#tiering-convention)) is the escape hatch for exactly this case, via a
reserved `radius` property: `mf.component.<component>.radius`.

This is the per-**component** counterpart to the per-**breakpoint-band**
[`--mf-content-margins-*` lever family](#spacing-and-container-width) above — same mechanism
(an outer, fallback-chained lever standing in for the global one, inside the same `calc()` shape),
with component as the second override axis instead of breakpoint band:

- **Valid keys.** `mf.component.<component>.radius` in `tokens/component/overrides.json`, `$type:
  "dimension"`, e.g. `mf.component.button.radius`. `<component>` is a free-form kebab-case
  component name; `radius` is the only property name this tier gives special build-time handling
  to (every other `mf.component.<component>.<property>` entry is a plain alias, per the file's
  existing convention).
- **Compiled output.** For every `mf.component.<component>.radius` entry, the compiler
  (`../style-dictionary/build-tokens.mjs`, `componentRadiusOverrideBlock`) emits a
  `[data-mf-component="<component>"]` scoped block that shadows `--radius-sm`, `--radius-md`,
  `--radius-lg`, and `--radius-xl` for any element nested under that attribute:

  ```css
  [data-mf-component="button"] {
    --radius-sm: calc(var(--mf-component-button-radius, var(--mf-component-button-radius-default)) * 0.6);
    --radius-md: calc(var(--mf-component-button-radius, var(--mf-component-button-radius-default)) * 0.8);
    --radius-lg: calc(var(--mf-component-button-radius, var(--mf-component-button-radius-default)) * 1);
    --radius-xl: calc(var(--mf-component-button-radius, var(--mf-component-button-radius-default)) * 1.4);
  }
  ```

  Each step keeps its usual global multiplier (`0.6` / `0.8` / `1.0` / `1.4`), scaled from the
  component's own lever instead of `--mf-radius`, so an overridden component still gets
  proportionate sm/md/lg/xl steps within its own scope — exactly like `--mf-radius` does globally.
  A component author opts in by rendering the primitive's root element with
  `data-mf-component="button"`; a Tailwind `rounded-md` (etc.) utility nested under that attribute
  resolves the shadowed `--radius-md` custom property via ordinary CSS cascade, no per-utility
  change needed. **Wiring the `data-mf-component` attribute onto mod-core's own primitives
  (`button.tsx`, `card.tsx`, …) is a separate, not-yet-done step** — this tier establishes the
  token-compiler mechanism only.
- **Interaction with `--mf-radius` / `--radius-*`.** `--mf-component-<component>-radius` resolves
  through the same fallback chain as every other `--mf-x` lever: a style package may set it
  directly, falling back to its compiler-baked `-default` twin (from `overrides.json`'s `$value`)
  when unset. It takes precedence over the global `--mf-radius` derivation **only inside its own
  `[data-mf-component="…"]` scope**; elements outside that scope, and any component with no
  `mf.component.<component>.radius` entry, are unaffected and continue deriving from the global
  `--mf-radius` lever exactly as before. `--mf-component-<component>-radius-default` is
  compiler-only, exactly like every other `-default` twin in this contract — not part of the
  runtime-settable chain.
- **Default (no-override) behavior is unchanged.** When `overrides.json` defines no
  `mf.component.<component>.radius` entries — the shipped default — the compiler emits no
  `[data-mf-component="…"]` blocks at all; the output is byte-identical to a build without this
  tier.
- **Example.** A brand wanting pill-shaped buttons while leaving cards square adds, in
  `tokens/component/overrides.json`:

  ```json
  {
    "mf": {
      "component": {
        "button": {
          "radius": { "$type": "dimension", "$value": "9999px" }
        }
      }
    }
  }
  ```

  and (once `data-mf-component="button"` is wired onto the `Button` primitive) every `rounded-*`
  utility on `Button` resolves to a full pill, while `Card` — which carries no
  `mf.component.card.radius` entry — keeps deriving from the global `--mf-radius` scale unchanged.

### Typography

- **Families:** `--mf-font-sans`, `--mf-font-mono` (both wired into the `@theme` font map),
  `--mf-font-heading` (defined; not yet mapped to a Tailwind key).
- **Type scale sub-tokens** follow the pattern `--mf-text-<level>-<axis>`, where
  `<level>` ∈ {`h1`…`h6`, `body`, `body-sm`, `label`} and `<axis>` ∈
  {`size`, `line-height`, `weight`, `tracking`}. (Note: `--mf-text-body` also names a *color* role
  above; the typography sub-tokens are the `--mf-text-body-size` / `-line-height` / `-weight` /
  `-tracking` forms — a known source-tier naming overlap between the color and typography tiers,
  flagged in the Phase 1 task-002 build notes.)

### Spacing and container width

Four scalar roles plus twelve per-band levers (six bands × two axes), all **mode-independent** —
emitted once in `:root` and never re-emitted in the `data-mf-theme` scope selectors (see
[Emission shape](#emission-shape) below).

- **`--mf-max-content-width`** — the max width page content grows to. A single scalar; the ordinary
  `var(--mf-max-content-width, var(--mf-max-content-width-default))` chain, no second axis.
- **`--mf-max-content-width-narrow`** — a second, narrower content-width scalar (followup XKY2),
  for reading-oriented pages nested inside a wider shell. Independent of `--mf-max-content-width`
  — the two roles do not interact or derive from one another — and is the ordinary
  `var(--mf-max-content-width-narrow, var(--mf-max-content-width-narrow-default))` chain, no second
  axis. See [Opting into the narrow measure](#opting-into-the-narrow-measure) below for how a page
  consumes it.
- **`--mf-content-margins-lr`** / **`--mf-content-margins-tb`** — the base inputs for the inline-
  and block-axis gutter ladders. Setting either rescales **every** band, exactly as `--mf-radius`
  rescales every derived radius step.
- **`--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}`** — the twelve per-band levers. Each
  overrides its own band only, short-circuiting the derived calculation for that band:
  `--mf-content-margins-lr-base`, `--mf-content-margins-lr-sm`, `--mf-content-margins-lr-md`,
  `--mf-content-margins-lr-lg`, `--mf-content-margins-lr-xl`, `--mf-content-margins-lr-2xl`
  (inline axis), and `--mf-content-margins-tb-base`, `--mf-content-margins-tb-sm`,
  `--mf-content-margins-tb-md`, `--mf-content-margins-tb-lg`, `--mf-content-margins-tb-xl`,
  `--mf-content-margins-tb-2xl` (block axis).

#### Resolution expression

Every per-band value resolves as:

```css
var(--mf-content-margins-lr-<band>,
    calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * <k_band>))
```

(the `tb` axis is the identical shape with `tb` substituted for `lr`). Read outward-in, the
precedence is:

1. `--mf-content-margins-lr-<band>` — the style package's explicit override *for this band*. Wins
   outright; the `calc()` is never evaluated.
2. `--mf-content-margins-lr` — the style package's base-scale override. Rescales **every** band
   through that band's own multiplier.
3. `--mf-content-margins-lr-default` — mod-core's baked default, used when the package sets
   neither.

This is the existing [Radius](#radius) idiom
(`--radius-md: calc(var(--mf-radius, var(--mf-radius-default)) * 0.8)`) with one outer `var()`
added, so the per-band lever can short-circuit the `calc()` for that band alone — the two are one
family.

#### Band-span semantics

Every band's declaration lives in one rule (the `@utility container` block — see
[Tailwind container integration](#tailwind-container-integration)), and every band is `min-width`
(`@variant <band>`), so at any viewport the **last matching** declaration wins. A per-band override
therefore governs its band's **span** — from that breakpoint up to the next declared band — not
every width above it. For example, setting `--mf-content-margins-lr-sm` alone changes the gutter
between `40rem` and `48rem` (the `sm` → `md` span) and nowhere else; widths at `48rem` and above
pick up the `md` band's own value (or its calc'd default), unaffected by the `sm` override.

#### Settable vs. internal — a divergence from radius

`--mf-content-margins-{lr,tb}` and `--mf-content-margins-{lr,tb}-<band>` are **all**
style-package-settable — including the per-band forms. `--mf-content-margins-{lr,tb}-<band>-default`
are compiler-only, exactly like `--mf-radius-{sm,md,lg,xl}-default`: they exist as typed `@property`
reference values and are **not** part of the runtime resolution chain.

This is the opposite of the radius rule: for radius, a derived step's bare `--mf-x` form is
deliberately **not** settable ("override `--mf-radius`, not the derived steps"). For content
margins, the derived step's bare form **is** settable — that per-band lever is the whole point of
this shape. A reader who has internalized the radius rule needs to be told this one works
differently.

#### Fluid values are expressible

Every band's value is typed `<length>`, and `clamp()` with a `vw` term computes to a length, so a
style package that wants continuous scaling inside a band can set, e.g.,
`--mf-content-margins-lr-lg: clamp(1rem, 4vw, 3rem)`. This is the answer to "can I do fluid
spacing?" — yes, inside any single band.

#### Opting into the narrow measure

`--mf-max-content-width` is a single global scalar, so an application whose shell is wide (e.g.
`80rem`) but whose reading-oriented pages want a narrower measure could not previously express both
through the token — this was formerly documented here as an accepted, open limitation. Followup
XKY2 closes it with a second, narrower content-width role: **`--mf-max-content-width-narrow`**,
defaulting to `42rem` (`spacing.max-content-width-narrow` in `tokens/base/spacing.json`, aliased
into `mf.max-content-width-narrow` in `tokens/semantic/layout.json`). It is purely additive: adding
it does not change `--mf-max-content-width`'s own default or behavior, and a build with no consumer
opting into the narrow measure is otherwise unaffected.

`--mf-max-content-width-narrow` is emitted the same way as every other scalar semantic token — a
compiler-baked `--mf-max-content-width-narrow-default: 42rem` in `:root` plus a typed `@property`
registration — but, like `--mf-max-content-width` before it, it does **not** feed a dedicated
Tailwind utility or its own `@utility container` variant; the existing `container` utility's
`max-width` still reads only `--mf-max-content-width`. A page opts into the narrow measure the same
way `ProfileEditor.tsx` opts into a page-specific width today (followup AkGw): adopt the `container`
utility, then pin `--mf-max-content-width` locally, for that scope only, to the narrow role's
fallback-chained value:

```tsx
<div
  className="container"
  style={{
    ['--mf-max-content-width' as string]:
      'var(--mf-max-content-width-narrow, var(--mf-max-content-width-narrow-default))',
  } as CSSProperties}
>
```

so the page's `container` resolves to the narrow measure (`42rem` by default, or a style package's
`--mf-max-content-width-narrow` override) while every other consumer of `--mf-max-content-width` —
including sibling pages that don't re-scope it — keeps resolving to the wide, shell-level default
unchanged.

## Unified scoping — `data-mf-theme`

**One** `data-*` attribute reassigns the `--mf-*` custom properties for its subtree, at any DOM
depth (following the Bootstrap 5.3 `data-bs-theme` / GitHub Primer `data-color-mode` precedent). It
serves **all three** scoping needs — do **not** introduce a second scoping mechanism.

| `data-mf-theme` value | Effect |
|-----------------------|--------|
| *(absent)* | Light — the `:root` baseline default set. |
| `light` | Re-asserts the light color defaults for a subtree (a light island nested inside a dark region). |
| `dark` | The dark color defaults. |
| `inverse` | A **relative** flip to the opposite surface of the surrounding mode: dark in a light context, light in a dark context (via a higher-specificity compound selector). |

### How the three uses map onto one attribute

1. **Light vs. dark.** Set `data-mf-theme="light"` / `"dark"` on the app shell (or any subtree).
2. **Inverse sections.** Set `data-mf-theme="inverse"` on a subtree that should flip to the
   opposite/emphasis surface (e.g. a dark call-to-action band on a light page). The flip is
   relative to the surrounding mode.
3. **Runtime brand selection.** A brand style package (loaded at runtime as a versioned
   `<link rel="stylesheet">`) is a sparse `--mf-x` override bundle. It scopes its overrides on the
   *same* attribute — e.g. it emits
   its dark-mode brand overrides under `[data-mf-theme="dark"]` — so brand and mode compose through
   the one attribute rather than needing a parallel `data-mf-brand`. Which brand is active is an
   app-shell concern (which bundle is loaded), not a second attribute.

### Emission shape

The compiler emits (`./dist/tokens.css`):

- `:root` — the full **light** default set (colors + mode-independent typography/radius).
- `[data-mf-theme="light"]` — light **color** defaults only (mode-independent tokens inherit
  unchanged).
- `[data-mf-theme="dark"], .dark` — dark color defaults.
- `[data-mf-theme="inverse"]` — dark color defaults (inverse in a light context).
- `[data-mf-theme="dark"] [data-mf-theme="inverse"], .dark [data-mf-theme="inverse"]` — light color
  defaults (inverse in a dark context; wins on specificity).

Only color roles are re-emitted in scoped selectors; radius/typography/font families and the
spacing/container-width roles (`--mf-max-content-width`, `--mf-content-margins-*`, see
[Spacing and container width](#spacing-and-container-width)) are mode-independent and live once in
`:root`.

### Back-compat bridge and reconciliation

The legacy `.dark` class is **bridged** alongside `[data-mf-theme="dark"]` in two places, so any
consumer still toggling `.dark` (or `dark:` Tailwind utilities) keeps working:

- The dark token block lists both `[data-mf-theme="dark"]` and `.dark`.
- `.ladle/styles.css` extends the Tailwind dark variant:
  `@custom-variant dark (&:is(.dark *, [data-mf-theme="dark"] *))`.

The Ladle wrapper (`.ladle/components.tsx`) was migrated onto the attribute — its theme-addon
toggle now sets `data-mf-theme="light|dark"`.

**Known limitation (accepted, still open).** `data-mf-theme="inverse"` reassigns the token *custom
properties* for its subtree but does not, by itself, re-trigger `dark:` *Tailwind utility variants*.
A handful of primitives lean on `dark:` utilities for subtle opacity/emphasis tweaks (e.g.
`dark:border-input dark:bg-input/30` on `button`, `dark:border-destructive` on `alert`/`toast`); on
those primitives the token-level color flip under `data-mf-theme="inverse"` is correct, but the
`dark:`-gated modifier does not retrigger, since `inverse` is not the `dark` class/media state the
`dark:` variant checks for. The task `002` primitive audit surveyed this gap and deliberately
deferred fixing it — closing it would mean changing shared component code to key those modifiers
off `data-mf-theme` instead of (or in addition to) `dark:`, which was judged a regression risk not
worth taking in that pass. No component code has been changed to close this gap; it remains an
open, accepted limitation of the current inverse-scoping mechanism, documented here for anyone
extending it later. Deeply nested inverse-within-inverse is also out of scope — inverse is designed
for a single flip relative to the page mode.

## Security posture

Every color / dimension / number / fontWeight token is registered as a typed CSS `@property`
(on the `-default` twin — see the compiler's inline note for why not on the bare `--mf-x`). This
keeps the door open to less-trusted / user-authored style packages later: a style package should
only ever supply a **typed values manifest** against this fixed contract — a set of `--mf-x`
declarations — never an arbitrary stylesheet. Sandboxing untrusted themes and an end-user theme
picker are explicitly out of scope now but are not architecturally foreclosed.

## Related documents

- [README.md](./README.md) — the DTCG token *sources* and tiering convention.
- [`../style-dictionary/build-tokens.mjs`](../style-dictionary/build-tokens.mjs) — the compiler that
  emits `./dist/tokens.css` (the fallback chains, `@property` typing, and scope selectors described
  here).
- The `gui-design-tokens` plan notes `runtime-theming.md` (runtime style-package loading model,
  contract discipline, security posture) and `token-architecture.md` (the `@theme` integration
  lever and one-scoping-mechanism rationale) — planning-side grounding, held in the plan branch
  rather than shipped in this repo.
