# Bump Token Contract Version

## Purpose and scope

Bump `MF_TOKEN_CONTRACT_VERSION` from `1.0.0` to `1.1.0` in
`gui/src/lib/token-contract-version.ts`, and extend that file's doc comment to describe what the new
version covers.

Adding the spacing / container-width roles is a **MINOR** change under the semantics
`gui/tokens/STYLE-PACKAGE-CONTRACT.md` defines and the file's own doc comment restates: new `--mf-x`
roles added, no existing role removed, renamed, or revalued, and no `data-mf-theme` value or selector
changed.

Only `gui/src/lib/token-contract-version.ts` is edited. This task is independent of tasks `001` and
`002` and can run in parallel with them.

## Requirements

1. Change the exported constant to `1.1.0`:

   ```ts
   export const MF_TOKEN_CONTRACT_VERSION = '1.1.0';
   ```

2. Update the doc comment's closing paragraph. It currently reads:

   > `1.0.0` is the Phase 1–2 surface: the 35 color roles, `--mf-radius`, the typography families
   > and type scale, and the `data-mf-theme` light/dark/inverse scoping.

   Keep that sentence as the description of `1.0.0` and add a sentence for `1.1.0`, naming the added
   surface: `--mf-max-content-width`, `--mf-content-margins-lr` / `--mf-content-margins-tb`, and
   their per-breakpoint-band override forms
   `--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}`.

3. State explicitly *why* this is MINOR rather than MAJOR — new roles only, nothing removed,
   renamed, or revalued — so the next person bumping it has the worked precedent.

4. Leave the MAJOR / MINOR / PATCH semantics list in the comment unchanged.

### Do not

- Do not touch `gui/package.json`'s `version` field. The token-contract version is deliberately
  distinct from the npm package version; conflating them is exactly what this constant exists to
  prevent.
- Do not edit `gui/tokens/STYLE-PACKAGE-CONTRACT.md` — Phase 2 task `002` owns the contract-doc side
  of the version story.
- Do not edit `gui/src/lib/theme-loader.ts`, `semver-range.ts`, or any test. The loader compares
  whatever the constant holds; no code change follows from a value bump.

## Validation

1. `grep -n "MF_TOKEN_CONTRACT_VERSION" gui/src/lib/token-contract-version.ts` shows `'1.1.0'`.
2. `cd gui && bun run typecheck` passes.
3. `cd gui && bun run test` passes. In particular `gui/src/lib/theme-loader.test.ts` imports the
   constant and interpolates it into `targetContractVersion: \`^${MF_TOKEN_CONTRACT_VERSION}\`` — it
   is written against the constant rather than a literal, so it should pass unchanged. If any test
   hardcodes `1.0.0`, halt and report rather than editing the test to match.
4. `grep -rn "1\.0\.0" gui/src/` surfaces no remaining stale reference to the contract version
   (matches on unrelated versions or on the doc comment's description *of* `1.0.0` are expected and
   correct).
5. `git status` shows exactly one modified file.

## Assumptions

- `gui/node_modules` may need a `bun install` before `typecheck` / `test` run; the checkout's
  `tailwindcss` symlinks are known broken. If `bun install` fails, halt and report.
- No published style package currently declares a `targetContractVersion` narrower than `^1.0.0`, so
  the bump cannot strand an existing artifact — `^1.0.0` admits `1.1.0`.

## References

- `gui/src/lib/token-contract-version.ts` — the file being edited; its doc comment already states
  the bump semantics to apply.
- `gui/tokens/STYLE-PACKAGE-CONTRACT.md`, "Token-contract versioning" — the authoritative
  MAJOR/MINOR/PATCH table and the mismatch policy.
- [`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md) — the full enumeration of
  the roles being added, for the doc-comment sentence.
