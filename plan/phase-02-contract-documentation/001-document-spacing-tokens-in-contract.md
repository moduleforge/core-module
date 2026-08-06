# Document Spacing Tokens In Contract

## Purpose and scope

Document the new spacing / container-width token category in mod-core's two token-side
source-of-truth documents:

- `gui/tokens/CONTRACT.md` — the fixed token-*consumption* contract.
- `gui/tokens/README.md` — the DTCG token *sources* and tiering convention.

Enumerate the new roles the same way colour, radius, and typography are enumerated today, and
document the two mechanisms that are genuinely new: the per-band resolution chain and the
`@utility container` integration.

**Describe what actually shipped, not what was planned.** Read the emitted
`gui/tokens/dist/tokens.css` (regenerate it with `cd gui && bun install && bun run build:tokens` if
absent — the directory is gitignored) and the two token source files, and document those. Where the
emitted output disagrees with this task document, the emitted output is right and this document is
stale: report the discrepancy rather than documenting a fiction.

The style-package *supply* side is Phase 2 task `002`'s file and is deliberately not covered here.

## Requirements

### `gui/tokens/CONTRACT.md`

1. **Add a `### Spacing and container width` subsection** under "The fixed `--mf-*` surface", beside
   the existing `### Radius` and `### Typography` subsections and in the same register. It must
   enumerate:

   - `--mf-max-content-width` — a single scalar, ordinary
     `var(--mf-max-content-width, var(--mf-max-content-width-default))` chain, no second axis.
   - `--mf-content-margins-lr` / `--mf-content-margins-tb` — the base inputs for the inline- and
     block-axis ladders. Setting either rescales **every** band, exactly as `--mf-radius` rescales
     every derived radius step.
   - `--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}` — the per-band levers.

   State that these tokens are **mode-independent** — emitted once in `:root`, never re-emitted in
   the `data-mf-theme` scope selectors — and add them to the "Only color roles are re-emitted in
   scoped selectors" sentence under "Emission shape" so that statement stays accurate.

2. **Document the resolution expression** verbatim, as the section's centrepiece:

   ```css
   var(--mf-content-margins-lr-<band>,
       calc(var(--mf-content-margins-lr, var(--mf-content-margins-lr-default)) * <k_band>))
   ```

   with the precedence spelled out outward-in: per-band lever wins for its band alone → base lever
   rescales every band through that band's multiplier → mod-core's baked default. Note that this is
   the existing `--radius-md: calc(var(--mf-radius, …) * 0.8)` idiom with one outer `var()` added,
   and cross-reference the radius paragraph so the reader sees the two as one family.

3. **Document the band-span semantics explicitly** — this is the non-obvious part and the most
   likely source of a future bug report. Because every band's declaration lives in one rule and every
   band is min-width, the *last matching* declaration wins at any viewport. A per-band override
   therefore governs its band's **span** — from that breakpoint up to the next declared band — not
   every width above it. Give a worked example: setting `--mf-content-margins-lr-sm` alone changes
   the gutter between `40rem` and `48rem` and nowhere else.

4. **Extend the settable-vs-internal table** (or add a note directly beneath it) to cover the new
   shape, because it diverges from the radius rule in a way an author will otherwise get wrong:

   - `--mf-content-margins-{lr,tb}` and `--mf-content-margins-{lr,tb}-<band>` are **all**
     style-package-settable.
   - `--mf-content-margins-{lr,tb}-<band>-default` are compiler-only. Like
     `--mf-radius-{sm,md,lg,xl}-default`, they exist as typed `@property` reference values and are
     **not** part of the runtime chain.
   - Call out the divergence in as many words: for radius, a derived step's bare `--mf-x` form is
     deliberately **not** settable ("override `--mf-radius`, not the derived steps"). For content
     margins it **is** — that per-band lever is the whole point. A reader who has internalized the
     radius rule needs to be told this is different.

5. **Add a `### Tailwind container integration` subsection** under "Where the contract is enforced",
   documenting:

   - The compiler emits an `@utility container` block into `dist/tokens.css`, wiring the tokens into
     Tailwind's established container idiom rather than parallel machinery.
   - It uses `@variant <band>`, so breakpoint *values* resolve against the consuming build's theme
     and only the band *names* are baked into mod-core. A downstream app customizing `--breakpoint-lg`
     gets a container that moves with it.
   - The `@theme inline` key `--container-content` yields a `max-w-content` utility for a consumer
     that wants the width token without the whole container utility.
   - **`@utility container` extends Tailwind's built-in container rather than replacing it.** Both
     `.container` rules are emitted, built-in first. Because media queries add no specificity and the
     custom rule comes second, its unconditional `max-width` **overrides Tailwind's built-in
     per-breakpoint max-width ladder** (`40rem` → `96rem`). This is intended — the token takes
     ownership of the width — but it is a **behavior change for any consumer already using
     `.container`**, and must be stated as such, not buried.
   - **`.container` now carries `padding-block`,** which Tailwind's built-in container never did.
     Intended; it is what `--mf-content-margins-tb` is for. Note the escape hatch: ordinary utilities
     (`py-0`, `py-8`, `max-w-md`) are emitted after the container rules and still win.

