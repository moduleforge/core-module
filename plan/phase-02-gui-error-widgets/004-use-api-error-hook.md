# useApiError Hook

## Purpose and scope

Implement `useApiError(error)` — the single place the design doc's surface-classification rule lives —
and finalize the mod-core/gui barrel so the whole error/toast toolkit is exported and typechecks as a
cohesive unit. This is the capstone of Phase 02.

Source of truth: the **Surface classification** table in
`docs/mf-standards/architecture/api-response-design.md`. Implement the routing **exactly** as specified.

Depends on: `phase-02-gui-error-widgets/001-client-foundation.md` (`ApiRequestError`, wire types),
`002-toast-provider.md` (`useToast`), and `003-error-widgets.md` (the widgets that consume the hook's
output). The hook returns *data*; it does not import the widgets.

Skill: `implement-task` (TypeScript/React).

## Requirements

Create the hook (e.g. `gui/src/lib/use-api-error.ts` or `gui/src/useApiError.ts`):

1. **`useApiError(error: ApiRequestError | null | undefined)`** returns
   `{ fieldErrors, bannerError }` and dispatches toast-worthy errors to the toast provider (via
   `useToast` from task 002). Implement the surface-classification table exactly:

   | Surface | When |
   |---|---|
   | **Field-level** | Any `details[]` entry whose `field` matches a rendered input. Returned in `fieldErrors` (keyed/lookup-able by `field`). |
   | **Banner** | Top-level `forbidden`, `not_found`, or `conflict`/`invalid_input` with **no** field-bound details; **also** any `details[]` entry with **no** matching input. Returned as `bannerError`. |
   | **Toast-worthy** | `network_error`, `internal_error` (500), and optimistic-update rollbacks. **Dispatched to the toast provider**, not returned for inline rendering. |

   - `unauthenticated` is **neither** field/banner/toast — it is handled by the task-001 redirect path;
     `useApiError` must not surface it inline (it should not normally reach the hook because `request()`
     redirects, but guard so it is not mis-routed to a banner/toast).
   - Because "which `field`s are rendered inputs" is caller-dependent, accept the set of known field
     names as an argument or option (e.g. `useApiError(error, { fields })` or a returned matcher) so the
     hook can split `details[]` into field-bound (→ `fieldErrors`) vs unbound (→ `bannerError`). Choose
     a clean, documented API; the classification logic — not the exact signature — is what must match
     the table.
   - The one-line rule to honor: **inline (field or banner) when the user can act on it where they are;
     toast-worthy when the failure is transient/global and the form is not where it is resolved.**
   - Toast dispatch must be effectful and idempotent per error instance (dispatch in a `useEffect`
     keyed on the error, not on every render) so a re-render does not spam duplicate toasts.
   - `null`/`undefined` error → empty `fieldErrors`, `null` `bannerError`, no toast.

2. **Finalize the barrel.** Ensure `gui/src/index.ts` exports the complete Phase-02 surface:
   the client foundation (task 001: wire types, `ApiRequestError`, `request`, `RequestOptions`, any
   `configureApiClient`), the toast surface (task 002: `ToastProvider`, `useToast`), the widgets
   (task 003: `<FieldError>`, `<ErrorBanner>`), and this hook (`useApiError`) — all alongside the
   existing exports (`./ui`, `ProfileEditor`, the form components). Resolve any export duplication/name
   collisions.

3. **Whole-package verification.** Run and pass `cd gui && bun run typecheck` and `cd gui && bun run
   build` for the complete package.

Do not add a dependency. No `any` in the hook's public types.

## Validation

- `useApiError` exists and is exported from `gui/src/index.ts`.
- The classification matches the design doc table: field-bound details → `fieldErrors`; top-level
  `forbidden`/`not_found`/`conflict`/`invalid_input`-without-field-details and unmatched details →
  `bannerError`; `network_error`/`internal_error`/rollbacks → toast dispatch; `unauthenticated` not
  surfaced inline.
- Toast dispatch is `useEffect`-guarded (no duplicate toasts on re-render).
- `gui/src/index.ts` exports the full set (grep confirms `useApiError`, `ToastProvider`, `useToast`,
  `FieldError`, `ErrorBanner`, `ApiRequestError`, `request` are all reachable).
- `cd gui && bun run typecheck` (tsc --noEmit) passes; `cd gui && bun run build` succeeds.
- `git diff gui/package.json` shows no new dependency.

## Metadata

architectural_impact: true

## Assumptions

- Tasks 001, 002, and 003 have landed (wire types + `ApiRequestError`, `ToastProvider`/`useToast`, and
  the `<FieldError>`/`<ErrorBanner>` widgets are all present and exported).

## References

- `docs/mf-standards/architecture/api-response-design.md` — **Surface classification** table (the
  routing spec) and the one-line classification rule; **Widget set implied** (`useApiError` is "the
  single place the routing rule lives").
- `plan/phase-02-gui-error-widgets/001-client-foundation.md`, `002-toast-provider.md`,
  `003-error-widgets.md` — the surfaces this hook ties together.
- `gui/src/index.ts` — the barrel to finalize.

## Checkpoint hints

- After `useApiError`'s classification returns correct `fieldErrors`/`bannerError`.
- After toast dispatch is wired via `useEffect` + `useToast`.
- After finalizing `gui/src/index.ts` and a clean whole-package typecheck + build.
