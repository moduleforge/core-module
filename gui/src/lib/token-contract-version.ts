/**
 * The single authoritative version of the ModuleForge **token contract** —
 * the `--mf-*` surface + `data-mf-theme` scoping mechanism defined across
 * `../../tokens/CONTRACT.md` — as distinct from `@moduleforge/core-gui`'s own
 * npm package version (which may rev for reasons unrelated to the token
 * surface).
 *
 * This is the "authoritative constant" `../../tokens/STYLE-PACKAGE-CONTRACT.md`
 * ("Token-contract versioning") requires mod-core to expose so the runtime
 * loader (`./theme-loader.ts`) can compare it against a style package's
 * declared `targetContractVersion` range. Bump it only when the contract
 * itself changes, following the MAJOR/MINOR/PATCH semantics that document
 * defines:
 *
 * - **MAJOR** — a `--mf-x` role removed/renamed, or a `data-mf-theme`
 *   value/selector changed.
 * - **MINOR** — new `--mf-x` role(s) added; existing roles unchanged.
 * - **PATCH** — a baked `--mf-x-default` value changed, or a build fix; no
 *   surface change.
 *
 * `1.0.0` is the Phase 1–2 surface: the 35 color roles, `--mf-radius`, the
 * typography families and type scale, and the `data-mf-theme`
 * light/dark/inverse scoping.
 * `1.1.0` adds the spacing/container roles: `--mf-max-content-width`,
 * `--mf-content-margins-lr` / `--mf-content-margins-tb`, and their
 * per-breakpoint-band override forms `--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}`.
 * This is a MINOR bump: new roles only, no existing role removed, renamed, or revalued.
 */
export const MF_TOKEN_CONTRACT_VERSION = '1.1.0';
