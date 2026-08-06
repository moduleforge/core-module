# Add Spacing Token Sources

## Purpose and scope

Author the two new DTCG token source files that define the spacing / container-width token
category — the raw tier and the semantic tier — following the existing tiering convention in
`gui/tokens/README.md`. **Token definitions only.** This task emits no CSS, changes no compiler
code, and changes no consumer wiring; the compiler work is task `002`.

This mirrors how the token system was originally built (its task `001` authored sources, its task
`002` built the compiler), and keeps the token-source review separable from the emission review.

Files created:

- `gui/tokens/base/spacing.json`
- `gui/tokens/semantic/layout.json`

No standard skill covers this; follow the `## Procedure` below.

## Requirements

### `gui/tokens/base/spacing.json` — raw tier

Two literal dimension tokens, plus the file-level `$description` every source file in this directory
carries:

| Path | `$type` | `$value` | Meaning |
|------|---------|----------|---------|
| `spacing.content-margin-base` | `dimension` | `1rem` | The base input both content-margin ladders derive from. |
| `spacing.max-content-width` | `dimension` | `80rem` | Max width page content grows to. Equals Tailwind's `--container-7xl` and its `xl` breakpoint. |

Per the raw-tier convention these are literal values only and are **never referenced directly by a
component** — only aliased from the semantic tier.

### `gui/tokens/semantic/layout.json` — semantic tier

All tokens live under the `mf.` path prefix (the compiler's `isSemanticToken` filter keys on
`path[0] === 'mf'`) and are `$type: dimension`, which the compiler's existing `SYNTAX_BY_TYPE` map
already routes to `@property syntax: "<length>"`.

**Three scalar roles**, each a DTCG alias into the raw tier:

| Path | `$value` |
|------|----------|
| `mf.max-content-width` | `{spacing.max-content-width}` |
| `mf.content-margins-lr` | `{spacing.content-margin-base}` |
| `mf.content-margins-tb` | `{spacing.content-margin-base}` |

**Twelve derived per-band roles** — `mf.content-margins-lr-<band>` and `mf.content-margins-tb-<band>`
for `<band>` ∈ {`base`, `sm`, `md`, `lg`, `xl`, `2xl`}. Each carries **both** its pre-computed
literal `$value` **and** its band + multiplier under
`$extensions["com.moduleforge.breakpoint"]`, exactly the dual-carry pattern
`gui/tokens/semantic/radius.json` already uses for `com.moduleforge.radius`:

```json
"content-margins-lr-sm": {
  "$type": "dimension",
  "$value": "1.5rem",
  "$description": "Inline content margin at the sm band (>= --breakpoint-sm). Derived: content-margin-base * 1.5.",
  "$extensions": {
    "com.moduleforge.breakpoint": {
      "band": "sm",
      "base": "{spacing.content-margin-base}",
      "multiplier": 1.5
    }
  }
}
```

The literal `$value` exists so the compiler's baked `@property` `initial-value` is exact; the
multiplier exists so the compiler can emit `calc(var(--mf-content-margins-lr, …) * k)` and a runtime
override of the base cascades to every band. Both must be present on every derived token, and the
literal must equal `1rem × multiplier` — task `002` will assert this.

Values and multipliers, base input `1rem` on both axes:

| Band | `lr` multiplier | `lr` `$value` | `tb` multiplier | `tb` `$value` |
|------|-----------------|---------------|-----------------|---------------|
| `base` | `1` | `1rem` | `1.5` | `1.5rem` |
| `sm` | `1.5` | `1.5rem` | `1.5` | `1.5rem` |
| `md` | `1.5` | `1.5rem` | `1.5` | `1.5rem` |
| `lg` | `2` | `2rem` | `2` | `2rem` |
| `xl` | `2` | `2rem` | `2` | `2rem` |
| `2xl` | `2` | `2rem` | `2` | `2rem` |

`base` names the unprefixed, below-`sm` band — the Tailwind-idiomatic name for the mobile-first
default. The inline ladder deliberately reproduces the ubiquitous `px-4 sm:px-6 lg:px-8` app-shell
idiom, so the default render is unsurprising. Every band is declared even where its value repeats the
band below it: a uniform per-band override surface is worth more than three saved declarations.

### Naming constraints that must be respected

