# Implement Origin Guard

## Purpose and scope

Resolve `plan/followups.yaml` item `7VD3` by adding a real, opt-in runtime origin-check guard to `request()`/`configureApiClient()` in `gui/src/lib/api-client.ts`, and document the new option in root `AGENTS.md`. No standard Flow skill covers this precisely — it is a scoped code change to a single file plus a short doc touch-up; implement directly per the requirements and validation below (no test-writing skill applies, per the no-test-runner constraint).

This task is the plan's only task. It fully resolves the origin-guard followup: implementation, JSDoc, and `AGENTS.md` update all land together.

## Requirements

Both open design questions are resolved (see `plan/notes/guard-shape-decision.md` and `plan/notes/guard-mismatch-behavior-decision.md`):

- **Config shape:** `allowedOrigins: string[]` allow-list (not `baseUrl`).
- **Mismatch behavior:** throw — fail closed, reject the request before it is made.
- **Default (unconfigured) behavior:** a hard constraint, not a design choice — when `allowedOrigins` has never been configured, `request()` must behave exactly as it does today (no origin check at all). This must hold for every existing/dogfooded caller.

### 1. New types and module state

Add a new exported interface next to `ApiClientAuthHandler` (same "Auth seam" region, or a new adjacent `// ─── Origin guard ───` region — match the file's existing banner-comment style):

```ts
export interface ApiClientOriginGuardConfig {
  /**
   * Allow-list of request origins (scheme + host + optional port, e.g.
   * "https://api.example.com" — no path, no trailing slash) permitted to
   * receive the bearer token. When unset (the default), request() performs
   * no origin check at all — identical to pre-guard behavior. When set,
   * request() throws ApiRequestError("origin_not_allowed", ...) before
   * issuing any request whose resolved target origin is not in this list
   * (including when the target origin cannot be resolved at all).
   */
  allowedOrigins?: string[];
}
```

Add module-singleton state alongside `authHandler`:

```ts
let originGuardConfig: ApiClientOriginGuardConfig = {};
```

`{}` (i.e. `allowedOrigins` absent) is the default, no-op state — do not default `allowedOrigins` to `[]` at module init; an empty array is a distinct, deliberate "block every origin" configuration a caller can opt into, not the default.

### 2. Extend `configureApiClient()`

Change the parameter type to accept both the existing auth-handler fields and the new guard field, and merge each independently so the two seams stay decoupled and each preserves the *partial merge* semantics the doc comment already promises (omitting a field keeps its current value):

```ts
export function configureApiClient(
  config: Partial<ApiClientAuthHandler> & Partial<ApiClientOriginGuardConfig>
): void {
  const { allowedOrigins, ...handlerFields } = config;
  authHandler = { ...authHandler, ...handlerFields };
  if (allowedOrigins !== undefined) {
    originGuardConfig = { allowedOrigins };
  }
}
```

Notes for the implementer:
- Destructuring `allowedOrigins` out of `config` before spreading `handlerFields` onto `authHandler` is required — `ApiClientAuthHandler` must not gain a stray `allowedOrigins` property.
- Only reassign `originGuardConfig` when `allowedOrigins` is explicitly present in the call (`!== undefined`), exactly mirroring how omitted `authHandler` fields keep their prior value. A caller who calls `configureApiClient({ getToken: ... })` after previously configuring `allowedOrigins` must not have their origin guard silently cleared.
- Passing `allowedOrigins: []` explicitly is valid and means "block everything" — do not special-case empty array as "no guard."
- Update the existing JSDoc above `configureApiClient` (currently describes only the auth-handler seam) to also describe the origin-guard field: what it does, that it is opt-in, and that omitting it never changes existing behavior.

### 3. Origin resolution helper

Add a small private helper (not exported) that resolves the origin `request()` will check:

```ts
function resolveRequestOrigin(input: string | URL): string | null {
  try {
    if (input instanceof URL) return input.origin;
    if (typeof window !== 'undefined' && window.location) {
      return new URL(input, window.location.origin).origin;
    }
    return new URL(input).origin;
  } catch {
    return null;
  }
}
```

Rationale to preserve in a short comment above the helper:
- `URL` instances and absolute URL strings (`"https://..."`) resolve directly.
- Relative strings (e.g. `"/api/widgets"`) only resolve when a browser `window.location` is available to serve as the base; outside a browser (SSR) a relative input's origin cannot be determined and resolution intentionally fails (returns `null`).
- Any parse failure returns `null` rather than throwing — the caller (`request()`) treats `null` as "not verified as allowed" and fails closed, per Requirement 4.

### 4. Enforce the guard in `request()`

Insert the check as the first statement inside `request()`, before the `skipAuthRedirect`/`fetchOptions` destructure and before `authHandler.getToken()` is called — the guard must reject before any request-preparation work happens, matching "reject the request before it's made":

```ts
if (originGuardConfig.allowedOrigins) {
  const origin = resolveRequestOrigin(input);
  if (!origin || !originGuardConfig.allowedOrigins.includes(origin)) {
    throw new ApiRequestError(
      'origin_not_allowed',
      origin
        ? `Request origin "${origin}" is not in the configured allowedOrigins list.`
        : 'Request origin could not be determined and no allowedOrigins match was possible.',
      0
    );
  }
}
```

Notes:
- Guard only runs when `originGuardConfig.allowedOrigins` is truthy (i.e. was configured — including the deliberate `[]` case, which is truthy as an array and will correctly reject everything since `.includes` on an empty array is always `false`).
- Reuse `ApiRequestError` (do not add a new error class) — status `0` mirrors the existing client-synthesized `network_error` convention (no real HTTP status because no request was made); the new `code` value `'origin_not_allowed'` is what callers switch on to distinguish this case from `network_error` and server-returned errors.
- This throw must happen synchronously before the `try { response = await fetch(...) }` block, so a guard rejection never touches the network and never invokes `authHandler.getToken()`.

### 5. Update `request()`'s JSDoc

Add a bullet to the existing JSDoc list above `request()` documenting the new guarded-rejection case, in the same style as the existing `network_error` bullet, e.g.: "When `configureApiClient({ allowedOrigins })` is set, a request whose resolved target origin is not in the list (or cannot be resolved) is rejected before any network call as `ApiRequestError('origin_not_allowed', ..., 0)` — the bearer token is never attached and `fetch` is never invoked." Do not remove or reword the existing bullets.

### 6. Update root `AGENTS.md`

Edit the "### gui/ error and toast toolkit" section's `api-types.ts` bullet (around line 114) to mention the new opt-in origin guard, consistent in tone/brevity with how it currently documents `configureApiClient({ getToken, onUnauthenticated })`. Example addition (adapt wording to fit the existing sentence flow rather than pasting verbatim): "an optional `allowedOrigins: string[]` guard (also set via `configureApiClient`) rejects — throws `ApiRequestError('origin_not_allowed', ...)` before any network call — requests whose resolved target origin isn't in the list; unconfigured, this is a complete no-op." Keep the edit to this one bullet; no other `AGENTS.md` section needs to change.

### Non-goals (explicitly out of scope for this task)

- Do not add a test runner to `gui/` (followup `vr20`).
- Do not change default token storage (`localStorage`) — followup `pBoG`.
- Do not add a `baseUrl` concept or otherwise change how `request()` resolves/constructs URLs beyond the origin-comparison logic above.
- Do not touch `gui/README.md` (confirmed out of scope — it documents build/yalc workflow only, not `api-client.ts` behavior).

## Validation

`gui/` has no test runner configured (followup `vr20`, out of scope for this plan — see [Constraints for the manager](../overview.md#constraints-for-the-manager) in `plan/overview.md`). Validation is therefore typecheck plus careful manual/documented reasoning, not new automated tests:

1. **Typecheck.** Run `bun run typecheck` in `gui/` (i.e. `cd gui && bun run typecheck`, equivalent to `tsc --noEmit`). Must pass with zero errors — this is the only automated check available for this task and is a hard requirement.
2. **Backward-compatibility trace-through (write this reasoning into the commit message or a short comment/PR note — no test file to encode it in).** Confirm and state explicitly:
   - With `configureApiClient()` never called (or called without `allowedOrigins`), `originGuardConfig.allowedOrigins` stays `undefined`, the `if (originGuardConfig.allowedOrigins)` guard in `request()` is `falsy`, and `request()`'s control flow is byte-for-byte identical to before this change — every existing/dogfooded caller sees zero behavior change.
   - Calling `configureApiClient({ getToken: ... })` alone (no `allowedOrigins`) does not touch `originGuardConfig` — the destructure/`!== undefined` guard in `configureApiClient` prevents the auth-handler-only call path from resetting a previously configured guard, and also does not enable a guard that was never configured.
3. **Manual trace-through of guard branches** (document the traced cases, e.g. as a short list in the commit message):
   - Absolute-URL string input (`"https://api.example.com/v1/widgets"`) with a matching entry in `allowedOrigins` → request proceeds normally, token attached if present.
   - Absolute-URL string input with an origin not in `allowedOrigins` → `ApiRequestError('origin_not_allowed', ..., 0)` thrown synchronously; `fetch` never called; confirm this by inspection of the code path (the throw is unconditionally before the `fetch` call).
   - `URL` object input, both matching and non-matching cases → same as above via `input.origin`.
   - Relative string input (e.g. `"/v1/widgets"`) in a browser context (`window.location` available) → resolves against `window.location.origin`; matches or rejects accordingly.
   - Relative string input with no `window` (SSR / non-browser call) and a guard configured → `resolveRequestOrigin` returns `null` → request rejected (fail-closed per Requirement 4's explicit "including when the target origin cannot be resolved at all" clause).
   - `allowedOrigins: []` explicitly configured → every request rejected (empty-array `.includes` is always `false`); confirm this was **not** implemented as a "guard disabled" special case.
4. **Grep sweep.** `grep -n "allowedOrigins" gui/src/lib/api-client.ts` shows the new interface field, the module state, the `configureApiClient` destructure, and the `request()` guard check — confirming no stray/duplicate declarations. `grep -n "origin_not_allowed" gui/src/lib/api-client.ts` shows exactly the one throw site plus its JSDoc mention.
5. **AGENTS.md check.** Confirm the "### gui/ error and toast toolkit" section's `api-types.ts` bullet now mentions the `allowedOrigins` guard and its throw-before-fetch, no-op-when-unconfigured behavior, and that no other section of `AGENTS.md` was touched.
6. **Public API surface check.** Confirm `ApiClientOriginGuardConfig` is exported (if declared) consistently with how `ApiClientAuthHandler`/`RequestOptions` are exported today, and that `gui/src/lib/index.ts` / `gui/src/index.ts` re-export it if those files re-export the sibling interfaces by name (check current re-export style before deciding whether an explicit re-export line is needed).

## Assumptions

- `allowedOrigins` entries are exact origin strings (`scheme://host[:port]`) compared via strict equality (`Array.prototype.includes`) against `URL.origin`/`resolveRequestOrigin`'s output — no wildcard, subdomain, or pattern matching is in scope. If a future consumer needs wildcard matching, that is a new followup, not part of this task.
- `URL.origin` and `new URL(...).origin` normalize scheme/host/port consistently with how a consuming app would specify `allowedOrigins` (e.g. default ports are omitted from `.origin` the same way in both). No extra normalization step is required.
- This task assumes no current consumer calls `request()` yet (confirmed in [research notes](../notes/api-client-origin-guard-context.md) — `use-api-error.ts` only imports the `ApiRequestError` type; no `.tsx` component calls `request()` directly), so there is no existing call site to migrate or that could regress from this change beyond the guard itself.

## References

- `gui/src/lib/api-client.ts` — the file being modified (`ApiClientAuthHandler`, `configureApiClient`, `request()`, `ApiRequestError`).
- `AGENTS.md` — "### gui/ error and toast toolkit" section (~line 110-119), specifically the `api-types.ts` bullet at ~line 114.
- `plan/notes/api-client-origin-guard-context.md` — prior research: current implementation details, consumer survey, documentation surfaces, no-test-runner constraint.
- `plan/notes/guard-shape-decision.md` — resolved: `allowedOrigins: string[]`.
- `plan/notes/guard-mismatch-behavior-decision.md` — resolved: throw / fail closed.
- `plan/followups.yaml` item `7VD3` — the followup this task resolves (removed by the manager/`apply-task-report` at task close, not by this task directly).

## Metadata

architectural_impact: false

## Status

**Outcome:** succeeded (2026-07-17)

**Validation summary:**
1. Typecheck — `cd gui && bun run typecheck` (`tsc --noEmit`) passed with zero errors, after running `bun install` in `gui/` (no `node_modules` was present at worktree provisioning).
2. Backward-compatibility trace-through — confirmed by code inspection:
   - `originGuardConfig` is initialized to `{}` (`allowedOrigins` absent/`undefined`) at module load and is only ever reassigned inside `configureApiClient` when its caller explicitly passes `allowedOrigins !== undefined`. With `configureApiClient()` never called, or called without `allowedOrigins`, `originGuardConfig.allowedOrigins` stays `undefined`, so the `if (originGuardConfig.allowedOrigins)` guard at the top of `request()` is falsy and every statement after it runs exactly as before this change — byte-for-byte identical control flow for every existing/dogfooded caller.
   - `configureApiClient({ getToken: ... })` destructures `allowedOrigins` out of `config` (it is `undefined` when the caller only passes auth-handler fields) and the `if (allowedOrigins !== undefined)` guard skips reassigning `originGuardConfig` in that case — an auth-handler-only call never resets a previously configured guard, and never spuriously enables one.
3. Manual trace-through of guard branches (by code inspection of the single guard block inserted as the first statement in `request()`, before the `fetchOptions` destructure and before `authHandler.getToken()`):
   - Absolute-URL string, origin present in `allowedOrigins` → when `window.location` is available, `new URL(input, window.location.origin).origin` resolves an absolute `input` to its own origin (the `base` argument is ignored by the WHATWG URL parser when `input` is already absolute); when no `window` is available, the fallback `new URL(input).origin` branch resolves it the same way. Either path yields the request's actual origin; `.includes` is true; the guard's `if` is false; execution falls through to the unchanged request body.
   - Absolute-URL string, origin not in `allowedOrigins` → `resolveRequestOrigin` returns a non-null origin; `.includes` is false; `ApiRequestError('origin_not_allowed', ..., 0)` is thrown synchronously, before the `try { fetch(...) }` block — `fetch` is never invoked and `authHandler.getToken()` is never called for this request.
   - `URL` object input, matching and non-matching — `resolveRequestOrigin`'s `input instanceof URL` branch returns `input.origin` directly; same match/reject logic as above applies.
   - Relative string input (e.g. `"/v1/widgets"`) in a browser context — `typeof window !== 'undefined' && window.location` is true, so `new URL(input, window.location.origin).origin` resolves the relative path against the current page origin; matches or rejects accordingly.
   - Relative string input with no `window` (SSR) and a guard configured — the `window` branch is skipped, falls to `new URL(input).origin`, which throws for a bare relative path (no base) — caught by the `try/catch`, returns `null`; back in `request()`, `!origin` is true so the request is rejected regardless of `allowedOrigins` contents (fail-closed per Requirement 4's "including when the target origin cannot be resolved at all" clause).
   - `allowedOrigins: []` configured explicitly — `originGuardConfig.allowedOrigins` is `[]`, which is truthy as an array reference, so the outer `if` still runs; `[].includes(origin)` is always `false` for any resolved origin, so every request is rejected. Confirmed by reading `configureApiClient`: `allowedOrigins` is stored as given (`{ allowedOrigins }`) with no `.length` check or other special-casing of the empty-array case anywhere in the module.
4. Grep sweep — `grep -n "allowedOrigins" gui/src/lib/api-client.ts` shows the interface field, module state (`originGuardConfig`), the `configureApiClient` destructure/reassignment, the `resolveRequestOrigin` JSDoc reference, and the `request()` guard check, with no stray/duplicate declarations. `grep -n "origin_not_allowed" gui/src/lib/api-client.ts` shows the one throw site plus its two JSDoc mentions (on the `allowedOrigins` field doc comment and on `request()`'s doc comment) — no duplicate throw sites.
5. AGENTS.md check — the "### gui/ error and toast toolkit" section's `api-types.ts` bullet (line 114) now mentions the `allowedOrigins` guard, its throw-before-fetch behavior, and its no-op-when-unconfigured default; `git diff AGENTS.md` confirms no other line in the file changed.
6. Public API surface check — `ApiClientOriginGuardConfig` is declared `export interface` at module top level, consistent with `ApiClientAuthHandler`/`RequestOptions`. `gui/src/lib/index.ts` and `gui/src/index.ts` both re-export via `export * from './api-client'` (and `export * from './lib'` respectively) rather than naming individual symbols, so the new interface is re-exported automatically with no additional re-export line needed — matches the file's existing re-export style.

**Affected source files:**
- `gui/src/lib/api-client.ts`
- `AGENTS.md`
- `plan/phase-01-gui-origin-guard/001-implement-origin-guard.md` (this file)

**Assumptions relied on (from `## Assumptions`):** all three — exact-string origin comparison (no wildcard matching), `URL.origin` normalization is sufficient with no extra step, and no current in-repo consumer calls `request()` (re-confirmed via `grep -rn "configureApiClient\|api-client" gui/src` during implementation — only the pre-existing `ApiRequestError` type import in `use-api-error.ts`).
