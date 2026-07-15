# Alias Entity Resolver Errors To Apiresp Sentinels

## Purpose and scope

Fix followup `mJ5k`: `api/entity/resolver.go`'s `ErrForbidden`/`ErrNotFound` are currently
package-local `errors.New(...)` values, distinct from `apiresp.ErrForbidden`/`apiresp.ErrNotFound`
with no `Unwrap()` relating them. Because `apiresp.WriteError`'s `classify()`
(`api/apiresp/writer.go`) matches only via `errors.Is` against apiresp's own five sentinels, a
masked entity-lookup miss returned through any of the five `entityResolver.Resolve(...)` call
sites in `api/service/*.go` currently falls through to `classify()`'s default case —
`internal_error`/500 — instead of the documented `forbidden`/403 (or `not_found`/404 for a
resource that has opted into `AllowNotFound`).

This task applies the fix at its source — `api/entity/resolver.go` — per the exact shape already
prescribed in `docs/mf-standards/architecture/api-response-design.md`'s "Go-layer ownership"
section ("How this subsumes the per-module shims"):

> `normalizeEntityResolveErr` disappears. The `core-api/entity` resolver returns the canonical
> `apiresp.ErrForbidden` / `apiresp.ErrNotFound` (per the masking policy) directly, and
> `WriteError` recognises them via `errors.Is`. There is nothing left to normalise.

No standard skill applies; this is a small, targeted Go bugfix plus unit-test tightening. Follow
the repo's existing conventions (see References) rather than inventing new patterns.

## Requirements

1. **`api/entity/resolver.go`**:
   - Add the `"github.com/moduleforge/core-api/apiresp"` import.
   - Change `var ErrForbidden = errors.New("entity: forbidden")` to
     `var ErrForbidden = apiresp.ErrForbidden`.
   - Change `var ErrNotFound = errors.New("entity: not found")` to
     `var ErrNotFound = apiresp.ErrNotFound`.
   - Update the doc comments on both vars (currently describing them as distinct
     `entity`-package errors) to reflect that they are now aliases of the canonical apiresp
     sentinels — keep the existing masking-policy explanation (default-403-on-missing,
     `AllowNotFound` opts a resource into 404), just correct the "this is a package-local error"
     framing.
   - Do not remove the `"errors"` import — `errors.Is(err, pgx.ErrNoRows)` in `Resolve` still uses
     it.
   - Verify no import cycle: confirm `api/apiresp/*.go` does not import `api/entity` (it does
     not — apiresp imports only `api/opctx`).
2. **Do not touch `api/service/*.go` or `api/httpapi/*.go` non-test files.** The alias fix at the
   source is sufficient to correct all five call sites (`EntityService.GetByUUID`,
   `EntityService.ResolveProfile`, `CorporationService.GetByEntityUUID`,
   `NaturalPersonService.GetByEntityUUID`, `ServiceAccountService.GetByEntityUUID`) — each already
   returns the resolver's error verbatim, so once `entity.ErrForbidden` / `entity.ErrNotFound`
   *are* `apiresp.ErrForbidden` / `apiresp.ErrNotFound` by identity, every handler's existing
   `apiresp.WriteError(w, r, err)` call classifies correctly with no further code change.
