# Client Foundation

## Purpose and scope

Establish the mod-core/gui client foundation the error widgets build on: the wire types, the
`ApiRequestError` class, and a shared typed `request()` fetch wrapper. This is a **promotion target** —
the design doc and prior planning locked in that mod-core/gui provides a reusable client helper (not
just the types) that other modules' GUI packages adopt in later, separate plans. Build the helper here.

Source of truth: the **GUI-facing error-data contract** section of
`docs/mf-standards/architecture/api-response-design.md` (Wire types, `Client contract (ApiRequestError)`).
The types are a **superset-compatible extension** of `mod-users/gui/src/lib/api.ts`
(`ApiError {code,message}`, `ApiErrorResponse {error}`) — additive optional `details` only. That
mod-users file is in a different repo and is **not** touched by this plan.

Skill: `implement-task` (TypeScript/React).

## Requirements

Create a new client module under `gui/src/lib/` (e.g. `gui/src/lib/api-client.ts`, or split types into
`gui/src/lib/api-types.ts` and the wrapper into `gui/src/lib/api-client.ts` — implementer's choice, keep
it cohesive and barrel-exportable):

1. **Wire types** (exported):
   ```ts
   export interface FieldError { field: string; code: string; message: string; }
   export interface ApiError { code: string; message: string; details?: FieldError[]; }
   export interface ApiErrorResponse { error: ApiError; }
   ```

2. **`ApiRequestError`** class (exported), extending `Error`, per the design doc:
   ```ts
   export class ApiRequestError extends Error {
     code: string;            // reserved top-level code, or "network_error"
     status: number;          // HTTP status, or 0 for a transport failure
     details?: FieldError[];
   }
   ```
   Set `name = "ApiRequestError"`. Constructor carries `code`, `message`, `status`, optional `details`.

3. **`request()` typed fetch wrapper** (exported) implementing the design doc's two special cases:
   - **`network_error` / status 0** — when `fetch` rejects (transport failure, no HTTP response),
     synthesize and throw `new ApiRequestError("network_error", <message>, 0)` (no envelope; classified
     toast-worthy downstream, never a field error).
   - On a non-`2xx` response, parse the nested `{error:{code,message,details?}}` envelope and throw
     `ApiRequestError` carrying `error.code`, `error.message`, the HTTP `status`, and `error.details`.
     Be defensive if the body is missing/unparseable (fall back to a generic code/message with the real
     status).
   - **401 redirect split** — on `401 unauthenticated`, clear the stored auth token and redirect to
     login **unless** a `skipAuthRedirect` option is set. **Only 401 triggers the redirect.** A
     `403 forbidden` (including a masked not-found) MUST NOT redirect — surface it as a thrown
     `ApiRequestError` for inline handling.
   - On success, parse and return the JSON body typed via a generic (`request<T>(...) : Promise<T>`).
     Handle empty/204 bodies gracefully.
   - Accept a `RequestOptions extends RequestInit` with `skipAuthRedirect?: boolean` (mirror the
     mod-users `RequestOptions` documented behavior for `skipAuthRedirect`).
   - **Token storage & redirect are environment-dependent.** mod-core/gui is a component library, not
     an app, so hard-coding `localStorage`/`window.location` couples it to a browser app. Prefer a
     small injectable seam: a module-level configurable auth handler (e.g. a `configureApiClient({
     getToken, onUnauthenticated })` setter) with a browser-default, so consuming apps can supply their
     own token/redirect behavior. If a simpler direct `window`/`localStorage` default is chosen, guard
     for non-browser environments so `tsc`/SSR does not break. Document the chosen seam in the file.

4. Export everything above from `gui/src/index.ts` (add a `lib`/client export line alongside existing
   exports). If introducing a `gui/src/lib/index.ts` barrel, wire it through; otherwise export the file
   directly.

Do not add any new dependency. Do not modify `mod-users`. Keep the public surface minimal and typed.

## Validation

- New client module exists under `gui/src/lib/`; `FieldError`, `ApiError`, `ApiErrorResponse`,
  `ApiRequestError`, `request` (and `RequestOptions`, and any `configureApiClient` seam) are exported
  and reachable from `gui/src/index.ts`.
