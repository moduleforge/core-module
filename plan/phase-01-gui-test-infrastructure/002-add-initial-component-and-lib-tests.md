# Add Initial Component And Lib Tests

## Purpose and scope

Writes the first real tests in `gui/`, establishing the colocated `*.test.ts(x)` pattern the [test runner choice note](../notes/test-runner-choice.md) and task 001 set up. Deliberately not exhaustive: two presentational component tests (`FieldError`, `ErrorBanner`) and one lib unit test (`api-client`'s `request()` wrapper). No tests for `ProfileEditor`, `CorporationForm`, `NaturalPersonForm`, `ServiceAccountForm`, `useApiError`, or `ToastProvider` — those are out of scope for this plan (see `plan/overview.md`).

Depends on task 001 (`bun test` must be runnable with the happy-dom/testing-library preload chain in place before these tests can execute).

## Requirements

Colocate each test file next to the source it covers, matching the existing `*.stories.tsx` colocation convention already used in `gui/src/`.

1. **`gui/src/FieldError.test.tsx`** — covers `gui/src/FieldError.tsx` (`FieldError` component: `{ error?: FieldErrorData | null; id?: string; className?: string }`, renders `null` when `error` is falsy, otherwise a `<p role="alert">` with `error.message`). At minimum:
   - Renders nothing (empty container) when `error` is `undefined`.
   - Renders nothing when `error` is `null`.
   - Renders an element with `role="alert"` containing the error's `message` text when `error` is a populated `FieldErrorData` (e.g. `{ field: 'email', code: 'invalid', message: 'Enter a valid email address.' }`).
   - The rendered element's `id` matches the `id` prop when one is passed (confirms the `aria-describedby` association point works).

2. **`gui/src/ErrorBanner.test.tsx`** — covers `gui/src/ErrorBanner.tsx` (`ErrorBanner` component and its `resolve()` shape-normalization: a plain `string`, an `ApiError`-like `{ message }`, or an explicit `{ title?, description }`). At minimum:
   - Renders nothing when `error` is `undefined`/`null`.
   - A plain `string` error renders the string as the description, with no title element present.
   - An `ApiError`-like value (`{ code: 'forbidden', message: '...' }`) renders `message` as the description, no title.
   - An explicit `{ title, description }` value renders both the title and description text.
   - The rendered root carries the `destructive` `Alert` variant's `role="alert"` (from the underlying `Alert` primitive in `gui/src/ui/alert.tsx`) — assert via `getByRole('alert')` rather than a CSS-class check, since variant class names are an implementation detail.

3. **`gui/src/lib/api-client.test.ts`** — unit tests for `request()` in `gui/src/lib/api-client.ts`, mocking `global.fetch` with `bun:test`'s `mock()` (do **not** hit the network). Restore `global.fetch` to its original value in an `afterEach` in this file so the mock doesn't leak into other test files run in the same `bun test` invocation. At minimum, cover:
   - **Success path:** a mocked `fetch` resolving to a `200` `Response` with a JSON body — `request()` resolves to the parsed, typed body.
   - **Empty success body:** a mocked `fetch` resolving to a `204` `Response` — `request()` resolves to `undefined` without attempting to parse a body.
   - **Network failure:** a mocked `fetch` that rejects (simulating a transport failure) — `request()` throws an `ApiRequestError` with `code: 'network_error'` and `status: 0`.
   - **Non-2xx with the nested error envelope:** a mocked `fetch` resolving to a `404` `Response` whose JSON body is `{ error: { code: 'not_found', message: '...', details: [...] } }` — `request()` throws an `ApiRequestError` whose `code`, `message`, `status` (404), and `details` all come from the envelope.

   Do not test the `401` auth-redirect path or the `allowedOrigins` origin-guard path in this task — both are real `request()` behaviors but are follow-on coverage, not part of this plan's "establish the pattern" scope (see `plan/overview.md`'s Out of scope). Do not call `configureApiClient(...)` in this file (keeps `authHandler`/`originGuardConfig` module state at its default across every test in the file, avoiding cross-test state leakage since `api-client.ts`'s auth handler and origin guard are module-level mutable singletons).

## Validation

Run from `gui/`:

- `bun run test` — all new tests pass; total test/assertion counts in the run output should reflect the cases enumerated above (roughly 4 for `FieldError`, 5 for `ErrorBanner`, 4 for `api-client`; exact counts may vary slightly with implementation, but none should be skipped or failing).
- `bun run typecheck` — must still pass; the new `*.test.ts(x)` files are covered by `gui/tsconfig.json`'s existing `"include": ["src"]`, so they're typechecked automatically.
- `bun run build` — must still succeed; confirm (`ls dist`) that none of the new `*.test.ts(x)` files are present in `dist/` output.
- Re-run `bun run test` a second time to confirm no test leaves global state (e.g. an un-restored `global.fetch` mock) that causes flakiness or ordering-dependent failures.

## Assumptions

- Task 001 has landed first (or its changes are otherwise present in the working tree/branch this task builds on) — the `bunfig.toml` preload chain, `src/test-support/matchers.d.ts` augmentation, and `"test"` script must already exist.

## References

- [`plan/notes/test-runner-choice.md`](../notes/test-runner-choice.md) — the runner/config decision and spike this task's tests run against.
- `gui/src/FieldError.tsx`, `gui/src/ErrorBanner.tsx`, `gui/src/lib/api-client.ts`, `gui/src/lib/api-types.ts`, `gui/src/ui/alert.tsx` — source under test.

## Status

Implementation outcome: **succeeded** (2026-07-18).

- `gui/src/FieldError.test.tsx` — 4 tests (undefined, null, populated-error alert text, `id` propagation).
- `gui/src/ErrorBanner.test.tsx` — 6 tests (undefined, null, plain-string, `ApiError`-like, explicit `{title, description}`, dedicated `role="alert"` assertion).
- `gui/src/lib/api-client.test.ts` — 4 tests (200 success, 204 empty body, network-error rejection, 404 nested-envelope non-2xx), with `global.fetch` restored in `afterEach`.
- `bun run test` reported `14 pass / 0 fail / 23 expect() calls` across 3 files, both on the initial run and a second re-run (no leaked global state).
- `bun run typecheck` passed with no errors.
- `bun run build` succeeded; `ls dist` confirms no `*.test.ts(x)` files in the build output.
- The worktree had no installed `node_modules` at task start despite `gui/bun.lock` already containing the task-001 test-runner devDependencies; ran `bun install` inside `gui/` to materialize `node_modules` from the existing lockfile before running validation (no `package.json`/`bun.lock` changes resulted).

Affected source files (repo-relative):
- `gui/src/FieldError.test.tsx`
- `gui/src/ErrorBanner.test.tsx`
- `gui/src/lib/api-client.test.ts`

## Metadata

architectural_impact: false
