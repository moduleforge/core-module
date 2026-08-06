# ModuleForge design tokens (`@moduleforge/core-gui`)

Canonical, single-source design tokens in **W3C DTCG** format (`$value` / `$type`). This
directory is the one source of truth that replaces the four copy-pasted shadcn token sets
across the ecosystem (`mod-core`, `mod-users`, `mod-tags`, `app-mfdemo`).

This task (Plan 1 / Phase 1 / task `001`) authors **token definitions only**. The Style
Dictionary compiler, the Tailwind `@theme` wiring, and the CSS `@property` typed-custom-property
emission are **task `002`** — no CSS is emitted here and no consumer wiring is changed.

## Tiering convention

Three tiers plus a typography tier (GitHub Primer / Radix / MD3 converged pattern):

| Tier | Directory | Role |
|------|-----------|------|
| **Raw / base** | `base/` | Literal values — oklch color primitives, the base radius, font stacks, and the size/weight/line-height/tracking ramps. **Never referenced directly by components.** |
| **Semantic / purpose** | `semantic/` | The `--mf-*` roles components consume. Every value is a DTCG alias into the raw tier. Named for purpose and mapped onto MD3 color-role vocabulary. Split into `color.light.json` + `color.dark.json` mode sets; `radius.json` is mode-independent. `layout.json` (the spacing / container-width roles) is also mode-independent, joining `radius.json` in that respect. |
| **Typography** | `typography/` | Semantic font families (`families.json`) and the type scale as tokens (`scale.json`), aliasing the raw font ramps. Mode-independent. |
| **Component-override** | `component/` | Sparse escape-hatch namespace (`mf.component.<component>.<property>`). Intentionally empty at this stage — establishes the convention only. |

### File layout

```
tokens/
  base/
    color.json          neutral ramp + functional hues (red/amber/blue/green/purple) + alpha whites
    radius.json         --radius base (0.625rem)
    font.json           family stacks + size/weight/line-height/tracking ramps
    spacing.json        spacing.content-margin-base (1rem) + spacing.max-content-width (80rem)
  semantic/
    color.light.json    --mf-* color roles, LIGHT set (aliases into base)
    color.dark.json     --mf-* color roles, DARK set (same paths, dark values)
    radius.json         --mf-radius + sm/md/lg/xl derived steps
    layout.json         --mf-max-content-width + --mf-content-margins-{lr,tb} + per-band steps
  typography/
    families.json       --mf-font-sans / -mono / -heading
    scale.json          --mf-text-h1..h6 / -body / -body-sm / -label (size/line-height/weight/tracking)
  component/
    overrides.json      sparse escape-hatch convention (empty by design)
```

### Light / dark mode sets

`semantic/color.light.json` and `semantic/color.dark.json` define the **same** `--mf-*` token
paths with mode-appropriate values. They are **alternative source sets**, not files meant to be
merged together — task `002` builds the light output from base + `color.light.json` and the dark
output from base + `color.dark.json`. The raw tier and the semantic radius / typography tiers are
mode-independent and shared by both builds.

### Fallback-chaining contract (implemented in task `002`)

Each semantic token is authored so task `002` can emit **both** a "current" var (`--mf-x`) and a
baked default (`--mf-x-default`) — the default value lives here in the source. Components resolve
as `var(--mf-x, var(--mf-x-default))`, so a partial/sparse style package can override `--mf-x`
alone and any omitted token degrades gracefully to the mod-core default. Nothing to do in this
task beyond ensuring the source carries the default values (it does).

## Final semantic color enumeration (35 roles)

`--mf-*` namespace. Names map onto MD3 color roles; a `-foreground` suffix corresponds to MD3's
`on-` role.

