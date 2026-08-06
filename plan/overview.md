# GUI spacing and container-width design tokens

## Purpose and scope

Extend mod-core's `--mf-*` design-token contract (`gui/tokens/`) with a new **spacing /
container-width** token category, so downstream ModuleForge apps can consume brand-overridable
page-content width and margin values instead of hardcoding raw Tailwind literals per component
(`app-mftodo`'s task pages currently repeat `mx-auto max-w-2xl p-6` across three containers).

Today the contract governs colour (35 roles), radius (`--mf-radius` plus derived `sm`/`md`/`lg`/`xl`
steps via `calc()`), and typography. There is no token for page-content max-width or margins at all.

### This is one half of a federated plan

The plan slug `gui-spacing-tokens` spans two repositories:

- **`mod-core` (this project, dispatched first)** — owns the token contract and builds the concrete
  mechanism. The design decisions recorded here are authoritative.
- **`docs-mf-standards` (dispatched second)** — the cross-cutting architecture-documentation module
  mod-core consumes as a git submodule at `docs/mf-standards/`. Its documentation-only follow-on
  updates `architecture/gui-design-tokens.md` to describe what this project actually builds. It reads
  this `overview.md` and [`notes/token-shape-decision.md`](./notes/token-shape-decision.md) as its
  source; nothing flows the other way.

### What must change

1. Two new DTCG token source files (raw + semantic tier) defining the new roles.
2. `gui/style-dictionary/build-tokens.mjs` — emit the new `@property` registrations, the new
   `@theme inline` key, and a compiler-generated `@utility container` block.
3. `gui/src/lib/token-contract-version.ts` — a MINOR bump, `1.0.0` → `1.1.0` (new roles added,
   existing roles unchanged).
4. `gui/package.json` + build script — publish the Tailwind-*source* token CSS as a consumable
   export, without which none of the above is reachable by a downstream app.
5. `gui/tokens/CONTRACT.md`, `gui/tokens/README.md`, `gui/tokens/STYLE-PACKAGE-CONTRACT.md` — the
   in-code source-of-truth contract docs.
6. `plan/followups.yaml` — record the deferred per-component-type radius extension and the
   downstream-adoption follow-on.
7. `docs/mf-standards` submodule pointer — bumped last, once the sibling project's work merges.

### What must not change

- **Colour tokens.** Confirmed correct; no work needed.
- **The existing fallback-chaining contract or the `data-mf-theme` scoping mechanism.** The new
  tokens are mode-independent and slot into the existing machinery unchanged.
- **Any existing `--mf-*` role name or baked value.** This release is additive — that is what makes
  it a MINOR contract bump.
- **`docs/mf-standards/**` content.** Submodule-mounted and owned by the sibling project. This
  project may read it and may move the submodule *pointer*, but must never edit a file inside it.

### Explicitly out of scope

- **Per-component-type radius overrides** (independently settable button vs. card radius, against
  today's single `--mf-radius` lever). Deliberately deferred; recorded as a follow-up note only.
- **Downstream adoption** — rewiring `app-mftodo` or any other app/module to consume the new tokens
  in place of hardcoded literals. This mirrors the adoption-gap follow-on already documented in
  `gui-design-tokens.md`. The need is recorded; the work is not done here. (Note the distinction
  from item 4 above: publishing a consumable export from mod-core's own package is *this* project's
  surface, not downstream adoption.)

### Success criteria

- `bun run build:tokens` in `gui/` emits `tokens/dist/tokens.css` containing typed `@property`
  registrations for every new token, the new `--container-content` `@theme inline` key, and an
  `@utility container` block with all six breakpoint bands on both axes.
- A Tailwind compile of `.ladle/styles.css` succeeds and produces a `.container` rule whose
  `max-width` and per-band `padding-inline` / `padding-block` resolve through the documented
  fallback chains.
- Setting `--mf-content-margins-lr` alone rescales every band; setting
  `--mf-content-margins-lr-lg` alone changes the `lg` band and nothing else.
- The three contract docs enumerate the new roles the same way radius/colour/typography are
  enumerated today, and `STYLE-PACKAGE-CONTRACT.md` documents the per-band override as a new,
  explicitly-permitted style-package capability.
- `MF_TOKEN_CONTRACT_VERSION === '1.1.0'`.
- No existing test, typecheck, or build step regresses.

## Current status

Planning complete; no implementation work has started. Phase 1 begins first.

**Pre-conditions the first implementing agent must know:**

- `mod-core/gui/node_modules` is **broken in the current checkout** — its `tailwindcss` and
  `@tailwindcss/*` entries are symlinks into a `bounded-width-task-editor/node_modules/.bun/…`
  directory that no longer exists. Any task that runs a `gui/` build must run `bun install` inside
  `gui/` first. Follow-up `0xGl` already records the sibling form of this problem for task worktrees.
- `gui/tokens/dist/` is **gitignored** — the compiled bundle is a build artifact, regenerated by
  `bun run build:tokens`, never committed. Do not add it to git.
- Two committed docs carry stray trailing agent-tooling artifacts that should be removed while
  editing them: `gui/tokens/CONTRACT.md` ends with a spurious `</content>` line, and
  `gui/tokens/STYLE-PACKAGE-CONTRACT.md` ends with spurious `</content>` and `</invoke>` lines.
- The `docs/mf-standards` submodule is pinned at `1ab046e0b1f710497dcf81013bf9ab8fea3b479f`.

## Overview

Three phases. Phase 1 builds and ships the mechanism; Phase 2 writes the contract documentation that
describes it; Phase 3 handles the architecture-doc surface and the cross-repository submodule bump
that must come last.

### The chosen token shape (authoritative summary)

Full reasoning, the alternatives weighed, and the exact value tables are in
[`notes/token-shape-decision.md`](./notes/token-shape-decision.md); the measured Tailwind behavior it
rests on is in [`notes/tailwind-container-mechanics.md`](./notes/tailwind-container-mechanics.md).
The essentials:

**`--mf-max-content-width`** is a single scalar with an ordinary
`var(--mf-max-content-width, var(--mf-max-content-width-default))` chain. Default `80rem`.

**`--mf-content-margins-lr` / `--mf-content-margins-tb`** use a **hybrid**: a derived-multiplier
ladder — precisely the idiom `CONTRACT.md` already documents for radius
(`--radius-md: calc(var(--mf-radius, var(--mf-radius-default)) * 0.8)`) — with a per-band escape
hatch layered on as a single outer `var()`:

```css
padding-inline: var(--mf-content-margins-lr-<band>,
                    calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * <k_band>));
```

Precedence, outward-in: the style package's per-band lever wins for that band alone; otherwise the
style package's base lever rescales *every* band through each band's multiplier; otherwise mod-core's
baked default. This satisfies both halves of the requirement — one calculated relationship the bands
derive from, and a per-breakpoint override that wins for its breakpoint — in one expression, with no
mechanism the contract does not already have.

The alternatives were weighed and rejected on the merits: **discrete per-breakpoint tokens** satisfy
the override half but destroy the single-relationship half (no lever rescales the ladder), and
**a fluid `clamp()` scalar** is the more elegant shape but cannot express a named-breakpoint override
at all. The hybrid is a strict superset of the fluid option — every band's value is typed `<length>`,
and `clamp()` computes to a length, so a brand can still set a fluid curve inside any band.

**Bands** are Tailwind v4's untouched defaults, confirmed by inspection rather than assumed:
`base` (unprefixed) / `sm` 40rem / `md` 48rem / `lg` 64rem / `xl` 80rem / `2xl` 96rem. mod-core
overrides no `--breakpoint-*` key anywhere.

**Tailwind integration** goes through the established container idiom rather than parallel machinery:
a compiler-emitted `@utility container` block declaring `margin-inline: auto`, the token-backed
`max-width`, and per-band `padding-inline` / `padding-block` via `@variant <band>` (so breakpoint
*values* resolve against the consuming build's theme; only the band *names* are baked into mod-core).
Verified empirically: `@utility container` **extends** Tailwind's built-in container rather than
replacing it, works from an `@import`ed file, and its later source position means the token-backed
`max-width` takes ownership of the width behavior while ordinary utilities (`py-0`, `max-w-md`)
still override the container. One `@theme inline` line
(`--container-content: var(--mf-max-content-width, …)`) additionally yields a `max-w-content`
utility.

New settable surface: 15 names — `--mf-max-content-width`, `--mf-content-margins-{lr,tb}`, and
`--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}`. All `$type: dimension` → `@property`
`syntax: "<length>"`, all mode-independent (emitted once in `:root`, never re-emitted in the
`data-mf-theme` scope selectors, exactly like radius and typography).

The one genuinely new style-package capability is that a *derived step's* bare `--mf-x` form is
settable here, where for radius it deliberately is not — which is why
`STYLE-PACKAGE-CONTRACT.md` needs a real edit and not just an enumeration.

### Phase 1 — Spacing token contract *(4 tasks)*

Builds the mechanism and leaves `gui/` building, testing, and publishable.

1. **`001-add-spacing-token-sources`** — author `gui/tokens/base/spacing.json` and
   `gui/tokens/semantic/layout.json`, carrying both pre-computed literals and per-band multipliers
   under `$extensions["com.moduleforge.breakpoint"]`, mirroring `semantic/radius.json`'s dual-carry
   pattern.
2. **`002-emit-spacing-tokens-and-container-utility`** — extend `build-tokens.mjs`: wire the new
   sources into the correct Style Dictionary instance, emit the `@property` registrations and `:root`
   defaults (free, once wired), add the `--container-content` `@theme inline` key, and emit the
   `@utility container` block. **Depends on 001.**
3. **`003-bump-token-contract-version`** — `MF_TOKEN_CONTRACT_VERSION` `1.0.0` → `1.1.0` plus its
   doc comment. Independent of 001/002; parallel-eligible.
4. **`004-export-token-source-css`** — publish `tokens/dist/tokens.css` as a consumable
   `@moduleforge/core-gui/tokens.css` export so a downstream Tailwind build can see the `@theme`
   and `@utility` at-rules. **Depends on 002** (needs the emitted file to exist to validate against).

Parallel-eligible: `003` alongside `001`.

### Phase 2 — Contract documentation *(3 tasks)*

The in-code source-of-truth docs. All three read the *emitted* `tokens/dist/tokens.css` from Phase 1
rather than describing an intent, so they cannot drift from what actually ships.

1. **`001-document-spacing-tokens-in-contract`** — `gui/tokens/CONTRACT.md` and
   `gui/tokens/README.md`: enumerate the new roles alongside radius/colour/typography, document the
   per-band resolution chain and its band-span semantics, the container-utility integration, and the
   two behavior changes to Tailwind's built-in container.
2. **`002-document-per-band-override-for-style-packages`** — `gui/tokens/STYLE-PACKAGE-CONTRACT.md`:
   the per-band override as a new, explicitly-permitted capability, its emission rules, and its place
   in the settable-vs-internal and security-posture framing.
3. **`003-record-deferred-token-followups`** — record the deferred per-component-type radius
   extension, the downstream-adoption follow-on, and the single-width-lever limitation in
   `plan/followups.yaml`.

All three are parallel-eligible with each other — disjoint files, all depending only on Phase 1.

### Phase 3 — Documentation updates *(2 tasks)*

1. **`001-update-architecture-docs`** — reconcile mod-core's own architecture-level doc surface
   (`gui/README.md`, `README.md`, `AGENTS.md`) with the new token category, and verify — read-only —
   that the sibling project's `docs/mf-standards/architecture/gui-design-tokens.md` accurately
   describes what was built, reporting any discrepancy rather than editing the submodule.
2. **`002-bump-mf-standards-submodule-pointer`** — **explicitly sequenced last, and blocked on an
   external event**: the sibling `docs-mf-standards` plan must merge before this can run. It cannot
   be automated inside this project's task graph; the plan executor confirms the sibling landed
   before dispatching it.
