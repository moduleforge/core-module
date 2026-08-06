# Document Per Band Override For Style Packages

## Purpose and scope

Document the new **per-breakpoint-band override** capability in
`gui/tokens/STYLE-PACKAGE-CONTRACT.md` — the style-package *supply*-side contract.

The new spacing tokens introduce the first genuine capability change to that contract since it was
written. Every token before now has been a single scalar per mode/scope; content margins add a second
axis (breakpoint band) on top of the existing style-package-override axis, and — unlike radius —
a *derived step's* bare `--mf-x` form is deliberately settable. A style-package author who has
internalized the current document will get this wrong unless it is spelled out.

Only `gui/tokens/STYLE-PACKAGE-CONTRACT.md` is edited. This task is parallel-eligible with Phase 2
task `001`, whose file set is disjoint.

**Describe what actually shipped.** Read the emitted `gui/tokens/dist/tokens.css` (regenerate with
`cd gui && bun install && bun run build:tokens` if absent — the directory is gitignored) before
writing. Where reality disagrees with this task document, reality wins; report the discrepancy.

## Requirements

Work with the grain of the existing document's four organizing ideas — the three-part artifact,
contract versioning, the `gui.peers`-analog dependency declaration, and the security posture — rather
than bolting on a new top-level section.

1. **Extend "1. Compiled `--mf-*` override bundle" with the new lever family.** Its "Hard rules" list
   currently says a package "sets `--mf-x` only, never `--mf-x-default`", which remains true and
   unchanged. Add the per-band shape underneath:

   - `--mf-content-margins-lr` / `--mf-content-margins-tb` are the **base-scale levers**. Setting
     either rescales every band through that band's compiler-baked multiplier — the same relationship
     `--mf-radius` has to the derived radius steps.
   - `--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}` are the **per-band levers**. Setting one
     replaces the derived value for that band alone; the base lever still governs every other band.
   - `--mf-max-content-width` is an ordinary single scalar with no second axis.

2. **State the divergence from the radius rule explicitly.** The document currently instructs, under
   "Emission rules": radius means `--mf-radius` — "the *single* settable radius lever; never the
   derived `--mf-radius-{sm,md,lg,xl}` steps, per CONTRACT.md". A reader will generalize that to
   content margins and be wrong. Say plainly: for content margins the derived per-band forms **are**
   settable, and that is the intended escape hatch. Do not leave this to inference.

3. **Document band-span semantics** — a per-band override governs its band's span, from that
   breakpoint up to the next declared band, not every width above it. Give the worked example:
   setting `--mf-content-margins-lr-sm` alone changes the gutter between `40rem` and `48rem` and
   nowhere else. A brand wanting a change to persist across several bands sets each of them, or sets
   the base lever.

4. **Extend "Emission rules — what the override bundle must look like".** Its "Mode-independent
   overrides live once in `:root`" bullet currently names radius, font families, and type-scale
   sub-tokens. Add the spacing family: `--mf-max-content-width`, both base levers, and every per-band
   lever are mode-independent and are **not** re-emitted in the `data-mf-theme` scope selectors.
   Update the illustrative `dist/style.css` fragment to show at least one base-lever and one per-band
   override in the `:root` block, so the shape is visible rather than only described.

5. **Extend the "Token-contract versioning" section for `1.1.0`.** The section fixes `1.0.0` as the
   Phase 1–2 surface. Record that adding these roles is a **MINOR** bump to `1.1.0` under the
   existing table's semantics — new roles only, nothing removed, renamed, or revalued — and that the
   authoritative constant is `MF_TOKEN_CONTRACT_VERSION` in
   `gui/src/lib/token-contract-version.ts`. Keep this consistent with the constant's own doc comment,
   which Phase 1 task `003` updated; if the two disagree, halt and report.

   Confirm the existing MAJOR/MINOR/PATCH table still reads correctly for the new tokens: an older
   package that sets none of them renders with mod-core's defaults; a newer package that sets them
   against an older core is inert. Both hold; say so if the table needs a clarifying word, otherwise
   leave it untouched.

