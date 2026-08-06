# Spacing/container token shape — the decision and the alternatives weighed

## Purpose and scope

The design record for the new spacing/container-width token category: the three candidate shapes,
why the hybrid was chosen, and the exact token surface and resolution expressions that follow from
it. This note is the authoritative description of the chosen shape. `plan/overview.md` summarizes
it; `gui/tokens/CONTRACT.md` and `gui/tokens/STYLE-PACKAGE-CONTRACT.md` are where it becomes
shipped contract. The companion `docs-mf-standards` plan transcribes from here and from
`plan/overview.md`.

Grounded in the measured behavior recorded in
[tailwind-container-mechanics.md](./tailwind-container-mechanics.md).

## The two requirements, restated precisely

1. **`--mf-max-content-width`** — a single scalar governing the max width page content grows to.
   Sparsely overridable by a style package through the ordinary
   `var(--mf-x, var(--mf-x-default))` chain. No second axis. This one is uncontroversial.
2. **`--mf-content-margins-lr` / `--mf-content-margins-tb`** — the margin applied below
   `--mf-max-content-width`, on the inline and block axes respectively. These must
   **(a)** be defined by *one* base/calculated relationship from which each breakpoint's value is
   derived, and **(b)** let an explicit per-breakpoint override, when a style package supplies one,
   win over the calculated value *for that breakpoint only*.

Requirement 2 is the architecturally novel part: it is the first `--mf-*` role needing a second axis
(breakpoint) on top of the existing style-package-override axis.

## Candidate A — discrete per-breakpoint tokens

One baked default and one settable lever per band, each assigned inside its own `@media` block.

**Rejected.** It satisfies 2(b) but *fails 2(a)*. With each band an independent literal there is no
single relationship the bands derive from, and therefore no single lever a brand can pull to rescale
the whole ladder — a brand wanting "everything 25% roomier" would have to restate all six bands on
both axes. It also multiplies the emitted `:root`/`@media` scope blocks, which is new structural
surface for both the compiler and the emitted bundle, for no gain over Candidate C.

## Candidate B — a single fluid `clamp()` scalar

One token per axis, e.g. `clamp(1rem, 4vw, 2rem)`. No `@media` at all; scales continuously.

**Rejected as the primary mechanism.** It is the most elegant option and stays perfectly consistent
with every other token in the contract being a single scalar — but it *fails 2(b)* outright: there is
no named breakpoint to override, so "let an explicit per-breakpoint override win for that specific
breakpoint" is simply not expressible. A secondary problem: a style package overriding the scalar
would replace the entire curve with whatever it supplies, so the brand author has to understand and
restate the fluid expression rather than adjust one number.

**But it is not lost.** Under Candidate C every band's value is typed `<length>`, and `clamp()` with
a `vw` term computes to a length — so a brand that *wants* a fluid curve can set
`--mf-content-margins-lr-lg: clamp(1rem, 4vw, 3rem)` and get exactly Candidate B's behavior inside
that band. Candidate C is a strict superset. This is worth stating in `CONTRACT.md`, because it is
the answer to "can I do fluid spacing?" and the answer is yes.

## Candidate C — derived-multiplier ladder plus a per-band escape hatch (chosen)

This is the hybrid, and it is not a compromise between A and B so much as a direct reuse of the
idiom the contract already documents for radius:

```css
/* existing, from CONTRACT.md */
--radius-md: calc(var(--mf-radius, var(--mf-radius-default)) * 0.8);
```

One settable base lever, a compiler-baked multiplier per derived step, `calc()` doing the derivation
so a runtime override of the base cascades to every step. The new work is a single outer `var()`
wrapping that expression, whose first argument is the per-band lever:

```css
padding-inline: var(--mf-content-margins-lr-<band>,
                    calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * <k_band>));
```

Read outward-in, the precedence is:

1. `--mf-content-margins-lr-<band>` — the style package's explicit override *for this band*. Wins
   outright; the `calc()` is never evaluated.
2. `--mf-content-margins-lr` — the style package's base-scale override. Rescales **every** band
   through each band's own multiplier, exactly as `--mf-radius` rescales every radius step.
3. `--mf-content-margins-lr-default` — mod-core's baked base, when the package sets neither.

That is requirement 2(a) and 2(b) simultaneously, in one expression, with no new mechanism — the
only genuinely new thing is that a *derived step's* bare `--mf-x` form is settable here, where for
radius it deliberately is not. That divergence is the one thing a style-package author must be told
about explicitly, and it is why `STYLE-PACKAGE-CONTRACT.md` needs an edit rather than just an
enumeration.

