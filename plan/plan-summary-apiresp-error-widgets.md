# Plan Session Summary: API Response & Error Widgets (Wave 0 / Phase 2)

## What was planned and why

This plan implemented **phase 2** of a previously-planned three-phase API response & error
standardization effort. Phase 1 (design + documentation, no code) had already landed separately; the
settled contract lives at `docs/mf-standards/architecture/api-response-design.md`
(submodule-mounted from the `docs-mf-standards` repo) and served as the source of truth for shapes,
sentinel names, function signatures, and error-code vocabulary throughout.

Scope was **mod-core only**. The plan built the two deliverables the design doc specified as "phase 2,
design only," and dogfooded the Go one within mod-core itself:

1. **Go — `api/apiresp` package** (new import path `github.com/moduleforge/core-api/apiresp`): the
   canonical sentinel set, a bare success encoder (`WriteJSON`), a sentinel→status/code mapper and
   nested-envelope encoder (`WriteError`), a field-error builder (`InvalidInput`), and the `FieldError`
   wire type — plus a **dogfooding migration** of mod-core's own `api/httpapi/response.go` trio
   (`jsonOK`/`jsonErr`/`writeServiceErr`) and `api/service/errors.go` sentinels onto the new package,
   proving it end-to-end before any other repo depends on it.
2. **GUI — `mod-core/gui` error/toast widgets**: wire types + `ApiRequestError` class + a shared typed
   `request()` fetch wrapper (promoted into `mod-core/gui` for other modules to adopt later), a
   `<FieldError>` widget, an `<ErrorBanner>` widget (wrapping the existing `Alert` `destructive`
   variant), a Toast provider + `useToast` hook (built on the already-present `radix-ui` Toast
   primitive), and a `useApiError(error)` hook implementing the design doc's surface-classification
   table.

A final documentation phase brought the repo-owned `AGENTS.md` and `README.md` into sync with both new
subsystems.

Explicitly out of scope and untouched: `model/` (a separate Go module), the design doc itself (no
doc-only patches), any other repo's response writer/sentinels/GUI components (6 separate follow-on
plans), and `mod-users/gui/src/lib/api.ts`. No app regeneration was performed.

Phase 01 (apiresp-go) and Phase 02 (gui-error-widgets) were planned to run — and did run — fully
independently/concurrently, since the GUI phase consumes the *wire contract* (fully specified in the
design doc) rather than the Go code. Phase 03 (doc-updates) ran last, after both implementation phases
landed.

## What shipped

### Phase 01 — Apiresp Go Package (`phase-01-apiresp-go`)

- **001 — Create Apiresp Package** (merge `359e35be54a90cf688c9baeae3489bb4d188b429`). Created the new
  `api/apiresp` package: sentinels (`ErrUnauthenticated`, `ErrForbidden`, `ErrNotFound`,
  `ErrInvalidInput`, `ErrConflict`), the `Envelope`/`ErrorBody`/`FieldError` wire types (`Details` with
  `omitempty`), `WriteJSON`, `WriteError` (`errors.Is` classification, nested-envelope encoding, 5xx
  `slog` logging including `opctx.RequestID`, no raw-text leak), and the `InvalidInput` field-error
  builder. Full unit-test suite passes; `go build`/`go test`/`make test` pass with no regressions.
  `make lint` failed only on 7 pre-existing gofmt violations outside this task's scope (confirmed
  pre-existing, not introduced by this task).
- **002 — Dogfood-Migrate Mod-Core** (merge `9c9f1a92af6f662124f3a11c68611edfb256234e`). Migrated
  mod-core's own HTTP surface off the local `jsonOK`/`jsonErr`/`writeServiceErr` trio and local sentinel
  set onto `apiresp`: `api/service/errors.go` now aliases `apiresp.ErrNotFound`/`ErrForbidden`/
  `ErrInvalidInput` and adds `ErrConflict`; `api/httpapi/response.go` is reduced to `profileResponse`
  only; all four handler files route success through `apiresp.WriteJSON` and errors through
  `apiresp.WriteError(w, r, err)`, with 401/400 `jsonErr` call-sites brought into design-doc vocabulary
  conformance (`unauthenticated`/`invalid_input`) at unchanged HTTP status. Tests updated to the nested
  envelope; `ErrConflict`→409/`conflict` coverage added. `go build`, `make test`, `make lint` all green
  after the manager cherry-picked the separately-landed gofmt fix onto this branch.

### Phase 02 — GUI Error Widgets (`phase-02-gui-error-widgets`)

- **001 — Client Foundation** (merge `ca9680ed85a7b197483e9f99759216bee77707c1`). Implemented
  `gui/src/lib/api-types.ts` (`FieldError`/`ApiError`/`ApiErrorResponse`) and
  `gui/src/lib/api-client.ts` (`ApiRequestError`, `configureApiClient`/`ApiClientAuthHandler`
  injectable auth seam, `request<T>()` typed fetch wrapper with `network_error`/status-0 synthesis and
  the 401-redirect/403-never-redirect split). Barreled through `gui/src/lib/index.ts` and
  `gui/src/index.ts`. Typecheck and build pass; no new dependency added; `mod-users` untouched.