- **No collision with an existing token path.** `mf.text-body` already collides between the colour
  and typography tiers and forced the compiler into three separate Style Dictionary instances
  (see the compiler's header comment). Do not create a second such collision: verify that none of
  `max-content-width`, `content-margins-lr`, `content-margins-tb`, or any `-<band>` suffix form
  already exists under `mf.` in any file under `gui/tokens/`.
- **No token may be named such that appending `-default` collides with another token's name.** The
  compiler emits `--mf-<name>-default` for every token; `--mf-content-margins-lr-sm-default` must be
  unambiguous. The chosen names satisfy this.

### Do not

- Do not edit `build-tokens.mjs` — that is task `002`.
- Do not edit `gui/tokens/CONTRACT.md`, `README.md`, or `STYLE-PACKAGE-CONTRACT.md` — that is
  Phase 2. (Adding a source file without immediately documenting it is intentional here; the docs
  describe the *emitted* shape, which does not exist until task `002` runs.)
- Do not add anything to `gui/tokens/component/overrides.json`.
- Do not commit anything under `gui/tokens/dist/` — that directory is gitignored.

## Validation

1. Both files exist and are valid JSON: `node -e "JSON.parse(require('fs').readFileSync('gui/tokens/base/spacing.json'))"` and the same for `gui/tokens/semantic/layout.json`.
2. Every token in `semantic/layout.json` sits under the `mf.` root and carries `$type: "dimension"`.
3. Every one of the twelve derived tokens carries a `$extensions["com.moduleforge.breakpoint"]`
   object with `band`, `base`, and a numeric `multiplier`.
4. For each derived token, the literal `$value` equals `1rem × multiplier` — check all twelve by
   hand against the table above.
5. Both files carry a top-level `$description`, matching every other file in `gui/tokens/`.
6. No path collision: `grep -rn 'max-content-width\|content-margins' gui/tokens/` returns hits only
   in the two new files.
7. `git status` shows exactly two new files and no modifications to any existing file.

Compiler-level validation (that the sources resolve, that references bind, that the emitted CSS is
correct) belongs to task `002` and is deliberately not attempted here — the compiler does not read
these files until `002` wires them in, so a build run now proves nothing either way.

## Metadata

architectural_impact: true

## Assumptions

- The DTCG alias form `{spacing.content-margin-base}` resolves the same way `{radius.base}` already
  does in `semantic/radius.json` — Style Dictionary deep-merges all files in one instance's `source`
  array before resolving references, and task `002` will add both new files to the same instance
  that already resolves `base/radius.json` + `semantic/radius.json`.
- `$type: "dimension"` is the correct DTCG type for all of these, matching how radius is typed.

## References

- [`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md) — the full design record:
  the alternatives weighed, the resolution expression, and the reasoning behind these exact values.
- `gui/tokens/semantic/radius.json` — the dual-carry (literal `$value` + `$extensions` multiplier)
  pattern this task copies. Read it before authoring.
- `gui/tokens/base/radius.json` — the raw-tier shape `base/spacing.json` mirrors.
- `gui/tokens/README.md` — the tiering convention and file-layout table these files must fit.
- `gui/style-dictionary/build-tokens.mjs` — read the header comment for the `mf.text-body` collision
  story that motivates the naming constraints above. Do not edit it in this task.

## Status

Implementation outcome: **succeeded**. Date: 2026-08-06.

Created `gui/tokens/base/spacing.json` (raw tier: `spacing.content-margin-base` = `1rem`,
`spacing.max-content-width` = `80rem`) and `gui/tokens/semantic/layout.json` (semantic tier: the
three scalar `mf.*` roles plus the twelve `mf.content-margins-{lr,tb}-<band>` derived tokens, each
dual-carrying its literal `$value` and `$extensions["com.moduleforge.breakpoint"]` per the
`semantic/radius.json` pattern). No other files touched.

Validation summary — all seven checks passed:
1. Both files parse as valid JSON (`node -e "JSON.parse(...)"`).
2. Verified programmatically: every token under `mf.*` in `semantic/layout.json` is `$type: "dimension"`.
3. Verified programmatically: all twelve derived tokens carry `$extensions["com.moduleforge.breakpoint"]`
   with `band`, `base`, and a numeric `multiplier`.
4. Verified programmatically: for each derived token, literal `$value` equals `1rem × multiplier`
   (all twelve pass).
5. Both files carry a top-level `$description`.
6. `grep -rn 'max-content-width\|content-margins' gui/tokens/` returns hits only in the two new files
   — no path collision with any existing token.
7. `git status --porcelain` shows exactly two new, untracked files and no modifications to any
   existing file.

Assumptions applied (both from `## Assumptions` above, taken as given per task scope): the DTCG
alias form resolves the same way `{radius.base}` already does in `semantic/radius.json`, and
`$type: "dimension"` is the correct DTCG type — both are compiler-level (task `002`) concerns not
re-verified here, consistent with the task's stated boundary.

Affected source files:
- `gui/tokens/base/spacing.json`
- `gui/tokens/semantic/layout.json`
