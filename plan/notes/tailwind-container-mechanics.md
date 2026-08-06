# Tailwind v4 container mechanics — empirical findings

## Purpose and scope

Records what was **measured**, not assumed, about Tailwind v4's `container` utility, its breakpoint
set, and the `@utility` / `@variant` directives, as the factual basis for the token shape chosen in
[token-shape-decision.md](./token-shape-decision.md). Every claim below was verified by compiling a
probe stylesheet with the same Tailwind CLI version `mod-core/gui` pins (`@tailwindcss/cli` 4.3.1)
and reading the emitted CSS.

## The current breakpoint set is untouched Tailwind v4 defaults

`mod-core/gui` overrides no `--breakpoint-*` key anywhere. A sweep of every `.css` / `.ts` / `.tsx` /
`.mjs` / `.json` file under `gui/` (excluding `node_modules/` and `dist/`) found no `--breakpoint-`
declaration, no `@utility` block of any kind, and no existing `container` class usage in mod-core's
own component source. The active set is therefore Tailwind's shipped default, read from
`tailwindcss/theme.css`:

| Band | `--breakpoint-*` | Value |
|------|------------------|-------|
| `base` (unprefixed) | — | below `sm` |
| `sm` | `--breakpoint-sm` | `40rem` |
| `md` | `--breakpoint-md` | `48rem` |
| `lg` | `--breakpoint-lg` | `64rem` |
| `xl` | `--breakpoint-xl` | `80rem` |
| `2xl` | `--breakpoint-2xl` | `96rem` |

## The built-in `container` utility, exactly

Tailwind v4 registers `container` as a *static* utility built from the `--breakpoint` namespace. Its
emitted shape is `width: 100%` plus one `max-width` per breakpoint:

```css
.container {
  width: 100%;
  @media (width >= 40rem) { max-width: 40rem; }
  @media (width >= 48rem) { max-width: 48rem; }
  @media (width >= 64rem) { max-width: 64rem; }
  @media (width >= 80rem) { max-width: 80rem; }
  @media (width >= 96rem) { max-width: 96rem; }
}
```

There is **no** centering and **no** padding — v3's `theme.container.center` / `theme.container.padding`
config keys do not exist in v4. That is why v4's documented customization path is to extend the
utility with `@utility container`.

## `@utility container` extends, it does not replace — confirmed

Compiling an input that declares `@utility container { … }` emits **two** `.container` rules inside
`@layer utilities`: the built-in first, the custom one second.

Consequences, both load-bearing for the design:

1. **A `max-width` declared in the custom block wins over the entire built-in ladder.** Both rules
   have identical specificity `(0,1,0)` and media queries add none, so the later rule's unconditional
   `max-width` beats every `@media`-nested `max-width` in the earlier rule. Declaring
   `max-width: var(--mf-max-content-width, …)` therefore takes complete ownership of the container's
   width behavior; the built-in breakpoint ladder becomes inert rather than fighting the token.
2. **Emission order (built-in, then custom) is what makes this work.** It should be asserted on the
   compiled output rather than trusted, since it is an ordering property of the compiler, not a
   documented guarantee. The implementation task carries that assertion in its `## Validation`.

## `@utility` works from an `@import`ed file — confirmed

An `@utility container` block placed in a file reached via `@import "./tokens.css"` (itself reached
from an entry point that also does `@import "tailwindcss"`) is processed normally. This is what lets
the compiler-generated `tokens/dist/tokens.css` own the block, rather than hand-writing it into
`.ladle/styles.css`.

## Nested `@media` and `@variant` inside `@utility` — confirmed

All three per-band spellings compile:

| Spelling | Emits | Tracks a consumer's `--breakpoint-*` override? |
|----------|-------|-----------------------------------------------|
| `@media (width >= 40rem) { … }` | `@media (width >= 40rem)` | No — value is frozen at authoring time |
| `@media (width >= --theme(--breakpoint-sm)) { … }` | `@media (width >= 40rem)` | Yes |
| `@variant sm { … }` | `@media (width >= 40rem)` | Yes |

`@variant 2xl { … }` also compiles correctly (→ `@media (width >= 96rem)`) — a leading digit in the
variant name is not a problem.

`@variant <band>` is the chosen spelling: it is the shortest, it is the Tailwind-idiomatic way to say
"this breakpoint," and because it resolves against the theme at the *consumer's* build time, a
downstream app that customizes `--breakpoint-lg` gets a container whose gutter step moves with it.
Only the band *names* are baked into mod-core's compiler; the values never are.

## Ordinary utilities still override the container — confirmed

Compiling `class="container py-0 max-w-md"` emits `.max-w-md` and `.py-0` **after** both `.container`
rules, so a consumer can still opt out of any container declaration with a plain utility. This is the
escape hatch that makes it safe for the custom block to declare `padding-block`, which the built-in
container never does.

## Verified probe output

The final probe input and its compiled output (abridged to the utilities layer):

```css
/* input */
@utility container {
  margin-inline: auto;
  max-width: var(--mf-max-content-width, var(--mf-max-content-width-default));
  padding-inline: var(--mf-content-margins-lr-base, calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * 1));
  padding-block:  var(--mf-content-margins-tb-base, calc(var(--mf-content-margins-tb, var(--mf-content-margins-tb-default)) * 1.5));
  @variant sm  { padding-inline: var(--mf-content-margins-lr-sm,  calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * 1.5)); }
  @variant 2xl { padding-inline: var(--mf-content-margins-lr-2xl, calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * 2)); }
}
```

```css
/* output, @layer utilities */
.container { width: 100%; @media (width >= 40rem) { max-width: 40rem; } /* …4 more… */ }
.container {
  margin-inline: auto;
  max-width: var(--mf-max-content-width, var(--mf-max-content-width-default));
  padding-inline: var(--mf-content-margins-lr-base, calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * 1));
  padding-block: var(--mf-content-margins-tb-base, calc(var(--mf-content-margins-tb, var(--mf-content-margins-tb-default)) * 1.5));
  @media (width >= 40rem) { padding-inline: var(--mf-content-margins-lr-sm, calc(…)); }
  @media (width >= 96rem) { padding-inline: var(--mf-content-margins-lr-2xl, calc(…)); }
}
.max-w-md { max-width: var(--container-md); }
.py-0 { padding-block: 0; }
```

Custom-property substitution is verbatim — Tailwind does not attempt to evaluate or rewrite the
nested `var()` / `calc()` chains, so an arbitrarily deep fallback chain passes through intact.

## Band-override semantics fall out of declaration order

Because every band's `padding-inline` is a separate declaration in one rule, and each band's
`@media` is min-width, at any viewport the **last matching** declaration wins. A per-band override
therefore governs its band's *span* — from that breakpoint up to the next declared band — not every
width above it. This is the correct and intended reading of "override just this one named
breakpoint," but it is non-obvious and must be stated explicitly in `CONTRACT.md`.

## Environment caveat found while probing

`mod-core/gui/node_modules` is currently **broken**: its `tailwindcss` and `@tailwindcss/*` entries
are symlinks into `/Users/zane/playground/moduleforge/bounded-width-task-editor/node_modules/.bun/…`,
a directory that no longer exists. `bun run build:tokens`, `bun run build:css`, and the Ladle dev
server therefore cannot run in the current checkout without a fresh `bun install` in `gui/`. Every
implementation task in this plan that runs a `gui/` build must run `bun install` in `gui/` first.
(A pre-existing follow-up, `0xGl` in `plan/followups.yaml`, already records that task worktrees
touching `gui/` need their own `bun install`; this is the same class of problem, in the main
checkout.)
