# Plan Summary: api-client-origin-guard

## What was planned and why

`gui/` (`@moduleforge/core-gui`) is a published/reusable GUI component library consumed by other ModuleForge modules over yalc, not an application. Its `request()` function attaches the configured bearer auth token to whatever URL string or `URL` object a caller passes to it, with no validation that the target is an expected/allowed host — a security-relevant gap flagged by a prior Phase-02 security review and tracked as followup `7VD3`.

This plan resolved `7VD3` by adding a **real runtime origin-check guard** to `request()`/`configureApiClient()` in `gui/src/lib/api-client.ts`, rather than the doc-comment-only alternative the followup also offered — the user explicitly chose the enforcement route. The guard was designed to be opt-in and backward-compatible: a consuming app can constrain which origins may receive the token via a new `allowedOrigins: string[]` config field, while every existing/dogfooded caller that doesn't configure it sees zero behavior change. Two load-bearing design questions (guard config shape, and mismatch behavior) were routed to the user rather than decided unilaterally, since they determine real runtime security/availability tradeoffs for every future consumer of `request()`.

## What shipped

### Phase 1 — `gui-origin-guard` (GUI API Client Origin Guard)

**Task 001 — Implement Origin Guard** (`phase-01-gui-origin-guard/001-implement-origin-guard.md`, tier `sonnet-high`)
Branch `phase-01-task-01-implement-origin-guard`, commit `36c3c5d`, merged as `e8b944e9`.

- Added an opt-in `allowedOrigins: string[]` allow-list guard to `gui/src/lib/api-client.ts`, resolving followup `7VD3`.
- `configureApiClient()` now accepts both the existing auth-handler fields and the new `allowedOrigins` field, merging each independently (additive/partial merge).
- New private `resolveRequestOrigin()` helper resolves the target origin from `string | URL` input — browser-relative-safe, SSR-safe, and never throws.
- `request()` enforces the guard as its first statement, throwing `ApiRequestError('origin_not_allowed', ..., 0)` before any network call when the resolved origin is disallowed or unresolvable (fail closed).
- Unconfigured default behavior is a complete no-op, verified byte-for-byte against pre-change control flow.
- Root `AGENTS.md`'s "### gui/ error and toast toolkit" section updated to document the new option.
- Validated via `gui`'s `bun run typecheck` (`tsc --noEmit`, zero errors) plus explicit manual reasoning/trace-through, since `gui/` has no unit-test runner (followup `vr20`, out of scope for this plan).

### Fix-forward: phase-review follow-on

After the task merged, a phase-review (combined correctness + efficiency + baseline-security pass) surfaced one suggestion-level finding: `gui/src/lib/api-types.ts`'s `ApiError.code` JSDoc did not list the new `origin_not_allowed` reserved code introduced by the guard. This was fixed via a follow-on `dispatch-simple-task`:

- Branch `2026-07-17-document-origin-not-allowed-co`, commit `99216ed` ("docs(gui): document origin_not_allowed as reserved ApiError code"), merged to `main` as `373a4a6f`.
- Single-line change: added `origin_not_allowed` to the JSDoc enumeration of reserved `ApiError.code` values in `gui/src/lib/api-types.ts`.

## Key decisions

- **Guard config shape — `allowedOrigins: string[]` allow-list, not `baseUrl: string`.** Chosen by the user; recorded in `plan/notes/guard-shape-decision.md`. An allow-list supports multi-origin consumers directly rather than forcing a single base URL.
- **Mismatch behavior — fail closed (throw).** The guard rejects the request before it is made (`ApiRequestError('origin_not_allowed', ..., 0)`) rather than silently stripping the token and letting the request proceed. Chosen by the user; recorded in `plan/notes/guard-mismatch-behavior-decision.md`.
- **Additive/independent merge semantics.** `allowedOrigins` merges into `configureApiClient()`'s config independently of the existing `Partial<ApiClientAuthHandler>` merge, so consumers who never touch the new field get byte-for-byte identical behavior to before the guard existed — a deliberate backward-compatibility guarantee, not just an implementation detail.
- **No test runner added.** `gui/` has no `vitest`/`jest` (followup `vr20`); this plan's task explicitly did not add one, relying instead on `tsc --noEmit` plus documented manual verification. Adding real unit-test coverage for the guard is treated as a dependency on resolving `vr20` first, not something to take on silently within this plan.
- **Scope boundary held on token storage.** Changing default token storage away from `localStorage` (followup `pBoG`) was explicitly out of scope and left untouched.
- **No `docs/architecture.md` / `docs/*-spec.md` updates.** Neither file exists in this project; this was treated as a library-internal change documented via `AGENTS.md` and JSDoc rather than an architectural one (confirmed against the `docs/mf-standards` submodule during research).

## Follow-up items

Carried forward in `plan/followups.yaml` (followup `7VD3`, this plan's own subject, was already removed at task close-out):

- **`vr20` — `gui/` package has no test runner configured.** Pre-existing gap, not introduced by this plan. The React Developer role doc calls for component tests as a core responsibility, but sibling tasks (and this plan's task) have shipped without them since no task doc's Validation section required them. Decide whether/when to add a test runner to `gui/`.
- **`pBoG` — Default token storage uses `localStorage`.** Low-confidence/suggestion-level finding from an earlier security review: the default `ApiClientAuthHandler` (`gui/src/lib/api-client.ts`) stores/reads the bearer token via `window.localStorage`, which is XSS-exfiltrable. Intentionally mirrors existing `mod-users` behavior, not a new regression; `configureApiClient` already lets consumers substitute a safer strategy. No action needed now — revisit if/when the project moves toward httpOnly-cookie or in-memory-token storage.
- **`womX` — Service validation-reason text lost on 400s.** Unrelated to this plan (tagged `phase-01-apiresp-go`, a Go API-layer finding); listed here only because it remains open in the shared `plan/followups.yaml`. Adopting `apiresp.WriteError` drops specific validation-reason text that older `writeServiceErr`-based errors used to surface. Decide: accept as a documented tradeoff, or route the relevant service-layer validation errors through `apiresp.InvalidInput(FieldError{...})` so the reason survives in `details[]`.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — GUI API Client Origin Guard

- [x] [001-implement-origin-guard.md](./phase-01-gui-origin-guard/001-implement-origin-guard.md) — tier `sonnet-high` · branch `phase-01-task-01-implement-origin-guard` · commit `36c3c5d` · merge `e8b944e974ffd8c0a636fca829adb322ca37a6d0`