### Considered and deferred within Candidate C

Emitting the per-band resolution into `:root` `@media` blocks as an intermediate resolved property
(e.g. `--mf-content-margins-lr-current`), with `@utility container` reading only that one property,
was considered. It localizes all breakpoint logic in the token bundle's `:root` blocks and makes the
resolved gutter readable by any component, not only the container. Deferred because it adds a third
custom-property family to the public surface for a consumer that does not exist yet, and because the
per-band levers remain directly readable if such a consumer ever appears. Revisit if a non-container
component needs the current gutter value.

## The resulting token surface

All new tokens are `$type: dimension` → `@property` `syntax: "<length>"`, and all are
**mode-independent**: they live once in `:root` and are *not* re-emitted in the `data-mf-theme`
scope selectors, exactly as radius and typography already are. The compiler's existing
`$type === 'color'` filter on the scoped selectors excludes them automatically.

### Settable levers (the `--mf-x` forms a style package may supply)

| Token | Role |
|-------|------|
| `--mf-max-content-width` | Max width page content grows to. Single scalar, no bands. |
| `--mf-content-margins-lr` | Base input for the inline-axis ladder. Rescales all six bands. |
| `--mf-content-margins-tb` | Base input for the block-axis ladder. Rescales all six bands. |
| `--mf-content-margins-lr-{base,sm,md,lg,xl,2xl}` | Per-band inline override; short-circuits the `calc()` for its band only. |
| `--mf-content-margins-tb-{base,sm,md,lg,xl,2xl}` | Per-band block override; same. |

15 new settable names. `base` names the unprefixed, below-`sm` band — the Tailwind-idiomatic name for
the mobile-first default band.

### Compiler-baked internals

- `--mf-max-content-width-default`, `--mf-content-margins-lr-default`, `--mf-content-margins-tb-default`
  — the three baked defaults that terminate the fallback chains.
- `--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}-default` — baked per-band literals. These exist
  **only** as typed `@property` registrations and human-readable reference values; they are *not* in
  the runtime resolution chain, precisely mirroring the existing
  `--mf-radius-{sm,md,lg,xl}-default` situation that `CONTRACT.md` already describes.

### Proposed default values

Base inputs, both `1rem`, sourced from one new raw token `spacing.content-margin-base`:

| Band | `lr` multiplier | `lr` value | `tb` multiplier | `tb` value |
|------|-----------------|------------|-----------------|------------|
| `base` | 1 | `1rem` | 1.5 | `1.5rem` |
| `sm` | 1.5 | `1.5rem` | 1.5 | `1.5rem` |
| `md` | 1.5 | `1.5rem` | 1.5 | `1.5rem` |
| `lg` | 2 | `2rem` | 2 | `2rem` |
| `xl` | 2 | `2rem` | 2 | `2rem` |
| `2xl` | 2 | `2rem` | 2 | `2rem` |

The inline ladder reproduces the ubiquitous `px-4 sm:px-6 lg:px-8` shadcn/Tailwind app-shell idiom
exactly, so the default render is unsurprising. Every band is declared even where its value repeats
the band below it, so that every band has its own override lever — a uniform surface is worth more
than three saved declarations.

`--mf-max-content-width-default: 80rem` (= Tailwind's `--container-7xl`, and its `xl` breakpoint).
This is the conventional app-shell width. Note it is *wider* than the `max-w-2xl` (42rem) reading
width `app-mftodo`'s task pages currently hardcode — see [Known limitation](#known-limitation-one-width-lever-is-not-always-enough).

### DTCG source layout

Two new files, following the existing tier convention:

- `gui/tokens/base/spacing.json` — raw tier: `spacing.content-margin-base` (`1rem`) and
  `spacing.max-content-width` (`80rem`).
- `gui/tokens/semantic/layout.json` — semantic tier: the `mf.*` roles above, each derived token
  carrying **both** its pre-computed literal `$value` (so the baked `@property` initial-value is
  exact) **and** its band + multiplier under `$extensions["com.moduleforge.breakpoint"]` — the same
  dual-carry pattern `semantic/radius.json` already uses for `com.moduleforge.radius`.

Both files join the compiler's first Style Dictionary instance (the one that already resolves
`base/radius.json` + `semantic/radius.json` alongside the light color set). They must **not** be
added to the dark instance or the typography instance.

