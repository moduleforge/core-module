# Origin-Check Guard — Research Context

## Followup being resolved

`plan/followups.yaml` item `7VD3` (Phase-02 security review, `security-001`, suggestion/medium-confidence), recorded 2026-07-15, tag `phase-02-gui-error-widgets`:

> `request()` (`gui/src/lib/api-client.ts` ~106-119) attaches the bearer token to whatever URL the caller passes, with no same-origin/allow-listed-host check. Not exploitable from anything in this diff (no consumer wired yet) — risk depends entirely on how a future consumer calls `request()`. Consider an optional allowedOrigins/base-URL guard in configureApiClient or request(), or document the implicit same-origin assumption in the doc comment.

The user has explicitly chosen the runtime-guard branch of that suggestion over the doc-comment-only alternative.

## Current implementation (`gui/src/lib/api-client.ts`)

- `request<T>(input: string | URL, options: RequestOptions = {})` (lines 106-157): resolves the bearer token via `authHandler.getToken()`, unconditionally attaches `Authorization: Bearer <token>` when a token is present, then calls `fetch(input, ...)`. No validation of `input`'s origin exists today.
- `ApiClientAuthHandler` (lines 43-48) is the one existing configuration seam: `{ getToken, onUnauthenticated }`. It is module-singleton state (`let authHandler`), overridden via `configureApiClient(handler: Partial<ApiClientAuthHandler>)` (lines 62-72), which merges partial overrides onto the current handler. The doc comment above the seam (lines 30-41) explicitly states the design rationale: `mod-core/gui` is a component *library*, not an app, so anything environment-specific (storage, routing, and by the same logic, allowed API origins) must be injectable rather than hard-coded, and the browser-only default must SSR-safely no-op.
- `configureApiClient` is additive/partial — omitted fields keep their current implementation. This is the established backward-compatibility pattern any new guard config should follow.
- No `baseUrl` concept exists anywhere in the file or its callers today; `request()` takes the full URL/path as given by the caller and passes it straight to `fetch`.

## Consumers today

- `gui/src/lib/use-api-error.ts` imports only the `ApiRequestError` type from `api-client.ts`; it does not call `request()` or `configureApiClient()`.
- No component in `gui/src/*.tsx` (`ProfileEditor`, `CorporationForm`, `NaturalPersonForm`, `ServiceAccountForm`) calls `request()` directly — confirmed via grep. This matches the followup's own note that "no consumer was wired up yet at the time" the followup was written, and is still true today.
- `configureApiClient` and `request` are re-exported from `gui/src/lib/index.ts` → `gui/src/index.ts`, i.e. they are part of the library's public API surface consumed by other ModuleForge modules over yalc (e.g. `mod-users/gui`, per `gui/README.md`'s yalc workflow).

## Documentation surfaces

- No `gui/AGENTS.md` exists. The library's `gui/src/lib/api-client.ts` behavior is documented in the root `AGENTS.md`, section "### gui/ error and toast toolkit" (~line 110-119), which currently describes `request()`'s network-error/401/403 behavior and the `configureApiClient({ getToken, onUnauthenticated })` seam. This section will need a short addition once the guard shape is decided.
- `gui/README.md` covers only the build/yalc-link workflow and the Tailwind content-glob requirement — it does not document `api-client.ts` behavior and does not need changes for this followup.
- `docs/mf-standards/` (submodule, present in the main checkout but not initialized in this plan worktree) has no existing guidance on origin/CORS/allow-list patterns for GUI API clients (checked `architecture/api-response-design.md`, `architecture/authorization-design.md`, and the rest of `architecture/*.md` — no hits on `origin`, `CORS`, `allowlist`, `baseUrl`, or `allowedOrigins`). There is no established project convention to defer to for the two open design questions below.
- Per the dispatcher's framing, this change is library-internal (extends `configureApiClient`'s options and `request()`'s internal guard logic) and does not touch `docs/architecture.md` (does not exist in this project) or a `docs/*-spec.md` (none exists). It is being treated as **not** triggering the `analyze-change-request` Phase 4 "architectural implications" `doc-updates` phase machinery; the root `AGENTS.md` gui-toolkit section update is instead folded into the implementation task's normal requirements.

## No-test-runner constraint (followup `vr20`, out of scope here)

Confirmed via `gui/package.json`: no `vitest`/`jest`/any test-runner dependency or `test` script exists in `gui/`; only `tsc --noEmit` (`typecheck`) and `tsup`/`ladle` build/preview scripts. `AGENTS.md`'s `make test` target explicitly says "unit-test api; **typecheck gui** (model has no unit tests)" — confirming gui has no unit-test step in the build pipeline today.

Per explicit task instruction: do not add a test runner as part of this plan (that gap is tracked separately as `vr20`). The task(s) in this plan can only be validated via `tsc`/typecheck, manual code-path reasoning documented in the task doc, and (if useful) a throwaway manual verification script — not automated unit tests. This is recorded as a plan constraint in `plan/overview.md` and flagged for the manager.

## Open design questions (asked of the user — see `plan/notes/guard-shape-decision.md` and `plan/notes/guard-mismatch-behavior-decision.md` for answers once recorded)

1. **Config shape.** `allowedOrigins: string[]` allow-list vs. a single `baseUrl: string`. A `baseUrl` implies also using it to resolve/construct request URLs (a materially larger behavior change to `request()`'s calling convention, beyond a token-attachment guard); `allowedOrigins` is a pure validation guard that leaves `request(input, options)`'s existing calling convention untouched. No existing code or doc settles which the project wants.
2. **Mismatch behavior.** When configured and the resolved URL's origin is not in the allow-list: throw (fail closed), silently drop the token and still issue the request (fail open on connectivity / fail closed on token leakage), or log/warn only and still attach the token — and whether this should differ between development and production. No existing precedent (e.g. how `onUnauthenticated`/401 handling behaves) fully settles the desired mismatch semantics for a *client-side misconfiguration* case, as opposed to a *server-rejected* case.

**Already resolved, not asked:** default (unconfigured) behavior. The task instructions state explicitly that the default must stay fully backward-compatible with existing/dogfooded consumers that have not opted in — i.e., an unconfigured guard must be a complete no-op, identical to today's behavior. This is a hard constraint, not an open question.
