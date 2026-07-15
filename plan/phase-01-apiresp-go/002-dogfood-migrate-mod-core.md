# Dogfood-Migrate mod-core

## Purpose and scope

Migrate mod-core's **own** HTTP surface off the ad-hoc `jsonOK`/`jsonErr`/`writeServiceErr` trio and
off its standalone sentinel set onto the new `api/apiresp` package built in task 001. This proves the
package end-to-end inside mod-core before any other repo depends on it. Scope is mod-core's `api/`
module only.

Depends on: `phase-01-apiresp-go/001-create-apiresp-package.md`.

Skill: `implement-task` (Go).

## Requirements

1. **Point `api/service/errors.go` at the apiresp sentinels and add `ErrConflict`.** Replace the three
   local `errors.New` sentinels with aliases to the canonical ones so existing service code that
   returns/compares `service.ErrNotFound` etc. keeps working and `apiresp.WriteError`'s `errors.Is`
   matches them:
   ```go
   package service
   import "github.com/moduleforge/core-api/apiresp"
   var (
       ErrNotFound     = apiresp.ErrNotFound
       ErrForbidden    = apiresp.ErrForbidden
       ErrInvalidInput = apiresp.ErrInvalidInput
       ErrConflict     = apiresp.ErrConflict // newly available to mod-core
   )
   ```
   Keep the doc comments. `ErrConflict` is now exposed to mod-core's service layer; no service method
   is required to return it yet, but it must exist and map correctly.

2. **Replace the `api/httpapi/response.go` trio.** Remove `jsonOK`, `jsonErr`, and `writeServiceErr`;
   route through `apiresp`:
   - Success writes (`jsonOK(w, status, body)`) → `apiresp.WriteJSON(w, status, body)`.
   - Service-error writes (`writeServiceErr(w, err)`) → `apiresp.WriteError(w, r, err)`. Note the new
     `*http.Request` argument — the calling handlers already have `r` in scope; thread it through.
   - Keep `profileResponse` (unchanged; it only shapes a map).

3. **Update handler call-sites** in `api/httpapi/{entities,corporations,natural_persons,service_accounts}.go`.
   Each currently calls the local trio directly. Migrate them:
   - Bare success: `jsonOK(...)` → `apiresp.WriteJSON(...)`.
   - `writeServiceErr(w, err)` → `apiresp.WriteError(w, r, err)`.
   - The ad-hoc `jsonErr` call-sites: bring these into conformance with the design doc's vocabulary
     while preserving their HTTP status:
     - Missing-actor 401 (`jsonErr(w, http.StatusUnauthorized, "unauthorized", ...)`) →
       `apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)` (status 401 unchanged; top-level code
       becomes `unauthenticated`, the canonical 401 code).
     - Bad JSON body / bad UUID 400 (`jsonErr(w, http.StatusBadRequest, "bad_request", ...)`) →
       `apiresp.WriteError(w, r, apiresp.ErrInvalidInput)` (status 400 unchanged; top-level code
       becomes `invalid_input`, the canonical 400 code). Use `apiresp.InvalidInput(...)` only if a
       field-level detail is warranted; a plain `ErrInvalidInput` is sufficient here.
   - **Rationale / behavioral note:** no existing test asserts the `unauthorized`/`bad_request` code
     *strings* (they assert only HTTP status — verified: `handlers_create_test.go`,
     `handlers_extra_test.go`), so these code renamings bring mod-core into design-doc conformance
     without breaking tests. If, contrary to this expectation, a test does assert the old code string,
     update it to the canonical code and note it in the task report.

4. **Update `api/httpapi` tests to the nested envelope.** The response envelope changes from flat
   `{"error":"<code>","message":"<msg>"}` to nested `{"error":{"code","message","details?"}}`:
   - `handlers_test.go`'s `TestWriteServiceErr_MapsCorrectly` (currently decodes
     `map[string]string` and asserts `body["error"] == wantCode`) must be updated to decode the nested
     shape and assert `body.error.code == wantCode`. Since `writeServiceErr` is removed, retarget the
     test to `apiresp.WriteError` (passing an `httptest.NewRequest`) or fold its intent into the
     apiresp package tests and delete the mod-core copy — keep equivalent coverage either way.
   - **Status/top-level-code mapping for the sentinel-mapped path must remain unchanged**:
     `ErrNotFound`→404 `not_found`, `ErrForbidden`→403 `forbidden`, `ErrInvalidInput`→400
     `invalid_input`.
   - Any other test that decodes an error body must be updated to the nested shape.

5. **Add `ErrConflict` coverage.** Add a test asserting `apiresp.ErrConflict` (or `service.ErrConflict`)
   → 409 `conflict` via `apiresp.WriteError` in mod-core's suite (or confirm it is covered by the
   apiresp package tests and add a mod-core-level assertion if the mapping is exercised through a
   handler path). At minimum, mod-core's tests must exercise the 409/`conflict` mapping.

## Validation

- `grep -rn "jsonOK\|jsonErr\|writeServiceErr" api/httpapi/` returns **no** matches outside comments/
  history (the trio is fully removed).
- `api/httpapi/response.go` no longer defines the trio; success/error writes go through `apiresp`.
- `api/service/errors.go` aliases the apiresp sentinels and defines `ErrConflict`.
- `cd api && go build ./...` succeeds.
- `cd api && make test` passes — all `httpapi` tests green, including the updated nested-envelope
  assertions and the new `ErrConflict` → 409 `conflict` coverage.
- `cd api && make lint` (go vet + gofmt) is clean.
- Manual spot check: an error response body is now nested (`{"error":{"code":...}}`), and a 500 path
  returns the generic message with no raw error text.

## Metadata

architectural_impact: true

## Assumptions

- Task 001 has landed: `api/apiresp` exists with the sentinels, `WriteJSON`, `WriteError`, and
  `InvalidInput`.
- No existing test asserts the literal `unauthorized`/`bad_request` top-level code strings (only HTTP
  status). Verified at plan time against `handlers_create_test.go` and `handlers_extra_test.go`; if a
  hidden assertion surfaces, update it to the canonical code and report.

## References

- `docs/mf-standards/architecture/api-response-design.md` — **Go-layer ownership** ("How this subsumes
  the per-module shims", thin-handlers) and **Error-code vocabulary** (`invalid_input` not
  `bad_request`; `unauthenticated` not `unauthorized`).
- `api/httpapi/response.go` — the trio to remove; `profileResponse` to keep.
- `api/httpapi/{entities,corporations,natural_persons,service_accounts}.go` — handler call-sites to
  migrate.
- `api/httpapi/handlers_test.go` (`TestWriteServiceErr_MapsCorrectly`, ~L152–181),
  `handlers_create_test.go`, `handlers_extra_test.go`, `taxid_test.go` — tests to update to the nested
  envelope.
- `api/service/errors.go` — sentinel set to re-home onto apiresp + add `ErrConflict`.
- `plan/phase-01-apiresp-go/001-create-apiresp-package.md` — the package this task consumes.

## Checkpoint hints

- After `api/service/errors.go` aliases apiresp + adds `ErrConflict` and the module still builds.
- After `api/httpapi/response.go` is reduced to `profileResponse` and delegates to `apiresp`.
- After migrating each handler file's call-sites.
- After updating the tests to the nested envelope and adding `ErrConflict` coverage; full suite green.