- **002 — Toast Provider** (merge `3052492afcde7daec34a780b04581ceb473c9632`). Added the Toast provider
  surface: `gui/src/ui/toast.tsx` wraps radix Toast primitives with `cva`/`cn` styling
  (default + destructive variants matching `Alert`); `gui/src/lib/toast-context.tsx` provides
  `ToastProvider` (owns the toast queue, mounts the radix provider + viewport) and `useToast`
  (imperative `toast()`/`dismiss()` API, throws outside the provider). Barrel-exported. No new
  dependency; typecheck and build pass.
- **003 — Error Widgets** (merge `77b69ae457a6619f23aa324f103bef632720cdc5`). Implemented
  `<FieldError>` and `<ErrorBanner>` as presentational-only widgets per the surface-classification
  table. `FieldError` binds one task-001 `FieldError` to an input with `role=alert` and an optional
  `id` for `aria-describedby`, rendering nothing for undefined/null. `ErrorBanner` wraps the existing
  `Alert`/`AlertTitle`/`AlertDescription` (`variant=destructive`) as the mod-core/gui promotion of
  mod-users' `ErrorMessage`, accepting a string, message-bearing object, or title/description pair via
  `ErrorBannerData`. Both exported from `gui/src/index.ts`. No new dependency; typecheck and build pass.
- **004 — useApiError Hook + Barrel** (merge `39839e0a90586bb930cea8b43c08e42fd9b8ecdb`). Implemented
  `useApiError` in `gui/src/lib/use-api-error.ts` as the single place the design doc's
  surface-classification rule lives: `unauthenticated`/null errors surface nothing; `forbidden`/
  `not_found`/`conflict`/`invalid_input` split `details[]` into field-bound (`fieldErrors`) vs
  banner-bound (`bannerError`); everything else (`network_error`, `internal_error`,
  unrecognized/rollback) is toast-dispatched via a `useEffect` keyed on the error instance.
  Whole-package typecheck and build pass; no new dependency; the full Phase-02 export surface
  (`useApiError`, `ToastProvider`, `useToast`, `FieldError`, `ErrorBanner`, `ApiRequestError`,
  `request`) was confirmed reachable from the package root. Closed out Phase 02.

### Phase 03 — Documentation Updates (`phase-03-doc-updates`)

- **001 — Update Architecture Docs** (merge `2404591e6096d3c881f692f20485c9868c07c050`). Updated
  `AGENTS.md` (`apiresp/` row, `service/`/`httpapi/` row updates, a new "gui error/toast toolkit"
  subsection) and `README.md` (brief mentions of `apiresp` and the gui error/toast toolkit) to reflect
  Phases 01–02's new subsystems. `docs/mf-standards/` was left untouched (submodule-owned, out of
  scope, confirmed via `git status`). No repo-owned `docs/architecture.md` exists. Closed out the
  plan's final phase.

All seven tasks across all three phases are marked `done: true`; none are outstanding.

## Key decisions

- **Phase-02 boundary review caught and the manager fixed a CRITICAL bug before the phase gate
  passed.** The `FieldError` wire-type interface (from task 001) shadowed the `<FieldError>` component
  export (from task 003) in the public barrel, making the component unreachable from
  `gui/src/index.ts`. Fixed via a dedicated fix commit
  (`0313398387e2415f24bb04a98019a624b4e01f71`, merged as
  `9001a1095e053fdd17a993b54ed43986618c5764`, branch `2026-07-15-fix-fielderror-collision`) that
  renamed the wire type to `FieldErrorData` across `gui/src/FieldError.tsx`, `gui/src/index.ts`,
  `gui/src/lib/api-client.ts`, `gui/src/lib/api-types.ts`, and `gui/src/lib/use-api-error.ts`, verified
  with a standalone probe importing the built dist barrel to confirm both symbols are now
  independently reachable. This is a load-bearing correction, not a cosmetic rename — without it the
  `<FieldError>` component was not usable by any consumer of the built package. The same fix commit
  also opportunistically addressed two other rough edges surfaced by review: unbound banner detail
  messages are now joined with `'; '` instead of a bare space, and the toast queue is capped at
  `MAX_TOASTS=5` so it can no longer grow unbounded.
