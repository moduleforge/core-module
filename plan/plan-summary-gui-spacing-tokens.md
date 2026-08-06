# Plan Summary: gui-spacing-tokens

## What was planned and why

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

## What shipped

### Phase 01 — Spacing Token Contract

1. **Add Spacing Token Sources** (`001-add-spacing-token-sources.md`, tier `sonnet-high`) — Authored the two new DTCG token source files for the spacing/container-width category: gui/tokens/base/spacing.json (raw tier, two literal dimension tokens) and gui/tokens/semantic/layout.json (semantic tier, three scalar mf.* aliases plus twelve derived per-band tokens under mf.content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}, each dual-carrying a pre-computed literal $value and a $extensions["com.moduleforge.breakpoint"] block with band/base/multiplier, following the exact pattern semantic/radius.json already uses for com.moduleforge.radius). No compiler code, CSS, or docs were touched. All seven validation checks passed. Task document updated with a Status section; all changes committed at e97bebd.
   Commit `e97bebd`, merged at `78bb789`.

2. **Emit Spacing Tokens And Container Utility** (`002-emit-spacing-tokens-and-container-utility.md`, tier `opus-high`) — Extended gui/style-dictionary/build-tokens.mjs to carry the new spacing/container-width surface: 15 typed @property registrations, 15 :root baked defaults, a --container-content @theme inline key, and a new containerUtilityBlock() emitting @utility container with margin-inline:auto, a token-backed max-width, and six-band per-axis padding ladders shaped var(<per-band lever>, calc(<base lever> * k)). Band names/multipliers read from token $extensions; compiler-side ordering only, unknown band throws. All ten validation checks passed including the Tailwind compile probe (two .container rules, built-in first, correct @variant breakpoints, utility overrides still win).
   Commit `e03855a`, merged at `4619fa3`.

3. **Bump Token Contract Version** (`003-bump-token-contract-version.md`, tier `haiku-med`) — Bumped MF_TOKEN_CONTRACT_VERSION from 1.0.0 to 1.1.0 to reflect the addition of spacing/container-width token roles. Updated the constant's doc comment to describe the new surface (--mf-max-content-width, --mf-content-margins-lr/tb, and their per-breakpoint-band overrides) and to explicitly state why this is a MINOR bump: new roles only, with no existing roles removed, renamed, or revalued. All validation checks passed - typecheck, 53 unit tests, and verification that no stale hardcoded version references remain in the codebase.
   Commit `421bc66`, merged at `e454f60`.

4. **Export Token Source Css** (`004-export-token-source-css.md`, tier `sonnet-high`) — Extended gui/package.json's build script with a build:tokens-css step that copies the compiler-emitted tokens/dist/tokens.css into dist/tokens.css, and added a ./tokens.css export alongside the existing ./styles.css export. Documented both exports in gui/README.md. Verified end-to-end: built dist/tokens.css is byte-identical to the compiler's own output, contains @theme inline and @utility container, resolves via Node exports resolution, and yields the expected .container/.max-w-content/.bg-primary rules when fed into a real Tailwind v4 build. Full build, test, and typecheck pass; dist/index.css unaffected; no downstream consumer touched.
   Commit `3293577`, merged at `314c5a6`.

### Phase 02 — Contract Documentation

1. **Document Spacing Tokens In Contract** (`001-document-spacing-tokens-in-contract.md`, tier `sonnet-high`) — Documented the new spacing/container-width token category in gui/tokens/CONTRACT.md (new Spacing and container width subsection with resolution-expression, band-span-semantics, settable-vs-internal-divergence, fluid-value, and known-limitation sub-sections; new Tailwind container integration subsection; divergence note under settable-vs-internal table; removal of a stray trailing artifact) and in gui/tokens/README.md (file-layout entries, tiering-convention row, enumeration line, dual-carry paragraph, multiplier-ladder table). Regenerated tokens/dist/tokens.css and cross-checked all 15 new settable names plus -default @property twins against the emitted bundle — no discrepancies. All 10 validation checks passed.
   Commit `6af006e`, merged at `8c26596`.