3. **Tighten/add unit-test assertions at each of the five call sites** so the test suite would
   have caught this regression (all in package `service` unless noted):
   - `api/service/entity_test.go`:
     - `TestEntityService_GetByUUID_NotFound` (~line 90-100): the existing assertion
       `errors.Is(err, entity.ErrForbidden)` still passes under the alias (identity-preserving)
       — leave it, but add a second assertion `errors.Is(err, ErrForbidden)` (the `service`
       package's own apiresp-aliased sentinel, per `api/service/errors.go`) in the same test, to
       lock in that the error now also satisfies the boundary the fix targets. Update the
       failure-message text if you add a second `t.Errorf` so it's clear which check failed.
     - Add a new test `TestEntityService_ResolveProfile_NotFound` (there is currently no
       not-found coverage for `ResolveProfile` at all — only the happy-path
       `TestEntityService_ResolveProfile`). Mirror `TestEntityService_GetByUUID_NotFound`'s
       structure: build via `newEntityService(q)`, call `svc.ResolveProfile(context.Background(),
       q, randomUUID(t))`, assert `errors.Is(err, ErrForbidden)`.
   - `api/service/corporation_test.go`: `TestCorporationService_GetByEntityUUID_NotFound`
     (~line 90-98) currently only asserts `err == nil` fails. Strengthen it to assert
     `errors.Is(err, ErrForbidden)` (add the `errors` import check — already imported per file's
     other tests; verify).
   - `api/service/natural_person_test.go`: `TestNaturalPersonService_GetByEntityUUID_NotFound`
     (~line 216-224) — same strengthening: assert `errors.Is(err, ErrForbidden)`.
   - `api/service/extra_test.go`: `TestServiceAccountService_GetByEntityUUID_NotFound`
     (~line 47-55) — same strengthening: assert `errors.Is(err, ErrForbidden)`.
4. **`api/entity/resolver_test.go`** already asserts `errors.Is(err, entity.ErrForbidden)` /
   `entity.ErrNotFound` in `TestResolver_Resolve_NotFound_DefaultForbidden` and
   `TestResolver_Resolve_NotFound_OptedIn404` — these remain valid as-is (identity-preserving
   alias). Add one new assertion in each of those two tests confirming the alias explicitly:
   `errors.Is(err, apiresp.ErrForbidden)` and `errors.Is(err, apiresp.ErrNotFound)` respectively
   (new `"github.com/moduleforge/core-api/apiresp"` import in this test file). This is the
   regression-proofing assertion for the exact bug this task fixes — without it, a future
   accidental revert of the alias would still pass the pre-existing `entity.Err*` identity checks
   (since `entity.ErrForbidden` would still equal itself) but silently reintroduce the
   apiresp-classification gap.

## Validation

- `cd api && make lint` — `go vet ./...` and `gofmt` check pass with no new findings.
- `cd api && go build ./...` — compiles cleanly (confirms no import cycle from `entity` →
  `apiresp`).
- `cd api && make test` (`go test ./...`) — full suite passes, including:
  - `api/entity` package: `TestResolver_Resolve_NotFound_DefaultForbidden`,
    `TestResolver_Resolve_NotFound_OptedIn404`, and the two new `apiresp.Err*` assertions added to
    them.
  - `api/service` package: the four strengthened `errors.Is(err, ErrForbidden)` assertions
    (`entity_test.go`'s `GetByUUID`, `corporation_test.go`, `natural_person_test.go`,
    `extra_test.go`), and the new `TestEntityService_ResolveProfile_NotFound`.
- Manual check: `grep -n "entity: forbidden\|entity: not found" api/entity/resolver.go` returns no
  matches (the old distinct error-text literals are gone).
- Manual check: `grep -rn "entity\.ErrForbidden\|entity\.ErrNotFound" api/` still shows only the
  test-file references noted above (`resolver_test.go`, `entity_test.go`) — confirming no
  non-test code depended on the two sentinel sets being distinct, and none was introduced by this
  change.
- Confirm `go.mod`/`go.sum` are unaffected (no new external dependency — `apiresp` is an in-module
  package).

## Assumptions

- `api/apiresp` importing into `api/entity` is architecturally acceptable: `api/entity` is
  described in `AGENTS.md` as carrying "service-layer types," and `api/service/errors.go` already
  establishes the precedent of aliasing local sentinels onto `apiresp`'s. This task extends the
  same precedent one layer down, matching the design doc's explicit prescription quoted above.
- No resource in this repo currently calls `entity.Resolver.AllowNotFound(...)` (confirmed via
  repo-wide grep — the only call sites are in test files); the `ErrNotFound` alias is exercised
  today only by `api/entity/resolver_test.go`'s `OptedIn404` test and is otherwise dormant in
  production wiring (a future consumer module opting a resource in via `AllowNotFound` will now
  get the correctly-mapped `404 not_found` for free).

## References

- `api/entity/resolver.go` — the fix target.
- `api/apiresp/errors.go` — the canonical sentinel set (`ErrForbidden`, `ErrNotFound`, etc.) and
  its doc comment pointing to the design doc as source of truth.
- `api/apiresp/writer.go` — `classify()`, the mapper this fix feeds into; not modified by this
  task.
- `api/service/errors.go` — the existing precedent for aliasing package-local sentinels onto
  `apiresp`'s (`var ErrForbidden = apiresp.ErrForbidden`, etc.).
- `docs/mf-standards/architecture/api-response-design.md` — "Existence masking (`not_found` vs
  `forbidden`)" and "Go-layer ownership" / "How this subsumes the per-module shims" sections are
  the authoritative source for this fix's shape. Read-only reference — do not edit (submodule,
  separate repo).
- `api/entity/resolver_test.go`, `api/service/entity_test.go`, `api/service/corporation_test.go`,
  `api/service/natural_person_test.go`, `api/service/extra_test.go` — existing test fixtures
  (`testEntityResolver()`, `newEntityService`, `newCorpService`, `newNPService`, `newSAService`,
  `randomUUID`, `mockQuerier`) to reuse; do not duplicate fixture helpers.
- `plan/followups.yaml` item `mJ5k` — the source finding this task addresses.