6. **Extend the security posture for the new type.** The "How the typed `@property` declarations
   constrain override values" paragraph enumerates the token types a future validator would check
   against (`--mf-primary` a `<color>`, `--mf-radius` a `<length>`, a weight a `<number>`). Add that
   every spacing token — the width, both base levers, and all twelve per-band levers — is registered
   `<length>`, so the per-band levers are type-constrained on exactly the same footing as every other
   token. **The new axis adds surface, not a new trust boundary**: a per-band lever is still a single
   typed value in a fixed, closed, enumerated set of names, so the structural argument ("a package is
   a values manifest by construction") is unweakened. Say this in as many words — a reviewer will
   reasonably ask whether a second axis widens the attack surface, and the answer is no.

   Also note that `<length>` admits `clamp()` with a `vw` term, so a brand may supply a fluid value
   for a band. That is a feature and remains inside the typed contract, not an escape from it.

7. **Update the style-package build convention's directory layout** — its `tokens/overrides/` tree
   lists `color.light.json`, `color.dark.json`, `radius.json`, `typography.json`. Add an optional
   `layout.json` for sparse `--mf-max-content-width` / `--mf-content-margins-*` overrides
   (mode-independent), matching how `radius.json` and `typography.json` are annotated.

8. **Remove the stray trailing `</content>` and `</invoke>` lines** at the end of the file. They are
   committed agent-tooling artifacts, not content.

### Do not

- Do not edit `gui/tokens/CONTRACT.md` or `gui/tokens/README.md` — Phase 2 task `001` owns them.
- Do not restate the *consumption*-side contract here; this document supplies against `CONTRACT.md`
  and links to it.
- Do not edit any file under `gui/tokens/*.json`, `build-tokens.mjs`, or `gui/src/`.
- Do not edit anything under `docs/mf-standards/` — submodule-mounted, owned by the sibling project.
- Do not change the manifest JSON Schema. Nothing about these tokens touches `style-package.json`'s
  shape; they are ordinary entries in the CSS override bundle.

## Validation

1. All 15 new settable role names appear in `gui/tokens/STYLE-PACKAGE-CONTRACT.md`, and each is
   identified as settable.
2. The divergence from the radius "never the derived steps" rule is stated explicitly, in its own
   sentence, and is discoverable from the "Emission rules" section a package author actually reads.
3. Band-span semantics are documented with the worked `sm`-band example.
4. The illustrative `dist/style.css` fragment includes at least one base-lever and one per-band
   override, both in `:root`, and no spacing token in a `data-mf-theme` scope selector.
5. The versioning section names `1.1.0` and MINOR, and agrees with
   `gui/src/lib/token-contract-version.ts`'s doc comment — cross-read both.
6. The security-posture section names `<length>` for the spacing tokens and states that the second
   axis adds surface but not a new trust boundary.
7. The build-convention directory layout lists `layout.json`.
8. `grep -c '</content>\|</invoke>' gui/tokens/STYLE-PACKAGE-CONTRACT.md` returns 0, and
   `tail -c 200` shows the file ending in prose.
9. Every relative link in the document still resolves to an existing file — in particular the
   `./CONTRACT.md` anchors and the `../../docs/mf-standards/…` links.
10. **Cross-check names against the emitted bundle**: every settable name documented here appears in
    `gui/tokens/dist/tokens.css`. Any mismatch is a defect — halt and report.
11. Markdown renders cleanly; tables well-formed, code blocks language-tagged.
12. `git status` shows exactly one modified file.

## Metadata

architectural_impact: true

## Assumptions

- All of Phase 1 has landed, including task `003`'s `MF_TOKEN_CONTRACT_VERSION = '1.1.0'` bump, so
  the versioning section can be written against a fact rather than an intention.
- No style package exists in-repo that would need rebuilding; `style-liquid-labs/` is an independent
  sibling repository and is out of scope. Adding these roles cannot strand it — an omitted token
  degrades to mod-core's default by construction.

## References

- [`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md) — the design record: the
  precedence semantics, the full lever surface, and why a derived step is settable here where it is
  not for radius.
- [`plan/notes/tailwind-container-mechanics.md`](../notes/tailwind-container-mechanics.md) — the
  measured band-span behavior (last-matching-declaration-wins) this document must describe correctly.
- `gui/tokens/STYLE-PACKAGE-CONTRACT.md` — the file being edited. Its "Hard rules", "Emission rules",
  "Token-contract versioning", and "Security posture" sections are the four insertion points.
- `gui/tokens/CONTRACT.md` — the consumption-side contract this document supplies against. Read Phase
  2 task `001`'s output if it has already landed, to keep the two consistent and avoid duplicating.
- `gui/src/lib/token-contract-version.ts` — the authoritative version constant and its doc comment.

## Checkpoint hints

- After the override-bundle lever documentation and the radius-divergence statement.
- After the emission-rules and illustrative-fragment updates.
- After the versioning and security-posture updates.
