# API Response & Error Widgets (Wave 0 / Phase 2)

## Purpose and scope

This plan implements **phase 2** of the previously-planned three-phase API response & error
standardization effort. Phase 1 (design + documentation, no code) is complete and merged; the settled
contract lives at `docs/mf-standards/architecture/api-response-design.md` (submodule-mounted from the
`docs-mf-standards` repo) and is the source of truth for shapes, sentinel names, function signatures,
and error-code vocabulary.

Scope is **mod-core only** (this repo). The plan builds two deliverables the design doc specifies as
"phase 2, design only" today, and dogfoods the Go one within mod-core:

1. **Go — `api/apiresp` package** (new, import path `github.com/moduleforge/core-api/apiresp`): the
   canonical sentinel set, a bare success encoder (`WriteJSON`), the sentinel→status/code mapper and
   nested-envelope encoder (`WriteError`), a field-error builder (`InvalidInput`), and the `FieldError`
   wire type. Plus a **dogfooding migration** of mod-core's own `api/httpapi/response.go` trio
   (`jsonOK`/`jsonErr`/`writeServiceErr`) and `api/service/errors.go` sentinels onto the new package,
   proving it end-to-end before any other repo depends on it.

2. **GUI — `mod-core/gui` error/toast widgets**: wire types + `ApiRequestError` class + a shared typed
   `request()` fetch wrapper (promoted into `mod-core/gui` for other modules to adopt later), a
   `<FieldError>` widget, an `<ErrorBanner>` widget (wrapping the existing `Alert` `destructive`
   variant), a Toast provider + `useToast` hook (built on the already-present `radix-ui` Toast
   primitive), and a `useApiError(error)` hook implementing the design doc's surface-classification
   table. All exported from the `gui/src/index.ts` barrel.

### Success criteria