6. **Note that a fluid value is expressible.** Every band's value is typed `<length>` and `clamp()`
   with a `vw` term computes to a length, so a style package wanting continuous scaling inside a band
   can set e.g. `--mf-content-margins-lr-lg: clamp(1rem, 4vw, 3rem)`. This is the answer to "can I do
   fluid spacing?" and it is yes.

7. **Note the known limitation.** `--mf-max-content-width` is a single global scalar, so an app whose
   shell is 80rem wide but whose reading-oriented pages want a narrower measure cannot express both
   through the token. Adoption needs either a per-app override or a narrower wrapper alongside the
   container. Record it in the same "known limitation, accepted and open" register the document
   already uses for the `data-mf-theme="inverse"` / `dark:` gap. Phase 2 task `003` files the
   matching follow-up; keep the two consistent.

8. **Remove the stray trailing `</content>` line** at the end of the file. It is a committed
   agent-tooling artifact, not content.

### `gui/tokens/README.md`

1. Add `base/spacing.json` and `semantic/layout.json` to the **File layout** tree, with the same
   one-line descriptions the existing entries carry.
2. Extend the **Tiering convention** table's `semantic/` row (or add a line beneath it) to note that
   the semantic tier now also carries the mode-independent layout/spacing roles alongside
   `radius.json`.
3. Add a **Spacing and container width** subsection to the enumeration area, beside the existing
   `**Radius:**` and `**Typography families:**` lines: the three scalar roles, the twelve per-band
   roles, and the base values (`1rem` base input on both axes, `80rem` max content width).
4. Document the **dual-carry pattern** in the same voice as the existing "Radius: calc preserved"
   subsection: each derived per-band token carries both a pre-computed literal `$value` (so the baked
   `@property` initial-value is exact) and its band + multiplier under
   `$extensions["com.moduleforge.breakpoint"]` (so the compiler can emit the `calc()` and a runtime
   base override cascades). Note the deliberate parallel with `com.moduleforge.radius`.
5. Record the multiplier ladder as a table, and note that the inline ladder deliberately reproduces
   the `px-4 sm:px-6 lg:px-8` app-shell idiom so the default render is unsurprising, and that every
   band is declared even where its value repeats the band below it, so every band has its own
   override lever.

### Do not

- Do not edit `gui/tokens/STYLE-PACKAGE-CONTRACT.md` — Phase 2 task `002` owns it. The two tasks are
  parallel-eligible precisely because their file sets are disjoint.
- Do not edit any file under `gui/tokens/*.json`, `build-tokens.mjs`, or `gui/src/` — Phase 1's
  output is settled.
- Do not edit anything under `docs/mf-standards/` — submodule-mounted, owned by the sibling project.
- Do not restate the style-package *supply*-side rules here; `CONTRACT.md` governs consumption and
  links to `STYLE-PACKAGE-CONTRACT.md` for supply.

## Validation

1. `gui/tokens/CONTRACT.md` enumerates all 15 new settable role names, and each appears at least
   once in the document.
2. The resolution expression appears verbatim in a fenced `css` block.
3. Both container behavior changes (built-in max-width ladder overridden; `padding-block` added) are
   stated in plain language, each in its own paragraph or bullet — not merged into one aside.
4. The settable-vs-internal treatment explicitly contrasts the new per-band levers with the radius
   rule that forbids setting derived steps.
5. `tail -c 200 gui/tokens/CONTRACT.md` shows the file ending in prose — no `</content>` line
   remains. `grep -c '</content>' gui/tokens/CONTRACT.md` returns 0.
6. `gui/tokens/README.md`'s file-layout tree lists both new source files, and its enumeration section
   covers the new roles and the multiplier ladder.
7. **Cross-check every documented name against the emitted bundle.** For each of the 15 settable
   names, `grep` it in `gui/tokens/dist/tokens.css` and confirm it appears; for each `-default` twin
   claimed to be compiler-only, confirm it appears as an `@property` registration. Any name in the
   docs that is not in the bundle, or vice versa, is a defect — halt and report rather than
   papering over it.