2. **Document Per Band Override For Style Packages** (`002-document-per-band-override-for-style-packages.md`, tier `sonnet-high`) — Extended gui/tokens/STYLE-PACKAGE-CONTRACT.md at four prescribed insertion points (Hard rules, Emission rules, Token-contract versioning, Security posture) plus the build-convention directory layout, documenting the new per-breakpoint-band override capability for --mf-content-margins-{lr,tb} and its 12 per-band forms alongside the single-scalar --mf-max-content-width. Stated explicitly, as a standalone sentence in Emission rules, that a derived per-band step is deliberately settable here unlike radius. Added a worked sm-band span example (40rem-48rem). Bumped documented contract version narrative to 1.1.0/MINOR, cross-checked against token-contract-version.ts. Extended security posture for the new <length>-typed tokens. Removed two stray </content>/</invoke> artifact lines at file end. Regenerated tokens/dist/tokens.css and cross-checked all 15 documented settable names against the emitted bundle. All 12 validation checks passed.
   Commit `950f268`, merged at `3048113`.

3. **Record Deferred Token Followups** (`003-record-deferred-token-followups.md`, tier `sonnet-med`) — Verified all file paths named in the three deferred-item descriptions exist (app-mftodo's three container files, mod-core's ProfileEditor.tsx, all gui/tokens/* references), confirmed the eight mx-auto max-w-2xl p-6 call sites across app-mftodo's three container files matches the task doc's count exactly, and confirmed item 3's text stays consistent with gui/tokens/CONTRACT.md's new Known limitation paragraph (added by the concurrently-running task 001, cross-checked after it landed). Drafted final, ready-to-file content for all three followup items (Per-component radius override levers; Adopt spacing tokens in downstream GUIs; Narrow-measure content width token), each with type:enhancement and the plan/phase:contract-documentation provenance tag. The manager filed all three via followups_add directly (ids 7nrP, AkGw, XKY2) since this task's dispatch tier lacked the followups_add tool; the task's substantive verification and drafting work is otherwise complete.
   Commit `306109a`, merged at `6beb7cc`.

### Phase 03 — Documentation Updates

1. **Update Architecture Docs** (`001-update-architecture-docs.md`, tier `sonnet-high`) — Added a new ### gui/ design tokens subsection to AGENTS.md, pointing at gui/tokens/, the Style Dictionary compiler, both package stylesheet exports, and both contract docs. Judged gui/README.md and root README.md to need no edits. Verified the submodule pointer was still at pre-plan SHA (task 002's job), then read the sibling doc's actual merged content directly from docs-mf-standards's own local checkout (main @ e96646d) and cross-checked its token-shape account, precedence chain, and @utility container integration against CONTRACT.md and a freshly regenerated tokens/dist/tokens.css: byte-for-byte match, no drift.
   Commit `20d2591`, merged at `ed526e8`.

2. **Bump Mf Standards Submodule Pointer** (`002-bump-mf-standards-submodule-pointer.md`, tier `sonnet-med`) — Bumped the docs/mf-standards submodule pointer from 1ab046e0b1f710497dcf81013bf9ab8fea3b479f to e96646d6e70421e394d8b13da666d1b1e956b448, the main tip of the sibling docs-mf-standards project's completed gui-spacing-tokens federated-plan work, fetched from the local sibling checkout. Verified before pinning that the target commit's architecture/gui-design-tokens.md contains the new Spacing and container-width tokens section and does not contradict gui/tokens/CONTRACT.md. Confirmed no mod-core-side inbound markdown link into docs/mf-standards/ broke. bun run test and typecheck remain green.
   Commit `69c95f2`, merged at `d212e49`.

## Key decisions

_No `## Why this shape` section is recorded in `plan/overview.md`, so this plan's cross-task rationale was never written down. Per-task outcomes are under "What shipped" above._

## Follow-up items

- **`dzEE`** — **Factual correction needed in plan/notes/token** — Factual correction needed in plan/notes/token-shape-decision.md and this task doc's validation-8 parenthetical: both claim dist/index.css will not contain a compiled .container rule because mod-core's own source never uses the class. That's wrong — baseline dist/index.css already had 6 .container{ rules (built-in, minification-split) and now has 12, because Tailwind's candidate extractor picks up the bare identifier `container` from `const { container } = render(...)` in gui/src/FieldError.test.tsx and gui/src/ErrorBanner.test.tsx. dist/index.css still carries zero @utility at-rules (verified), so a downstream consumer's own Tailwind pass genuinely cannot define the utility from the published bundle; the .container rule that IS there today is incidental and could change. Relevant to Phase 2 doc authors and to docs-mf-standards phase 4, which reads notes/token-shape-decision.md as source — the 'it gets purged' claim should not be repeated.

- **`yA2l`** — **Renaming decision worth a second look: colorL** — Renaming decision worth a second look: colorLightTokens -> lightAndModeIndependentTokens was optional per the task doc but accounts for a visible share of the diff. Trivially revertible if a reviewer wants a minimal diff.

- **`KJF0`** — **Pre-existing, untouched: a stale cross-refere** — Pre-existing, untouched: a stale cross-reference in the compiler header — "Why three separate Style Dictionary instances" note ends with "flagged to the manager in the task-002 report," referring to a PRIOR plan's task 002 (primitive audit), now ambiguous next to this plan's task 002. Not safely rewordable without knowing which plan it should name.

- **`uzKR`** — **The mf.text-body collision documented in the** — The mf.text-body collision documented in the compiler header (color-role tier vs typography-scale tier, worked around by the three-instance split) is still latent. New sources introduce no new collision (verified: appear only in the first instance, count exactly +15) but the underlying source-level hazard is unchanged and unowned.

- **`kdAa`** — **Task doc's validation step 6 literal command** — Task doc's validation step 6 literal command (tailwindcss -i /tmp/consumer-probe.css ... --content /tmp/consumer-probe.html) does not work in this environment: Tailwind v4's CLI resolves the bare @import "tailwindcss" specifier via Node-style resolution starting at the importing CSS file's own directory and walking up for node_modules; /tmp has no node_modules ancestor. Not environment-specific — inherent to the resolver. Agent substituted an equivalent probe from an untracked, deleted-before-commit temp directory inside gui/ with identical results. Suggest updating the task doc's validation step 6 to use an in-tree (but untracked) location instead of /tmp for future re-runs.

- **`PhFO`** — **Not a defect in this task's scope: gui/tokens** — Not a defect in this task's scope: gui/tokens/STYLE-PACKAGE-CONTRACT.md (owned by Phase 2 task 002, not touched here) also had a stray </content>/</invoke> artifact at file end, same class as the one removed from CONTRACT.md here. NOTE: this is already resolved — task 002's own report confirms it removed the identical artifact from STYLE-PACKAGE-CONTRACT.md as part of its own work, so no further action needed; recording only because task 001 and 002 ran concurrently and 001 could not see 002's fix at the time it flagged this.

- **`7nrP`** — **Per-component radius override levers** — gui/tokens/CONTRACT.md documents --mf-radius as the single settable radius lever, with --radius-sm/-md/-lg/-xl derived from it via calc(var(--mf-radius, var(--mf-radius-default)) * k) and explicitly not individually settable. A brand that wants, say, pill-shaped buttons but square-cornered cards cannot express that — every primitive rescales together. The component-override tier (gui/tokens/component/overrides.json, mf.component.<component>.<property>) already exists as the convention for exactly this kind of escape hatch and is intentionally empty; it is the natural home. Note that the per-breakpoint-band lever family added for --mf-content-margins-* is a worked precedent for layering a second override axis onto a derived-calc() token, and could be adapted — the axis there is breakpoint, here it would be component. Not designed, not scoped.

- **`AkGw`** — **Adopt spacing tokens in downstream GUIs** — app-mftodo/gui/src/tasks/TaskListContainer.tsx, TaskDetailContainer.tsx, and TaskEditorContainer.tsx hardcode mx-auto max-w-2xl p-6 across eight call sites, and mod-core/gui/src/ProfileEditor.tsx hardcodes p-6 max-w-xl. These should consume @moduleforge/core-gui's container utility (or max-w-content) via the @moduleforge/core-gui/tokens.css export, so page width and gutters become brand-overridable instead of per-component literals. This mirrors the existing adoption-gap follow-on already recorded for mod-tags, mod-tasks, mod-contacts, mod-users, and app-mfdemo. Note the concrete blocker an implementer will hit immediately: --mf-max-content-width defaults to 80rem while these pages currently sit at max-w-2xl (42rem), so adoption is not a mechanical substitution — it needs a per-app --mf-max-content-width override or a narrower wrapper, and is a visible layout change either way.

- **`XKY2`** — **Narrow-measure content width token** — --mf-max-content-width is a single global scalar, so an application whose shell is wide (80rem) but whose reading-oriented pages want a narrow measure (~42rem) cannot express both through the token contract. The obvious extensions are a second role (--mf-max-content-width-narrow) or a container-narrow utility emitted alongside container by gui/style-dictionary/build-tokens.mjs. gui/tokens/CONTRACT.md records this as an accepted open limitation; this item is the tracked follow-up for closing it. Deliberately not designed here — the right shape depends on what downstream adoption actually needs.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — Spacing Token Contract

- [x] [001-add-spacing-token-sources.md](./phase-01-spacing-token-contract/001-add-spacing-token-sources.md) — tier `sonnet-high` · branch `plan/gui-spacing-tokens-01-001` · commit `e97bebd` · merge `78bb789`
- [x] [002-emit-spacing-tokens-and-container-utility.md](./phase-01-spacing-token-contract/002-emit-spacing-tokens-and-container-utility.md) — tier `opus-high` · branch `plan/gui-spacing-tokens-01-002` · commit `e03855a` · merge `4619fa3`
- [x] [003-bump-token-contract-version.md](./phase-01-spacing-token-contract/003-bump-token-contract-version.md) — tier `haiku-med` · branch `plan/gui-spacing-tokens-01-003` · commit `421bc66` · merge `e454f60`
- [x] [004-export-token-source-css.md](./phase-01-spacing-token-contract/004-export-token-source-css.md) — tier `sonnet-high` · branch `plan/gui-spacing-tokens-01-004` · commit `3293577` · merge `314c5a6`

### Phase 02 — Contract Documentation

- [x] [001-document-spacing-tokens-in-contract.md](./phase-02-contract-documentation/001-document-spacing-tokens-in-contract.md) — tier `sonnet-high` · branch `plan/gui-spacing-tokens-02-001` · commit `6af006e` · merge `8c26596`
- [x] [002-document-per-band-override-for-style-packages.md](./phase-02-contract-documentation/002-document-per-band-override-for-style-packages.md) — tier `sonnet-high` · branch `plan/gui-spacing-tokens-02-002` · commit `950f268` · merge `3048113`
- [x] [003-record-deferred-token-followups.md](./phase-02-contract-documentation/003-record-deferred-token-followups.md) — tier `sonnet-med` · branch `plan/gui-spacing-tokens-02-003` · commit `306109a` · merge `6beb7cc`

### Phase 03 — Documentation Updates

- [x] [001-update-architecture-docs.md](./phase-03-doc-updates/001-update-architecture-docs.md) — tier `sonnet-high` · branch `plan/gui-spacing-tokens-03-001` · commit `20d2591` · merge `ed526e8`
- [x] [002-bump-mf-standards-submodule-pointer.md](./phase-03-doc-updates/002-bump-mf-standards-submodule-pointer.md) — tier `sonnet-med` · branch `plan/gui-spacing-tokens-03-002` · commit `69c95f2` · merge `d212e49`
