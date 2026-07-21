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
`--mf-error` (→ `destructive`), `--mf-border`, `--mf-input`, `--mf-ring`.

Defined and settable, consumed by direct reference or reserved for later primitives (not currently
in the Tailwind color map):

`--mf-primary-hover`, `--mf-brand-highlight`, `--mf-tertiary`, `--mf-tertiary-foreground`,
`--mf-text-heading`, `--mf-text-muted`, `--mf-link`, `--mf-link-visited`, `--mf-link-hover`,
`--mf-link-active`, `--mf-error-foreground`, `--mf-warning`, `--mf-warning-foreground`,
`--mf-info`, `--mf-info-foreground`, `--mf-success`, `--mf-success-foreground`.

### Radius

`--mf-radius` is the **single settable radius lever**. The derived Tailwind steps `--radius-sm`,
`--radius-md`, `--radius-lg`, `--radius-xl` are emitted as `calc(var(--mf-radius, …) * k)`, so
overriding `--mf-radius` rescales all of them. (The compiled `--mf-radius-{sm,md,lg,xl}-default`
values exist as typed `@property` reference values; they are **not** part of the runtime-settable
chain — override `--mf-radius`, not the derived steps.)

### Typography

- **Families:** `--mf-font-sans`, `--mf-font-mono` (both wired into the `@theme` font map),
  `--mf-font-heading` (defined; not yet mapped to a Tailwind key).
- **Type scale sub-tokens** follow the pattern `--mf-text-<level>-<axis>`, where
  `<level>` ∈ {`h1`…`h6`, `body`, `body-sm`, `label`} and `<axis>` ∈
  {`size`, `line-height`, `weight`, `tracking`}. (Note: `--mf-text-body` also names a *color* role
  above; the typography sub-tokens are the `--mf-text-body-size` / `-line-height` / `-weight` /
  `-tracking` forms — a known source-tier naming overlap between the color and typography tiers,
  flagged in the Phase 1 task-002 build notes.)

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

Only color roles are re-emitted in scoped selectors; radius/typography/font families are
mode-independent and live once in `:root`.

### Back-compat bridge and reconciliation

The legacy `.dark` class is **bridged** alongside `[data-mf-theme="dark"]` in two places, so any
consumer still toggling `.dark` (or `dark:` Tailwind utilities) keeps working:

- The dark token block lists both `[data-mf-theme="dark"]` and `.dark`.
- `.ladle/styles.css` extends the Tailwind dark variant:
  `@custom-variant dark (&:is(.dark *, [data-mf-theme="dark"] *))`.

The Ladle wrapper (`.ladle/components.tsx`) was migrated onto the attribute — its theme-addon
toggle now sets `data-mf-theme="light|dark"`.

**Known limitation.** `data-mf-theme="inverse"` reassigns the token *custom properties* for its
subtree but does not, by itself, re-trigger `dark:` *Tailwind utility variants*. Components that
lean on `dark:` utilities for subtle tweaks (a handful of primitives) are reconciled in the task
`002` primitive audit; the token-level inverse flip is correct regardless. Deeply nested
inverse-within-inverse is also out of scope — inverse is designed for a single flip relative to the
page mode.

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
</content>