8. Both documents' existing "Related documents" sections still resolve — every relative link points
   at a file that exists.
9. Markdown renders cleanly: tables well-formed, code blocks language-tagged, heading levels
   consistent with the surrounding document.
10. `git status` shows exactly two modified files.

## Metadata

architectural_impact: true

## Assumptions

- All of Phase 1 has landed, so `gui/tokens/dist/tokens.css` can be regenerated and read as the
  authority for what actually ships.
- `gui/tokens/dist/` is gitignored, so regenerating it produces no git changes.

## References

- [`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md) — the design record: the
  full token surface, the precedence semantics, the value tables, and the rationale for the chosen
  shape over the alternatives. The prose source for most of this task.
- [`plan/notes/tailwind-container-mechanics.md`](../notes/tailwind-container-mechanics.md) — the
  measured Tailwind behavior behind the two container behavior changes, including the built-in
  container's exact emitted shape and the utility-ordering findings.
- `gui/tokens/CONTRACT.md` — the file being edited. Its `### Radius` subsection, its
  settable-vs-internal table, and its "Known limitation (accepted, still open)" paragraph are the
  three registers to match.
- `gui/tokens/README.md` — the file being edited. Its "Radius: calc preserved" subsection is the
  model for the dual-carry documentation.
- `gui/tokens/dist/tokens.css` — the emitted bundle. The authority for what is actually documented.

## Checkpoint hints

- After the `CONTRACT.md` role enumeration and resolution-expression subsection.
- After the `CONTRACT.md` Tailwind container integration subsection and the two behavior-change
  statements.
- After the `README.md` source/tiering updates.

## Status

**Outcome:** succeeded — 2026-08-06.

`bun install` and `bun run build:tokens` were run in `gui/` (dependencies were not yet installed in
this worktree) to regenerate `gui/tokens/dist/tokens.css` as the source of truth, per the task's
own instruction; `dist/` remains gitignored and produced no tracked changes.

`gui/tokens/CONTRACT.md` changes:
- New `### Spacing and container width` subsection under "The fixed `--mf-*` surface", with
  `#### Resolution expression`, `#### Band-span semantics`, `#### Settable vs. internal — a
  divergence from radius`, `#### Fluid values are expressible`, and `#### Known limitation
  (accepted, still open)` sub-subsections. All 15 settable role names are spelled out literally
  (not only via `{...}` brace notation) so they are individually greppable.
- New `### Tailwind container integration` subsection under "Where the contract is enforced",
  covering `@variant <band>`, `--container-content` / `max-w-content`, the "extends not replaces"
  behavior change (built-in max-width ladder overridden), and the `padding-block` addition —
  each as its own bullet.
- A short divergence note added directly beneath the `### Settable vs. internal` table, cross-
  referencing the new section (per-band levers are settable, unlike radius's derived steps).
- The "Only color roles are re-emitted in scoped selectors" sentence under "Emission shape" extended
  to name the new mode-independent spacing/container-width roles.
- The stray trailing `</content>` artifact line removed from the end of the file.

`gui/tokens/README.md` changes:
- `base/spacing.json` and `semantic/layout.json` added to the File layout tree (one-line
  descriptions matching the existing entries' style).
- The Tiering convention table's `semantic/` row extended to note `layout.json` is also
  mode-independent, alongside `radius.json`.
- A `**Spacing and container width:**` enumeration line added beside the existing `**Radius:**` /
  `**Typography families:**` / `**Type scale:**` lines (three scalar roles, twelve per-band roles,
  base values).
- A `**Spacing: dual-carry preserved.**` paragraph documenting the `$value` +
  `$extensions["com.moduleforge.breakpoint"]` dual-carry pattern, cross-referencing `### Radius:
  calc preserved`.
- A `**Spacing: multiplier ladder.**` table plus notes on the `px-4 sm:px-6 lg:px-8` idiom parallel
  and on every band being declared even where its value repeats the band below.

**Validation:** all 10 checks in `## Validation` passed — see the structured report for command-level
detail. In particular, every one of the 15 settable names and their `-default` twins were confirmed
present in the regenerated `gui/tokens/dist/tokens.css` (check 7), and `git status` shows exactly
`gui/tokens/CONTRACT.md` and `gui/tokens/README.md` modified (check 10).

**Assumptions applied:** both `## Assumptions` bullets held — Phase 1 had already landed on the plan
branch, `dist/tokens.css` regenerated cleanly, and `dist/` produced no git changes.

**Files touched (repo-relative):**
- `gui/tokens/CONTRACT.md`
- `gui/tokens/README.md`
- `plan/phase-02-contract-documentation/001-document-spacing-tokens-in-contract.md` (this file)