| `--mf-*` role | MD3 role | shadcn var (crosswalk) |
|---------------|----------|------------------------|
| `primary` / `primary-foreground` | primary / on-primary | `--primary` / `--primary-foreground` |
| `primary-hover` | — | *new* (was `hover:bg-primary/90` opacity) |
| `brand-highlight` | — | *new* (placeholder = primary) |
| `secondary` / `secondary-foreground` | secondary / on-secondary | `--secondary` / `--secondary-foreground` |
| `tertiary` / `tertiary-foreground` | tertiary / on-tertiary | *new* (defaults to secondary) |
| `accent` / `accent-foreground` | secondary-container-like emphasis | `--accent` / `--accent-foreground` |
| `background` | background | `--background` |
| `surface` / `surface-foreground` | surface / on-surface | `--card` / `--card-foreground` |
| `surface-variant` / `surface-variant-foreground` | surface-variant / on-surface-variant | `--muted` / `--muted-foreground` |
| `popover` / `popover-foreground` | surface (elevated) | `--popover` / `--popover-foreground` |
| `text-body` | on-background | `--foreground` |
| `text-heading` | — | *new* (defaults to text-body) |
| `text-muted` | on-surface-variant | (== `--muted-foreground`) |
| `link` / `link-visited` / `link-hover` / `link-active` | — | *new* (no prior value) |
| `error` / `error-foreground` | error / on-error | `--destructive` / *new fg* |
| `warning` / `warning-foreground` | — | *new* |
| `info` / `info-foreground` | — | *new* |
| `success` / `success-foreground` | — | *new* |
| `border` | outline | `--border` |
| `input` | — | `--input` |
| `ring` | — | `--ring` |

**Radius:** `--mf-radius` + `--mf-radius-sm` / `-md` / `-lg` / `-xl` (preserving the existing
`calc(var(--radius) * k)` scale — see below).

**Typography families:** `--mf-font-sans`, `--mf-font-mono`, `--mf-font-heading`.

**Type scale:** `--mf-text-h1`…`--mf-text-h6`, `--mf-text-body`, `--mf-text-body-sm`,
`--mf-text-label` — each with `size` / `line-height` / `weight` / `tracking` sub-tokens.

**Spacing and container width:** three scalar roles — `--mf-max-content-width`,
`--mf-content-margins-lr`, `--mf-content-margins-tb` — plus twelve per-band levers,
`--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}` (six bands × two axes). Base values: both
axis base inputs are `1rem` (`spacing.content-margin-base`); `--mf-max-content-width` defaults to
`80rem` (`spacing.max-content-width`). Mode-independent, like radius and typography.