## Tailwind integration

### The `@utility container` block, compiler-emitted

Emitted into `gui/tokens/dist/tokens.css` — generated, not hand-written, because its multipliers come
from the token sources and the contract holds that the compiler owns the compiled surface:

```css
@utility container {
  margin-inline: auto;
  max-width: var(--mf-max-content-width, var(--mf-max-content-width-default));
  padding-inline: var(--mf-content-margins-lr-base, calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * 1));
  padding-block:  var(--mf-content-margins-tb-base, calc(var(--mf-content-margins-tb, var(--mf-content-margins-tb-default)) * 1.5));
  @variant sm  { /* padding-inline + padding-block at the sm multipliers */ }
  @variant md  { /* … */ }
  @variant lg  { /* … */ }
  @variant xl  { /* … */ }
  @variant 2xl { /* … */ }
}
```

`@variant <band>` rather than a literal `@media` so the breakpoint *values* resolve against whatever
theme the consuming build has, while only the band *names* are baked into mod-core.

### One `@theme inline` addition

```css
--container-content: var(--mf-max-content-width, var(--mf-max-content-width-default));
```

Tailwind v4's `--container-*` namespace backs the `max-w-*` utilities, so this one line yields a
`max-w-content` utility — a cheap adoption affordance for a consumer that wants the token's width
without the whole container utility.

### Why `padding-inline`, not `margin-inline`

Tailwind's preflight sets `box-sizing: border-box` on everything, so `padding-inline` on the same box
that carries `max-width` puts the gutter *inside* the max-width — content width at wide viewports is
`--mf-max-content-width` minus twice the gutter. That is the reading of "the margin applied below
`--mf-max-content-width`", and it is what v3's `theme.container.padding` did. It is also the only
choice that behaves correctly at narrow viewports, where `width: 100%` plus padding gives an
edge gutter while `margin-inline` would fight the `margin-inline: auto` centering.

## Consequences that must be documented, not just implemented

### The built-in container's max-width ladder becomes inert

Declaring `max-width` in the custom block overrides all five of Tailwind's built-in per-breakpoint
`max-width` declarations (same specificity, later source order). Any consumer already relying on
`.container` stepping through `40rem → 96rem` gets a single `80rem` cap instead. mod-core's own
component source uses `.container` nowhere, so there is no in-repo breakage — but this is a real
behavior change for downstream consumers and belongs in `CONTRACT.md` in as many words.

### `.container` gains block padding, which Tailwind's never had

`padding-block` on the container is a deliberate divergence. It is what `--mf-content-margins-tb` is
*for*, and the escape hatch is verified: an ordinary `py-0` / `py-*` utility is emitted after the
container rules and wins.

### Known limitation — one width lever is not always enough

`--mf-max-content-width` is a single global scalar, so an app whose shell is 80rem wide but whose
reading-oriented pages want ~42rem (precisely `app-mftodo`'s current `max-w-2xl`) cannot express both
through the token. Adoption will need either a per-app `--mf-max-content-width` override or a
narrower wrapper alongside the container. A second role (`--mf-max-content-width-narrow`, or a
`container-narrow` utility) is the obvious future extension. Recorded as a follow-up rather than
built, and worth a sentence in the architecture doc so the sibling project carries it.

## Distribution gap this design surfaces

`@moduleforge/core-gui` today publishes only `./styles.css` → `dist/index.css`, which is
**already-compiled** Tailwind output containing zero `@theme` and zero `@utility` at-rules. That is
why `app-mftodo/gui/src/styles.css` hand-mirrors mod-core's entire `@theme inline` block, with a
comment warning that it must be re-copied whenever mod-core's drifts.

An `@utility container` block has the same problem, only worse: it is not merely a mapping a consumer
can re-copy, it is a utility definition that only exists if the consumer's own Tailwind pass sees it,
and `dist/index.css` will not contain a compiled `.container` rule at all (mod-core's own source
never uses the class, so Tailwind purges it).

**Therefore the new tokens are not reachable downstream unless mod-core also publishes its
Tailwind-*source* token CSS as a consumable export.** The fix is small — have `gui`'s build copy
`tokens/dist/tokens.css` into `dist/` and add an `exports` entry for it — and it simultaneously
retires the existing hand-mirroring hazard. This is mod-core's own package surface, not downstream
adoption, so it is planned here; the judgment call is flagged to the manager.
