# Add Masked-Miss Httpapi Integration Test

## Purpose and scope

Depends on `phase-01-masked-lookup-fix/001-alias-entity-resolver-sentinels.md` — this task's new
test must run against the fixed code (dispatch after task 1 lands; do not attempt to land this
task's test against the pre-fix code, it will fail red).

Followup `mJ5k` explicitly calls out that the only existing coverage of the masked-miss path was
service-layer `errors.Is` assertions (weak or absent, per task 1), and that "no httpapi-layer or
integration test was found asserting a masked-missing-UUID request actually yields HTTP 403
forbidden through the full `apiresp.WriteError` chain." This task closes that specific gap.

**Why the existing `api/httpapi/*_test.go` tests don't already cover this**: every current httpapi
test builds its router via `buildTestDeps(...)`, which injects `fake*Service` implementations
(`api/httpapi/mock_test.go`) that return a caller-supplied `err` field directly —
e.g. `&fakeEntityService{err: service.ErrForbidden}`. These fakes bypass the real
`EntityService`/`entity.Resolver` entirely, so they cannot exercise (and could not have caught)
the bug task 1 fixed. This task adds a test that wires the **real** `service.EntityService` (via
`service.New`, with a real `entity.NewResolver()`) into the router instead of a fake, so the HTTP
response reflects what the real resolver→service→`WriteError` chain actually produces.

No standard skill applies; this is a new Go test file following existing httpapi-package test
conventions (see References).

## Requirements

1. Add a new test file `api/httpapi/masked_lookup_test.go` (package `httpapi`) containing:
   - A minimal stub `coredb.Querier` implementation (mirror the shape of
     `api/entity/resolver_test.go`'s `resolverStubQuerier`, which already implements all 18
     `coredb.Querier` methods with only `GetEntityByUUID` behavior configurable — do not
     reimplement the interface from scratch if a closer-to-hand pattern exists, but note
     `resolverStubQuerier` lives in `package entity_test` and cannot be imported directly; write a
     package-local equivalent, e.g. `stubQuerier`, with a configurable `getEntityErr error` field
     returned by `GetEntityByUUID`, all other 17 methods returning zero-value + nil (they are
     never invoked on the code path this test exercises)).
   - A minimal stub `authz.Authorizer` (single-method interface — see `api/authz/authz.go`) whose
     `Authorize` always returns `nil`. (It will not actually be reached on the masked-miss path,
     since `EntityService.GetByUUID`/`ResolveProfile` call the resolver before authorization, but
     `service.New` requires a non-nil `authz.Authorizer` to type-check the aggregate correctly and
     a real request that *does* resolve should not spuriously fail authz.)
   - A test `TestGetEntity_MaskedMiss_Returns403Forbidden` that:
     - Constructs `q := &stubQuerier{getEntityErr: pgx.ErrNoRows}` (import
       `"github.com/jackc/pgx/v5"` for `pgx.ErrNoRows`).
     - Builds `svcs := service.New(q, nil, <stub authorizer>{}, observer.NewObserverGroup(), nil,
       entity.NewResolver(), nil)` — `db` (`txhelper.DB`) and `cipher`
       (`*fieldcrypto.Cipher`) and `typeResolver` (`*types.Resolver`) may all be `nil`: the
       masked-miss path through `EntityService.GetByUUID`/`ResolveProfile` never dereferences
       them (confirmed by reading `api/service/entity.go` and `api/service/service.go`'s `New`).
     - Builds `d := Deps{Services: svcs, Logger: noopLogger()}` and `router := NewRouter(d)`
       (reuse the package's existing `noopLogger()` helper from `mock_test.go`).
     - Sends `GET /entities/{random-uuid}` via `httptest.NewRequest` +
       `withActor(req, 1)` (reuse the package's existing `withActor` helper) through
       `router.ServeHTTP`.
     - Asserts `rec.Code == http.StatusForbidden` (403).
     - Decodes the response body against the nested envelope shape (`{"error":{"code":...,
       "message":...}}` — see `TestServiceErr_MapsCorrectly` in `handlers_test.go` for the decode
       pattern) and asserts `Error.Code == "forbidden"`.
   - A second test, `TestGetEntity_MaskedMiss_Returns403NotInternalError`, is optional — the
     single assertion above (status + code) already fully proves the fix; do not pad the file with
     redundant cases. If you do add more cases, prefer one proving the *success* path still works
     with the same stub-backed real service (i.e. `stubQuerier` returns a well-formed
     `GetEntityByUUIDRow` for a known UUID and the response is 200) as a sanity check that the stub
     wiring itself is correct and not accidentally forcing every request down the error path.
2. Do not modify `api/httpapi/mock_test.go`'s existing `fake*Service` types or any existing test —
   this task is additive only.
3. Do not attempt to wire `NaturalPersonService`/`CorporationService`/`ServiceAccountService`
   through the real chain in this task — their `GetByEntityUUID` methods require a populated
   `*fieldcrypto.Cipher` and `*types.Resolver` to fully exercise (unlike `EntityService`, whose
   masked-miss path needs neither), which would meaningfully increase this task's scope for
   marginal additional proof: the fix in task 1 is a single alias applied once in
   `api/entity/resolver.go`, so one full-chain proof (`EntityService.GetByUUID`/`ResolveProfile`
   via `GET /entities/{uuid}`) is sufficient evidence the mechanism works generally; the other four
   call sites already have service-layer `errors.Is(err, ErrForbidden)` coverage from task 1. If,
   while implementing, you find `EntityService`'s path does *not* fully exercise the fix (e.g. it
   turns out to need a non-nil cipher or typeResolver you didn't anticipate), flag this in your
   report rather than silently expanding scope.

## Validation

- `cd api && go build ./...` — compiles cleanly.
- `cd api && go test ./httpapi/... -run TestGetEntity_MaskedMiss -v` — new test passes.
- `cd api && make test` — full suite passes (no regressions in existing httpapi tests).
- `cd api && make lint` — `go vet ./...` + `gofmt` check pass.
- Manual confirmation the new test would have failed against the pre-task-1 code: temporarily
  `git stash` task 1's change (or reason through it without actually reverting) and confirm the
  new test's assertion (`rec.Code == http.StatusForbidden`) would instead see `500` — do not leave
  the repo in a stashed/reverted state; this is a design-verification step, not a required CI
  artifact. State in your report whether you performed this check and what you observed.

## Assumptions

- Task 1 (`001-alias-entity-resolver-sentinels.md`) has already landed in this plan branch when
  this task is dispatched — the new test is written to pass against the fixed
  `api/entity/resolver.go`, not the pre-fix code.
- `service.New`'s `db`, `cipher`, and `typeResolver` parameters can be `nil` for this test's
  specific code path (masked-miss through `EntityService.GetByUUID`/`ResolveProfile`) without
  panicking — verify this assumption holds during implementation (read
  `api/service/entity.go`'s `GetByUUID`/`ResolveProfile` bodies and `api/service/service.go`'s
  `New` to confirm neither is dereferenced on this path) and flag in your report if it does not.

## Status

- **Outcome:** succeeded
- **Date:** 2026-07-15
- **Validation summary:** `go build ./...` clean; `go test ./httpapi/... -run TestGetEntity_MaskedMiss -v` passes
  (`TestGetEntity_MaskedMiss_Returns403Forbidden`); `make test` passes for the full `api` module with
  no regressions; `make lint` (`go vet ./...` + `gofmt -l .`) passes clean.
- **Affected files:** `api/httpapi/masked_lookup_test.go` (new).
- **Assumptions applied:** confirmed by reading `api/service/entity.go` and `api/service/service.go`
  that `service.New`'s `db`, `cipher`, and `typeResolver` parameters are never dereferenced on the
  masked-miss path through `EntityService.GetByUUID`/`ResolveProfile` (the resolver returns
  `apiresp.ErrForbidden` before authz or any subtype lookup runs) — the test passes `nil` for all
  three without panicking, confirming the assumption held.
- **Scope note:** `EntityService`'s path fully exercised the fix as anticipated; no expansion beyond
  the single required test was needed. The optional second (success-path sanity) test was not added,
  per the task doc's guidance not to pad the file with redundant cases — the single 403/`forbidden`
  assertion already fully proves the fix.
- **Pre-fix regression check:** performed via code-reading rather than an actual `git stash` (per the
  task doc's "or reason through it without actually reverting" option). `git show 2af5ce5 --
  entity/resolver.go` shows the pre-task-1 sentinels were distinct local values
  (`errors.New("entity: forbidden")` / `errors.New("entity: not found")`), not aliased to
  `apiresp.ErrForbidden`/`apiresp.ErrNotFound`. `apiresp.classify` (in `apiresp/writer.go`) dispatches
  purely via `errors.Is` against the `apiresp` sentinels, defaulting to `("internal_error", 500)` for
  any error that doesn't match. Against the pre-fix sentinel, `errors.Is(err, apiresp.ErrForbidden)`
  would be false, so `WriteError` would have fallen through to the default case and returned `500`
  instead of `403` — confirming this test's `rec.Code == http.StatusForbidden` assertion would have
  failed red against the pre-task-1 code.

## References

- `api/httpapi/entities.go` — `getEntity` handler (`GET /entities/{uuid}`), the entry point this
  test exercises.
- `api/httpapi/router.go` — route wiring (`r.Get("/{uuid}", h.getEntity)`), `Deps`, `NewRouter`.
- `api/httpapi/mock_test.go` — existing `fake*Service` pattern, `buildTestDeps`, `noopLogger`,
  `withActor`, `actorCtx` — reuse `noopLogger`/`withActor`; do not reuse the fakes (that's exactly
  what this task avoids).
- `api/httpapi/handlers_test.go` — `TestServiceErr_MapsCorrectly` for the response-envelope decode
  pattern (`{"error":{"code":...}}`), and the existing `TestArchiveEntity_403_NonAdmin` /
  `TestGetNaturalPerson_403_WrongOwner` fake-backed 403 tests this task's real-service test
  complements (not replaces).
- `api/entity/resolver_test.go` — `resolverStubQuerier`, a closely analogous 18-method
  `coredb.Querier` stub in a different package; mirror its shape for this task's `stubQuerier`
  rather than inventing a different pattern.
- `api/service/service.go` — `New(...)` constructor signature and what each parameter is used
  for; confirms which can safely be `nil` for this test.
- `api/authz/authz.go` — `Authorizer` interface (single `Authorize(ctx, operation string, target
  *int64) error` method) to stub.
- `model/db/querier.go` — the full `coredb.Querier` interface (18 methods) to implement in the
  stub.
- `plan/followups.yaml` item `mJ5k` — the source finding, specifically its call for
  "httpapi-layer or integration-style test" coverage.