- **Phase-03 boundary review caught and the manager fixed a factual doc inaccuracy directly.**
  `AGENTS.md`'s new gui error/toast toolkit subsection (written by task 001, before the phase-02 rename
  landed) still named the wire type `FieldError` rather than the corrected `FieldErrorData`. Fixed
  directly by the manager (commit `85b55307513cbb3e4d1beb2a61dbf268d9a742bd`, "docs: fix
  FieldError/FieldErrorData wire-type name in AGENTS.md") rather than by dispatching a new task,
  correcting both the symbol name and its owning file (`gui/src/lib/api-types.ts`).
- **Dogfooding-first for the Go package.** Rather than shipping `apiresp` as an inert library, task
  001-002 sequencing required mod-core's own HTTP surface to migrate onto it end-to-end (status codes
  and top-level error codes held constant for sentinel-mapped paths) before any other repo is expected
  to adopt it, surfacing real integration friction (e.g. the validation-reason-text tradeoff recorded
  as a follow-up) rather than deferring it. As part of that migration, existing 401/400 call-sites were
  brought into design-doc vocabulary conformance (`unauthenticated`/`invalid_input`) at unchanged HTTP
  status, rather than left on ad hoc strings.
- **GUI client auth is an injectable seam, not a hardcoded strategy.** `configureApiClient`/
  `ApiClientAuthHandler` let a future consumer substitute token storage/retrieval; the default
  implementation intentionally mirrors existing mod-users `localStorage` behavior rather than
  introducing a new strategy speculatively (see Follow-up items).
- **No new dependencies anywhere.** The Toast provider was built entirely on the already-declared
  `radix-ui` primitive; no GUI or Go dependency was added across all seven tasks.

## Follow-up items

`plan/followups.yaml` records five open items (all dated 2026-07-15; none marked as blockers):

- **Service validation-reason text lost on 400s** (`phase-01-apiresp-go`, id `womX`) — Phase-01
  correctness review (minor/medium-confidence): adopting `apiresp.WriteError` drops the specific
  validation-reason text (e.g. "legal_name is required") that existing service-layer
  `fmt.Errorf("%w: ...", ErrInvalidInput)` errors used to surface via `err.Error()` under the old
  `writeServiceErr`. `publicMessage` always substitutes a generic string for `invalid_input`, and these
  errors aren't built via `apiresp.InvalidInput`, so they carry no structured `details[]`. Consistent
  with the design doc's general allowance for generic 4xx messages, but not explicitly scoped by either
  task doc. Decide: accept as a documented tradeoff, or follow up to route
  `api/service/{corporation,natural_person,service_account}.go`'s validation errors through
  `apiresp.InvalidInput(FieldError{...})` so the reason survives in `details[]`. See
  `api/apiresp/writer.go`, `api/httpapi/response.go`.
- **gui/ package has no test runner configured** (`phase-02-gui-error-widgets`, id `vr20`) —
  Pre-existing gap, not introduced by this plan: `gui/` has no vitest/jest dependency or test script,
  though the React Developer role doc calls for component tests as a core responsibility. Sibling tasks
  002/003 shipped without tests since neither task doc's Validation section required them. Decide
  whether/when to add a test runner to `gui/`.
- **Banner drops detail text when all-unbound** (`phase-02-gui-error-widgets`, id `XKQ2`) — Phase-02
  correctness review (minor/medium-confidence): in `useApiError`'s `classifyInline`
  (`gui/src/lib/use-api-error.ts` ~561-567), when `details[]` is non-empty but none match a rendered
  field, `bannerError` falls back to the generic top-level `{code,message}` rather than surfacing the
  specific unbound detail messages — even though the adjacent "some bound, some unbound" branch does
  surface them (and does join them with `'; '` per the phase-02 fix commit). The design doc's Surface
  classification table supports the current literal reading, but the asymmetry may be a product
  surprise. Decide: is generic-message-on-all-unbound intentional, or should it join unbound detail
  messages like the partial-match branch does?
- **request() has no origin check for token** (`phase-02-gui-error-widgets`, id `7VD3`) — Phase-02
  security review (suggestion/medium-confidence): `request()` (`gui/src/lib/api-client.ts` ~106-119)
  attaches the bearer token to whatever URL the caller passes, with no same-origin/allow-listed-host
  check. Not exploitable from anything in this diff (no consumer wired yet) — risk depends entirely on
  how a future consumer calls `request()`. Consider an optional `allowedOrigins`/base-URL guard in
  `configureApiClient` or `request()`, or document the implicit same-origin assumption in the doc
  comment.
- **Default token storage uses localStorage** (`phase-02-gui-error-widgets`, id `pBoG`) — Phase-02
  security review (suggestion/low-confidence): the default `ApiClientAuthHandler`
  (`gui/src/lib/api-client.ts` ~50-60) stores/reads the bearer token via `window.localStorage`,
  readable by any script on the page (XSS-exfiltrable). Intentionally mirrors existing mod-users
  behavior per code comment, not a new regression; the `configureApiClient` seam already lets consumers
  substitute a safer strategy. No action needed now — revisit if/when the project moves toward
  httpOnly-cookie or in-memory-token storage.
