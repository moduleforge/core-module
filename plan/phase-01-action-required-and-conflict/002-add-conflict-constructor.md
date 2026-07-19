# Add Conflict Constructor

## Purpose and scope

Add a public `Conflict(details ...FieldError) error` constructor to `mod-core`'s shared
`api/apiresp` package, mirroring the existing `InvalidInput(details ...FieldError) error`
constructor exactly, so that `WriteError` can fully own conflict-with-details (409) responses. This
folds in mod-users followup `ZVum` (tag `go-apiresp-foundation`): mod-users' `writeServiceError`
currently hand-constructs the 409 envelope and duplicates `apiresp`'s unexported
`publicMessage("conflict")` string verbatim; once this constructor exists, the downstream Wave-1
mod-users plan collapses that onto `apiresp.WriteError(w, r, apiresp.Conflict(details...))`.

New code lives in a new file `api/apiresp/conflict.go` (mirroring `api/apiresp/invalidinput.go`),
with tests in a new `api/apiresp/conflict_test.go`. The single necessary edit to an existing file is
the shared `fieldErrors` detail-recovery accessor in `api/apiresp/invalidinput.go` (see requirement
2). This task does **not** touch the files task 001 creates, so it is parallel-eligible with 001.

Standard Go implementation work — no dedicated skill. Match the existing `invalidinput.go` pattern
exactly (wrapping, `Unwrap`, `errors.Is` behavior, doc-comment style).

## Requirements

### 1. `Conflict` constructor and `conflictError` type (mirror `invalidinput.go`)

In `api/apiresp/conflict.go`, add a `conflictError` type and `Conflict(...)` constructor that mirror
`invalidInputError` / `InvalidInput` one-for-one, but bound to `ErrConflict`:

- `conflictError` carries `details []FieldError`.
- Its `Error()` returns `ErrConflict.Error()` (the safe client-facing message is decided separately
  by `publicMessage`, exactly as documented on `invalidInputError.Error`).
- Its `Unwrap()` returns `ErrConflict`, so `errors.Is(err, ErrConflict)` succeeds for values built by
  `Conflict`.
- `Conflict(details ...FieldError) error` returns `&conflictError{details: details}`. Calling
  `Conflict()` with no details is valid and yields a plain conflict error with no details —
  `WriteError` then omits `error.details` entirely (via `omitempty`), never `null` or `[]`.

Do **not** add or duplicate any message string in `conflict.go`. `publicMessage` already maps the
`"conflict"` code to `"the request conflicts with the current state"` in `writer.go`; that remains
the single source of the conflict message. This is the whole point of the followup — the downstream
consumer stops duplicating that string.

### 2. Extend `fieldErrors` to recover `conflictError`'s details

`WriteError` surfaces field details via the unexported `fieldErrors(err error) ([]FieldError, bool)`
accessor in `invalidinput.go`, which today recovers only `*invalidInputError` via `errors.As`.
Extend it so it also recovers `*conflictError`'s details, so that `WriteError(w, r, Conflict(fe...))`
populates `error.details`.

- **Preferred approach:** introduce a small unexported interface at the point of use and match it
  with `errors.As`, so future detail-carrying errors need no further edits here — e.g.:

  ```go
  type fieldDetailCarrier interface{ fieldDetails() []FieldError }
  ```

  Have both `invalidInputError` and `conflictError` implement `fieldDetails()`, and rewrite
  `fieldErrors` to `errors.As(err, &carrier)`. This follows the Go design standard "define
  interfaces where they are used, keep them small."
- **Acceptable fallback:** add a second `errors.As(err, &ce)` check for `*conflictError` alongside
  the existing `*invalidInputError` check.

Either way, the existing `InvalidInput` behavior and its tests must remain unchanged.

### 3. Doc comments

`conflictError`, its methods, and `Conflict` carry doc comments matching the tone/detail of the
`invalidinput.go` equivalents. Note in `Conflict`'s comment that `WriteError` maps it to
409 `conflict` and surfaces any supplied details.

## Validation

- **Files:** `api/apiresp/conflict.go` and `api/apiresp/conflict_test.go` created;
  `api/apiresp/invalidinput.go` modified (only the `fieldErrors` accessor / added interface); no
  other existing file changed.
- **Build + lint:** `cd api && make lint` passes (go vet + gofmt clean).
- **Tests:** `cd api && make test` passes. New table-driven tests in `conflict_test.go`
  (`package apiresp_test`, mirroring `TestInvalidInput_WithDetails` / `TestInvalidInput_NoDetails`)
  must cover:
  - `errors.Is(Conflict(...), ErrConflict)` is true (both with and without details).
  - `WriteError(rec, req, Conflict(fe...))` → **409**, top-level `error.code: conflict`, non-empty
    `error.message`, and the supplied `details` populated in order and value.
  - `WriteError(rec, req, Conflict())` (no details) → **409**, `error.code: conflict`, and
    `error.details` **absent** from the raw JSON body (not `null`, not `[]`) — assert on raw body
    text as `TestInvalidInput_NoDetails` does.
  - Regression: the existing `TestInvalidInput_*` tests still pass (the `fieldErrors` change must not
    alter `InvalidInput` behavior).

## References

- `api/apiresp/invalidinput.go` — the exact pattern to mirror (`invalidInputError` type, `Error`,
  `Unwrap`, `InvalidInput` constructor, `fieldErrors` accessor).
- `api/apiresp/writer.go` — `WriteError`/`classify`/`publicMessage`; `ErrConflict` already maps to
  409 `conflict` with message "the request conflicts with the current state" (no new message needed).
- `api/apiresp/errors.go` — the `ErrConflict` sentinel.
- `api/apiresp/apiresp_test.go` — `TestInvalidInput_WithDetails` / `TestInvalidInput_NoDetails` are
  the templates for the new conflict tests.
- `docs/mf-standards/architecture/api-response-design.md` — `### Reserved core codes`
  (`conflict` → `ErrConflict` → 409) and the `### Module-specific extension codes` table
  (`last_identity` → top-level `conflict` + `details[].code: users.last_identity`) show the
  downstream shape `Conflict(...)` enables.
- Followup context: mod-users followup `ZVum` (tag `go-apiresp-foundation`).

## Assumptions

- `ErrConflict` and `publicMessage("conflict")` already exist in the package (confirmed in
  `errors.go` / `writer.go`); no sentinel or message additions are needed.