- `api/apiresp` exists with the exact sentinel set, function signatures, and nested-envelope shape the
  design doc's [Go-layer ownership](../docs/mf-standards/architecture/api-response-design.md) section
  specifies, including `ErrConflict` (which mod-core's current `api/service/errors.go` lacks).
- mod-core's own HTTP surface emits the nested `{error:{code,message,details}}` envelope via `apiresp`;
  `cd api && make test` passes, with **status codes and top-level error codes unchanged for the
  sentinel-mapped path**, plus new coverage for the `ErrConflict` → 409 `conflict` case.
- `mod-core/gui` exports the full widget/hook/client set; `cd gui && bun run typecheck` (tsc --noEmit)
  passes; no new external dependency is added (Toast uses the existing `radix-ui`).

### What must NOT change

- `model/` (separate Go module) — untouched.
- The design doc itself — no doc-only patches (out of scope).
- Any **other** repo's response writer, sentinels, or GUI components — those are 6 separate follow-on
  plans, out of scope here.
- `mod-users/gui/src/lib/api.ts` — not in this repo; the mod-core wire types are a superset-compatible
  extension of it, but that file is not touched.
- No app regeneration.

### Hard constraints

- `api/` and `model/` are separate Go modules (see `AGENTS.md`); `apiresp` is new code under `api/`.
- `gui/` uses tsup + Bun; typecheck via `tsc --noEmit`. Toast must use the already-declared `radix-ui`
  dependency — no new library.
- Follow mod-core conventions already in `AGENTS.md`: thin handlers, internal IDs never exposed,
  authorization-first.

## Current status

Plan created; no tasks started. **Phase 01 (apiresp-go) and Phase 02 (gui-error-widgets) are fully
independent and may run concurrently** — the GUI consumes the *wire contract* (fully specified in the
design doc), not the Go code, so neither phase blocks the other. Phase 03 (doc-updates) runs last,
after both implementation phases land.

Pre-conditions/notes at creation time:

- **The design doc is not visible through the mod-core submodule pointer.** `docs/mf-standards` is
  pinned at `094e30d…`, which predates `api-response-design.md`; the doc is merged on `main` of the
  standalone `docs-mf-standards` repo (also checked out at
  `/Users/zane/playground/moduleforge/docs-mf-standards/architecture/api-response-design.md`). This
  does not block implementation (the contract is captured in this overview and the task docs), but a
  submodule-pointer bump would be needed for an in-repo agent to read the doc at the documented path.
- No repo-owned `docs/architecture.md` or `docs/*-spec.md` exists in mod-core; the only architecture
  docs live in the out-of-scope `docs/mf-standards` submodule. The doc-updates phase therefore targets
  the repo-owned `AGENTS.md` (package table) and `README.md`.

## Overview

### Phase 01 — apiresp (Go) · `phase-01-apiresp-go`

Builds and dogfoods the shared Go response/error package.

- **001 — Create Apiresp Package** (`sonnet-high`). New `api/apiresp` package: sentinels
  (`ErrUnauthenticated`, `ErrForbidden`, `ErrNotFound`, `ErrInvalidInput`, `ErrConflict`), `FieldError`
  type, `Envelope`/`ErrorBody` types, `WriteJSON`, `WriteError` (errors.Is classification, nested
  envelope, 5xx `slog` logging with request context, no raw-text leak), `InvalidInput(...)` builder.
  Full package unit tests. No consumer changes yet.
- **002 — Dogfood-Migrate mod-core** (`sonnet-high`, depends on 001). Point `api/service/errors.go` at
  the apiresp sentinels and add `ErrConflict`; replace the `api/httpapi/response.go` trio and handler
  call-sites with `apiresp.WriteJSON`/`apiresp.WriteError`; update `httpapi` tests to the nested
  envelope shape (status + top-level codes unchanged for sentinel-mapped cases) and add `ErrConflict`
  coverage.

### Phase 02 — GUI error widgets · `phase-02-gui-error-widgets` (parallel with Phase 01)

Builds the mod-core/gui error-surface toolkit against the design doc's GUI-facing contract.

- **001 — Client Foundation** (`sonnet-high`). Wire types (`FieldError`, `ApiError`,
  `ApiErrorResponse`), `ApiRequestError` class (`code`/`status`/`details`), and the shared typed
  `request()` fetch wrapper: `network_error`/status-0 synthesis on transport failure, and the
  401-triggers-redirect / 403-never-redirects split (`skipAuthRedirect` escape hatch).
- **002 — Toast Provider** (`sonnet-high`, parallel with 001). Toast provider + `useToast` hook built
  on the existing `radix-ui` Toast primitive. No new dependency.
- **003 — Error Widgets** (`sonnet-med`, depends on 001). `<FieldError>` (binds one `FieldError` to an
  input, inline) and `<ErrorBanner>` (wraps the existing `Alert` `destructive` variant).
- **004 — useApiError Hook + Barrel** (`sonnet-high`, depends on 001, 002, 003). `useApiError(error)`
  implementing the surface-classification table (field vs banner vs toast-worthy); returns
  `{ fieldErrors, bannerError }` and dispatches toast-worthy errors to the provider. Finalizes the
  `gui/src/index.ts` barrel exports and verifies the whole-package typecheck.

### Phase 03 — Documentation Updates · `phase-03-doc-updates`

- **001 — Update Architecture Docs** (`sonnet-high`, after Phases 01–02). Update the repo-owned
  `AGENTS.md` package table (add `apiresp`; note the GUI error/toast subsystem) and `README.md`
  provides list to reflect the new subsystems. Records explicitly that the canonical architecture doc
  and the design doc live in the out-of-scope `docs/mf-standards` submodule and are not modified here.

### Parallelism summary

- Phase 01 and Phase 02 are independent (run concurrently).
- Within Phase 01: 001 → 002 (sequential).
- Within Phase 02: 001 and 002 are parallel; 003 depends on 001; 004 depends on 001+002+003.
- Phase 03 depends on Phases 01 and 02 completing.
