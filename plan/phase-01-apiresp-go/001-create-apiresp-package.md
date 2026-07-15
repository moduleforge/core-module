# Create Apiresp Package

## Purpose and scope

Create the new shared response/error package `api/apiresp` (import path
`github.com/moduleforge/core-api/apiresp`) that every ModuleForge module will eventually import in
place of its copy-pasted response trio. This task builds the package and its unit tests **only** — it
does not migrate any consumer (mod-core's own migration is task 002; other repos are out of scope).

Source of truth: the **Go-layer ownership** and **GUI-facing error-data contract** sections of
`docs/mf-standards/architecture/api-response-design.md`. Follow the sentinel names, function
signatures, and envelope shape there exactly.

Skill: `implement-task` (Go). No standard skill fully covers this novel package; see `## Procedure`.

## Requirements

Create `api/apiresp/` with the following, all in package `apiresp`:

1. **Sentinel set** (`errors.New` sentinels, one file e.g. `errors.go`):
   ```go
   var (
       ErrUnauthenticated = errors.New("unauthenticated")
       ErrForbidden       = errors.New("forbidden")
       ErrNotFound        = errors.New("not_found")
       ErrInvalidInput    = errors.New("invalid_input")
       ErrConflict        = errors.New("conflict")
   )
   ```
   These are the canonical, importable home. `ErrUnauthenticated` is defined here now (other repos'
   copies are superseded in later, out-of-scope plans). `ErrConflict` is new to the mod-core ecosystem.

2. **Wire/envelope types** (e.g. `types.go`), matching the design doc's nested envelope and the GUI
   wire contract:
   ```go
   type FieldError struct {
       Field   string `json:"field"`
       Code    string `json:"code"`
       Message string `json:"message"`
   }
   type ErrorBody struct {
       Code    string       `json:"code"`
       Message string       `json:"message"`
       Details []FieldError `json:"details,omitempty"` // omitted when empty, never null/[]
   }
   type Envelope struct {
       Error ErrorBody `json:"error"`
   }
   ```
   `details` MUST use `omitempty` so it is absent (not `null`, not `[]`) when there is no per-field
   detail — the design doc's field-semantics table requires this.

3. **`WriteJSON(w http.ResponseWriter, status int, v any)`** — bare success encoder: sets
   `Content-Type: application/json`, writes the status, JSON-encodes `v` as the top-level body (no
   wrapper). Mirrors the current `jsonOK` behavior.

4. **`WriteError(w http.ResponseWriter, r *http.Request, err error)`** — the single sentinel→status/code
   mapper and envelope encoder, per the design doc pseudocode:
   - `classify(err)` uses `errors.Is` against the sentinel set to produce `(code, status)` per the
     [reserved core codes](../../docs/mf-standards/architecture/api-response-design.md) table:
     `ErrUnauthenticated`→(`unauthenticated`,401), `ErrForbidden`→(`forbidden`,403),
     `ErrNotFound`→(`not_found`,404), `ErrInvalidInput`→(`invalid_input`,400),
     `ErrConflict`→(`conflict`,409), and any unrecognized error→(`internal_error`,500).
   - Builds `Envelope{Error: ErrorBody{Code: code, Message: publicMessage(err, code)}}`.
   - Attaches field-level details when `err` carries them (see `InvalidInput` below) via an
     `errors.As`/unwrap check (`fieldErrors(err)`), setting `env.Error.Details`.
   - **`publicMessage` must never leak raw error text on 5xx.** For `internal_error`/500, use a fixed
     generic string (e.g. "an internal error occurred"); the raw `err` is only logged, never returned.
     For the mapped 4xx sentinels, use a safe human-readable summary (a per-code default is acceptable;
     for `invalid_input` the message may summarize validation without echoing internal detail).
   - On `status >= 500`, log server-side with request context:
     `slog.ErrorContext(r.Context(), "request failed", "err", err, ...)`. Include an operation/request
     identifier when available from context — mod-core's `opctx` package exposes `RequestID` (see
     `api/opctx`); use it if reachable without creating an import cycle, otherwise log without it and
     leave a `// TODO` note. Do NOT hard-depend on any handler package.
   - Delegates the actual write to `WriteJSON(w, status, env)`.