- `cd gui && bun run typecheck` (tsc --noEmit) passes.
- `cd gui && bun run build` (tsup) succeeds and emits types.
- Code review confirms: transport failure → `network_error`/status 0; 401 → redirect unless
  `skipAuthRedirect`; 403 → never redirects; nested envelope parsed into `code`/`message`/`status`/
  `details`.
- `grep -n "network_error\|skipAuthRedirect\|ApiRequestError\|class ApiRequestError" gui/src/lib/*.ts`
  confirms the special cases are present.

## Metadata

architectural_impact: true

## References

- `docs/mf-standards/architecture/api-response-design.md` — **GUI-facing error-data contract** (Wire
  types, `Client contract (ApiRequestError)`, the 401-redirect/403-no-redirect split, `network_error`
  synthesis).
- `mod-users/gui/src/lib/api.ts` — the existing `ApiError`/`ApiErrorResponse`/`ApiRequestError`/
  `RequestOptions` (incl. `skipAuthRedirect` semantics) this is a superset-compatible extension of.
  **Reference only — not modified, different repo.**
- `gui/src/index.ts`, `gui/src/lib/utils.ts` — existing gui barrel and lib conventions.
- `gui/tsconfig.json`, `gui/tsup.config.ts` — strict TS, bundler resolution, tsup entry is
  `src/index.ts`.

## Checkpoint hints

- After the wire types + `ApiRequestError` typecheck.
- After the `request()` wrapper with `network_error` synthesis and the 401/403 split.
- After wiring the barrel export and a clean typecheck.

## Status

- **Outcome:** succeeded
- **Date:** 2026-07-15
- **Implementation:** Split the module per the task's suggested layout —
  `gui/src/lib/api-types.ts` (wire types: `FieldError`, `ApiError`, `ApiErrorResponse`) and
  `gui/src/lib/api-client.ts` (`ApiRequestError`, the `ApiClientAuthHandler` seam +
  `configureApiClient()`, `RequestOptions`, `request<T>()`), barreled through a new
  `gui/src/lib/index.ts` and re-exported from `gui/src/index.ts` via `export * from './lib';`.
  `request()` parses the non-2xx envelope once, throws `ApiRequestError` with the real
  `code`/`message`/`status`/`details` (falling back to a generic code/message if the body is
  missing/unparseable), and only routes 401 through the auth handler's `onUnauthenticated()`
  (skippable via `skipAuthRedirect`); 403 and all other non-2xx statuses fall through the same
  generic throw with no redirect. A transport failure (`fetch` reject) short-circuits to
  `ApiRequestError("network_error", <message>, 0)` before any envelope parsing.
- **Auth seam:** `configureApiClient({ getToken, onUnauthenticated })` overrides a
  browser-default (`localStorage` `auth_token` key, hard redirect to `/auth/login`) that is
  SSR-safe (`typeof window === 'undefined'` guarded no-op). Documented inline in
  `api-client.ts` above the seam.
- **Dependencies:** the dispatch's `dependencies_installed` was `none`, but `gui/` has a real
  `bun.lock`/`package.json` with no `node_modules` present. Ran `bun install` from `gui/`
  (matches the existing lockfile exactly — `git diff gui/bun.lock` is empty, no dependency was
  added or changed) so `bun run typecheck` / `bun run build` could execute; see
  `flagged_for_manager` in the structured report for the mismatch note.
- **Validation:** `bun run typecheck` (tsc --noEmit) — clean, no output. `bun run build`
  (tsup) — succeeds, emits `dist/index.d.ts`/`dist/index.d.mts` with all required symbols.
  Manual code review of `request()` confirms all four special cases. Grep check for
  `network_error|skipAuthRedirect|ApiRequestError|class ApiRequestError` in
  `gui/src/lib/*.ts` — matches present.
- **Files:** `gui/src/lib/api-types.ts`, `gui/src/lib/api-client.ts`, `gui/src/lib/index.ts`,
  `gui/src/index.ts` (added `export * from './lib';`).
- **No new dependency added; `mod-users` not touched.**
