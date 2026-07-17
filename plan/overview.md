# API Client Origin Guard

## Purpose and scope

Resolve `plan/followups.yaml` item `7VD3` ("`request()` has no origin check for token", Phase-02 security review) by adding a **real runtime origin-check guard** to `request()`/`configureApiClient()` in `gui/src/lib/api-client.ts` — the user has explicitly chosen this over the doc-comment-only alternative the followup also offered.

`gui/` (`@moduleforge/core-gui`) is a published/reusable GUI component library consumed by other ModuleForge modules over yalc, not an application. `request()`'s bearer-token attachment path is a security-relevant boundary: today it attaches the configured auth token to whatever URL string or `URL` object the caller passes to `request()`, with no validation that the target is an expected/allowed host. This plan adds an opt-in guard so a consuming app can constrain which origins are allowed to receive the token, while leaving today's behavior (and every existing/dogfooded caller) unaffected when the guard is not configured.

**In scope:**
- A new, backward-compatible guard configuration surface on `configureApiClient()`: `allowedOrigins: string[]` (resolved shape — see [Current status](#current-status)).
- Enforcement logic inside `request()` that validates the resolved target URL's origin against the configured guard before attaching the `Authorization` header, throwing (fail closed) on mismatch — reject the request before it is made (resolved behavior — see [Current status](#current-status)).
- Documentation of the new guard in root `AGENTS.md`'s "### gui/ error and toast toolkit" section (the section that currently documents `configureApiClient`'s existing `{ getToken, onUnauthenticated }` seam).
- Removing `7VD3` from `plan/followups.yaml` once the guard lands (per the plan-documents handling protocol, follow-up resolution is a manager/`apply-task-report` concern at task-close, not something this plan's task itself edits directly — noted here for continuity).

**Out of scope:**
- Adding a test runner to `gui/` (tracked separately as followup `vr20`; do not fix as part of this plan — see [Constraints](#constraints-for-the-manager) below).
- Changing default token storage (`localStorage`) — that is followup `pBoG`, a separate, lower-confidence, no-action-needed item.
- Any change to how `request()` constructs/resolves URLs beyond what the `allowedOrigins` guard strictly requires (i.e., this is a validation guard on the token-attachment path, not a URL-building helper).
- `docs/architecture.md` / `docs/*-spec.md` updates — neither file exists in this project; this is a library-internal change, not an architectural one. (Confirmed no relevant convention exists in the `docs/mf-standards` submodule either — see [research notes](./notes/api-client-origin-guard-context.md).)

## Current status

**Decomposed and ready for implementation.** Both open design questions have been answered by the user:

1. **Guard config shape** — `allowedOrigins: string[]` allow-list (not `baseUrl: string`). See [`plan/notes/guard-shape-decision.md`](./notes/guard-shape-decision.md).
2. **Mismatch behavior** — throw (fail closed): reject the request before it is made. See [`plan/notes/guard-mismatch-behavior-decision.md`](./notes/guard-mismatch-behavior-decision.md).

The plan is a single phase, `gui-origin-guard` (phase 1), with a single task registered in `plan/TODO.yaml`. No research delegation was needed and no further design decisions are outstanding.

## Overview

**Phase 1 — `gui-origin-guard` (GUI API Client Origin Guard).** One task:

1. **`phase-01-gui-origin-guard/001-implement-origin-guard.md` — Implement Origin Guard.** Adds `ApiClientOriginGuardConfig` (`allowedOrigins?: string[]`) as a new opt-in field mergeable via `configureApiClient()` (additive/partial, independent of the existing `Partial<ApiClientAuthHandler>` merge so unconfigured consumers see zero behavior change), enforces it inside `request()` — throwing `ApiRequestError('origin_not_allowed', ..., 0)` before any network call when the resolved target origin isn't in `allowedOrigins` (or can't be resolved) — updates the `request()`/`configureApiClient()` JSDoc, and updates root `AGENTS.md`'s "### gui/ error and toast toolkit" section to document the new option. Full design (resolution algorithm, merge semantics, error shape, branch-by-branch manual verification checklist) is specified in the task document. Validated via `gui`'s `bun run typecheck` (the only automated check available — see constraint below) plus the explicit manual reasoning/trace-through the task document requires, since `gui/` has no unit-test runner.

No parallelism applies — this is the plan's only task. No `doc-updates` phase was added: this is a library-internal change (no `docs/architecture.md` or `docs/*-spec.md` exists in this project), and the root `AGENTS.md` touch-up is folded into the implementation task's own requirements rather than a separate phase.

## Constraints for the manager

- **No test runner in `gui/`** (followup `vr20`, confirmed via `gui/package.json` — no `vitest`/`jest`, only `tsc --noEmit`/`tsup`/`ladle`). This plan's task(s) must not add one. Validation for the guard will rely on typecheck plus manual/documented reasoning rather than automated unit tests. If the manager or user later decides real unit-test coverage is a hard requirement before landing this change, that is a **dependency on resolving `vr20` first**, not something this plan's task should silently take on.
- This is a security-relevant change to a published library's token-attachment path (`@moduleforge/core-gui`, consumed by other modules over yalc). The two design decisions recorded under [Current status](#current-status) above were not stylistic — they determine actual runtime security/availability tradeoffs for every future consumer of `request()`, so they were routed to the user rather than decided unilaterally.