5. **`InvalidInput(details ...FieldError) error`** — builds an error that (a) satisfies
   `errors.Is(err, ErrInvalidInput)` and (b) carries the supplied `FieldError` details so `WriteError`
   can surface them in `env.Error.Details`. Implement via a small struct type that wraps
   `ErrInvalidInput` (its `Unwrap()` returns `ErrInvalidInput`) and holds `[]FieldError`, plus the
   `fieldErrors(err)` accessor `WriteError` uses (`errors.As` to the concrete type). Calling
   `InvalidInput()` with no details is valid and yields a plain invalid-input error with empty details.

6. **Package doc comment** on the package clause summarizing its role as the shared response/error
   contract owner.

7. **Unit tests** (`*_test.go`, package `apiresp` or `apiresp_test`) covering:
   - Each sentinel → correct (status, code) via `WriteError`, asserting the **nested** envelope shape
     (`body.error.code`, `body.error.status` mapping) and Content-Type.
   - Unmapped/wrapped errors → 500 `internal_error` with the generic message and **no raw error text**
     in the body (assert the raw message does not appear).
   - Wrapped sentinels (`fmt.Errorf("...: %w", ErrNotFound)`) still classify correctly (errors.Is).
   - `InvalidInput(fe...)` → 400 `invalid_input` with `details` populated in the envelope; and the
     `details` key is **absent** when there are no details (omitempty).
   - `WriteJSON` writes a bare body (no `error` wrapper) with the given status and Content-Type.
   - A 5xx path exercises `WriteError` with a non-nil `*http.Request` (use `httptest.NewRequest`) so the
     `slog` call has a real context and does not panic.

Do not edit any file outside `api/apiresp/`. Do not add new module dependencies (standard library
`errors`, `encoding/json`, `net/http`, `log/slog` only; `opctx` is an intra-module import if used).

## Validation

- `api/apiresp/` exists with the sentinel, types, and writer files plus `*_test.go`.
- `cd api && go build ./...` succeeds.
- `cd api && go test ./apiresp/...` passes; `cd api && make test` passes (no regressions elsewhere).
- `cd api && make lint` (go vet + gofmt) is clean.
- Grep confirms all five sentinels and the four exported functions/types exist:
  `grep -n "ErrUnauthenticated\|ErrForbidden\|ErrNotFound\|ErrInvalidInput\|ErrConflict\|func WriteJSON\|func WriteError\|func InvalidInput\|type FieldError" api/apiresp/*.go`.
- Confirm `details` uses `omitempty` and no test observes a `null`/`[]` details value.
- Confirm no `WriteError` 5xx path returns raw `err.Error()` text in the response body (test asserts
  absence).

## Metadata

architectural_impact: true

## References

- `docs/mf-standards/architecture/api-response-design.md` — **Go-layer ownership** section (sentinel
  set, `WriteJSON`/`WriteError`/`InvalidInput` signatures and pseudocode) and **Error-code vocabulary**
  / **HTTP status mapping** sections (the reserved code ↔ sentinel ↔ status table). Not visible via the
  submodule pointer in-repo; also at
  `/Users/zane/playground/moduleforge/docs-mf-standards/architecture/api-response-design.md`.
- `api/httpapi/response.go` — the current `jsonOK`/`jsonErr`/`writeServiceErr` trio the new package
  replaces (behavioral reference for `WriteJSON` and the sentinel mapping).
- `api/service/errors.go` — mod-core's current sentinel set (missing `ErrConflict`).
- `api/opctx/` — typed context accessors (`RequestID`) for the 5xx logging context.
- `AGENTS.md` — build/test/lint commands and conventions.

## Checkpoint hints

- After the sentinels + envelope/`FieldError` types compile.
- After `WriteJSON` + `WriteError` (with `classify`/`publicMessage`/5xx logging).
- After `InvalidInput` + `fieldErrors` accessor.
- After the unit-test suite passes.