**Spacing: dual-carry preserved.** DTCG has no calc primitive, so each derived per-band token in
`semantic/layout.json` carries **both** a pre-computed literal `$value` (so the baked `@property`
initial-value is exact) **and** its band + multiplier under
`$extensions["com.moduleforge.breakpoint"]` (so the compiler can emit
`calc(var(--mf-content-margins-lr, …) * k)` and a runtime base override cascades to every band).
This deliberately parallels `semantic/radius.json`'s `$extensions["com.moduleforge.radius"]`
pattern — see [Radius: calc preserved](#radius-calc-preserved) below.

**Spacing: multiplier ladder.**

| Band | `lr` multiplier | `lr` value | `tb` multiplier | `tb` value |
|------|------------------|------------|------------------|------------|
| `base` | 1 | `1rem` | 1.5 | `1.5rem` |
| `sm` | 1.5 | `1.5rem` | 1.5 | `1.5rem` |
| `md` | 1.5 | `1.5rem` | 1.5 | `1.5rem` |
| `lg` | 2 | `2rem` | 2 | `2rem` |
| `xl` | 2 | `2rem` | 2 | `2rem` |
| `2xl` | 2 | `2rem` | 2 | `2rem` |

The inline (`lr`) ladder deliberately reproduces the ubiquitous `px-4 sm:px-6 lg:px-8`
shadcn/Tailwind app-shell idiom, so the default render is unsurprising. Every band is declared even
where its value repeats the band below it (e.g. `sm` and `md` share `1.5rem` on both axes), so every
band still has its own independent override lever rather than silently inheriting from the band
below.

## Material (MD3) compatibility by naming, not runtime dependency

Semantic names deliberately track MD3 color-role vocabulary (Primary / Secondary / Tertiary +
surface / error roles, with `-foreground` ≈ MD3 `on-`). This achieves Material compatibility
**by naming only** — there is no Material Web / MUI runtime dependency (Material Web is in
maintenance mode; `@material/material-color-utilities` remains optional/future).

## Like-for-like migration: no visual drift on existing tokens

Every color value already present in `mod-core/gui/.ladle/styles.css` is preserved **exactly** via
the raw tier: the light `:root` block and the `.dark` block both round-trip unchanged. Task `002`'s
compiled default therefore matches today's rendered appearance. The palette was **not** redesigned
here. Spot-check crosswalk (raw step ← exact oklch):

- `neutral.0` = `oklch(1 0 0)`, `neutral.50` = `oklch(0.985 0 0)`, `neutral.100` = `oklch(0.97 0 0)`,
  `neutral.200` = `oklch(0.922 0 0)`, `neutral.400` = `oklch(0.708 0 0)`, `neutral.500` = `oklch(0.556 0 0)`,
  `neutral.700` = `oklch(0.269 0 0)`, `neutral.800` = `oklch(0.205 0 0)`, `neutral.900` = `oklch(0.145 0 0)`,
  `neutral.alpha-10` = `oklch(1 0 0 / 10%)`, `neutral.alpha-15` = `oklch(1 0 0 / 15%)`.
- `red.light` = `oklch(0.577 0.245 27.325)` (light `--destructive`), `red.dark` = `oklch(0.704 0.191 22.216)` (dark `--destructive`).
- `radius.base` = `0.625rem` (`--radius`).

### Newly introduced values (no prior styles.css value → no drift risk)

These roles/steps did **not** exist in `styles.css`; nothing renders them today, so their chosen
defaults cannot introduce drift. They are documented so the manager can review the choices:

- **New raw steps:** `neutral.150` (`0.87`) and `neutral.750` (`0.3`) — only for the new
  `primary-hover` default. New hues `amber`, `blue`, `green`, `purple` — for the new
  warning / info / success / link roles.
- **New roles defaulted to an existing role:** `tertiary`/`tertiary-foreground` (= secondary),
  `text-heading` (= text-body), `brand-highlight` (= primary — mod-core's palette is achromatic,
  so no distinct brand accent exists; expected to be set by a brand style package such as
  Liquid Labs), `error-foreground` (`styles.css` had no `--destructive-foreground`).
- **New roles given a fresh functional value:** `primary-hover` (lightened/darkened primary),
  `warning` (amber), `info` (blue), `success` (green), `link` family (blue + purple for visited).
  Links get a perceivable blue rather than inheriting the achromatic body color, for
  accessibility; all are overridable by style packages.

### Radius: calc preserved

`styles.css` defines the derived radii as `calc(var(--radius) * k)`
(`sm`×0.6, `md`×0.8, `lg`×1.0, `xl`×1.4). DTCG has no calc primitive, so `semantic/radius.json`
carries **both** the pre-computed literal `$value` (so the baked default matches today exactly)
**and** the multiplier under `$extensions["com.moduleforge.radius"]`. Task `002` should prefer
emitting `calc(var(--mf-radius) * k)` so a runtime `--mf-radius` override cascades to the derived
steps.

### Font stacks caveat

`mod-core/gui/.ladle/styles.css` does **not** define `--font-sans` / `--font-mono` literal values
— its `@theme` block maps `--font-sans` to `var(--font-sans)`, whose value comes from Tailwind v4 /
shadcn defaults. `base/font.json` therefore captures the standard Tailwind v4 system-font
equivalents so the token source is self-contained. If task `002` needs byte-exact parity with the
Tailwind-resolved stacks, confirm against the running Tailwind build.

## Deferred additions (known, NOT dropped — out of scope for this task)

`app-mfdemo`'s `src/app/globals.css` has drifted from mod-core and adds token families that are
**intentionally not** folded in here. They are reconciled in the downstream adoption plan
(Plan 2), not silently dropped:

- **`--sidebar-*`** — the sidebar color family (`--sidebar`, `--sidebar-foreground`,
  `--sidebar-primary`, etc.).
- **`--chart-*`** — the chart categorical color family (`--chart-1`…`--chart-5`).
- **Extra `--radius-*` steps** beyond mod-core's `sm`/`md`/`lg`/`xl`.
- **`--font-geist-mono`** — app-mfdemo maps `--font-mono` to the Next.js Geist font var rather than
  mod-core's stack.

## Format notes

- Color `$value`s are CSS-native **oklch strings** (not DTCG color objects), to preserve the exact
  existing values verbatim; task `002`'s Style Dictionary config parses them as-is.
- Alias syntax is DTCG reference form `{group.token}` (e.g. `{color.neutral.800}`), resolved by
  Style Dictionary's deep merge of all source files.
- The component-override tier's `$example` group is illustrative only; task `002` must not emit
  tokens from under `$example`.
