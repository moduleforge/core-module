# Plan Summary: masked-lookup-403

## What was planned and why

This plan addressed followup `mJ5k` (tag `apiresp-error-widgets`, title "Masked-miss may 500
not 403 (untraced)"), found while spot-checking `docs-mf-standards/building-common.md`'s
error-handling section against the merged `apiresp-error-widgets` implementation. The followup's
author suspected that `entity.Resolver.Resolve` (`api/entity/resolver.go`) returns package-local
`entity.ErrForbidden`/`entity.ErrNotFound` — distinct Go error values from
`apiresp.ErrForbidden`/`apiresp.ErrNotFound` with no `Unwrap()` relating them — and that no
translation step exists before those errors reach `apiresp.WriteError`'s `classify()`, so a
masked entity-lookup miss would fall through to the default case and return `500 internal_error`
instead of the documented `403 forbidden`.

The plan's own trace confirmed the gap was real: all five call sites of
`entityResolver.Resolve(...)` in `api/service/*.go` returned the resolver's error verbatim, and
no `api/httpapi/*.go` handler performed any intermediate translation before calling
`apiresp.WriteError`. The prescribed fix, per `docs/mf-standards/architecture/api-response-design.md`
("Go-layer ownership" section), was a two-variable alias fix at the source — `api/entity/resolver.go`'s
sentinels should be assigned the `apiresp` sentinels directly, exactly as `api/service/errors.go`
already does — rather than a translation shim scattered across the five call sites. The plan also
confirmed a real test-coverage gap: no `api/httpapi/*_test.go` test constructed a real
`service.EntityService`, so the actual resolver -> service -> `WriteError` chain that was broken
had zero httpapi-layer coverage.

Scope was a single phase with two sequential tasks: land the alias fix (with tightened/added
service-layer unit-test assertions) first, then add a genuine httpapi-layer integration test
proving the fix end-to-end and guarding against regression at the layer the original gap slipped
through. `model/` (separate Go module), `docs/mf-standards/` (submodule, reference-only), and
`gui/` (unaffected server-side fix) were explicitly out of scope.

## What shipped

### Phase 1 — Masked Lookup 403 Fix (`phase-01-masked-lookup-fix`)

**Task 1 — Alias Entity Resolver Errors To Apiresp Sentinels**
(`phase-01-masked-lookup-fix/001-alias-entity-resolver-sentinels.md`, merge `3fc99efc8025f6668b8b22d68e386d61b736c5c9`)

Aliased `api/entity/resolver.go`'s `ErrForbidden`/`ErrNotFound` sentinels directly onto
`apiresp.ErrForbidden`/`apiresp.ErrNotFound` (matching `api/service/errors.go`'s precedent and
the design doc's prescribed shape), so `apiresp.WriteError`'s `errors.Is`-based `classify()` now
correctly maps a masked entity-lookup miss to 403/404 instead of falling through to 500 at all
five `entityResolver.Resolve(...)` call sites. Doc comments were updated, no import cycle was
introduced, and `api/service/*.go`/`api/httpapi/*.go` non-test files were left untouched (the
alias fix at the source was sufficient — no downstream translation code was needed). Test
coverage was strengthened: alias assertions in `resolver_test.go`, a second `ErrForbidden`
assertion in `GetByUUID_NotFound`, a new `TestEntityService_ResolveProfile_NotFound`, and
strengthened bare `err == nil` checks in the corporation/natural_person/extra tests to
`errors.Is(err, ErrForbidden)`. `make lint`, `go build`, and `make test` all passed.

**Task 2 — Add Masked-Miss Httpapi Integration Test**
(`phase-01-masked-lookup-fix/002-add-masked-miss-httpapi-test.md`, merge `20dc33d01db5001b7412f052017089e7288a1825`)

Added `api/httpapi/masked_lookup_test.go` with `TestGetEntity_MaskedMiss_Returns403Forbidden`, an
httpapi-layer test that wires the real `service.EntityService` (via `service.New` with
`entity.NewResolver()`) into `NewRouter` instead of the package's usual `fake*Service` mocks, so a
`GET /entities/{random-uuid}` request against a stub `Querier` returning `pgx.ErrNoRows` exercises
the actual resolver -> service -> `apiresp.WriteError` chain and asserts the response is
403/forbidden rather than the pre-fix 500. This required a package-local 18-method `stubQuerier`
(mirroring `api/entity/resolver_test.go`'s `resolverStubQuerier`) and a trivial `stubAuthorizer`
that always allows. All four validation checks passed; the pre-fix regression check was done by
reading the task-1 commit diff and `apiresp`'s `classify()` logic rather than actually reverting,
per the task doc's stated alternative. No scope expansion was needed.

## Key decisions

- **Alias fix at the source, not a translation shim.** Per the design doc's explicit prescription
  ("Go-layer ownership" section — `normalizeEntityResolveErr` disappears, the resolver returns the
  canonical `apiresp` sentinels directly), the fix was implemented as a two-variable alias in
  `api/entity/resolver.go` rather than adding translation logic at each of the five call sites in
  `api/service/*.go`. This meant zero changes to `api/service/*.go` non-test files or
  `api/httpapi/*.go` were required — all five call sites started mapping correctly once aliased,
  because `errors.Is` compares by identity.
- **Test-only files were the only non-test consumers of the old distinct sentinel identity**
  (`api/entity/resolver_test.go`, `api/service/entity_test.go`), both via `errors.Is`, which
  remained safe under aliasing — this de-risked the change and confirmed no production code
  depended on the two sentinel sets being distinct values.
- **Pre-fix regression check done by code-reading, not by reverting.** Task 2 verified the new
  httpapi test would have failed pre-fix by reading the task-1 commit diff and `apiresp`'s
  `classify()` logic directly, rather than temporarily reverting the alias and re-running — an
  explicitly sanctioned alternative in the task doc.
- **Followup `mJ5k` is now resolved by this plan, pending removal by the manager.** The followup
  that motivated this plan (tag `apiresp-error-widgets`, "Masked-miss may 500 not 403 (untraced)")
  still exists in `plan/followups.yaml` as of this writing. Its concern — a masked entity-lookup
  miss returning 500 instead of the documented 403 — has been fixed and is now covered by both a
  service-layer unit test and a genuine httpapi-layer integration test. Removing the followup
  entry from `followups.yaml` is a manager-level action outside this plan's/this task's scope, and
  was intentionally not performed here.

## Follow-up items

No new followups were generated by this plan's tasks. The following pre-existing followups in
`plan/followups.yaml` remain outstanding and are unrelated to this plan's scope (all tagged to
earlier phases/plans):

- `womX` — Service validation-reason text lost on 400s (tag `phase-01-apiresp-go`): adopting
  `apiresp.WriteError` drops specific validation-reason text on some 400 responses; needs a
  decision on whether to route validation errors through `apiresp.InvalidInput(FieldError{...})`.
- `vr20` — `gui/` package has no test runner configured (tag `phase-02-gui-error-widgets`):
  pre-existing gap, decide whether/when to add a test runner to `gui/`.
- `XKQ2` — Banner drops detail text when all-unbound (tag `phase-02-gui-error-widgets`): possible
  product-surprise asymmetry in `useApiError`'s `classifyInline` fallback behavior.
- `7VD3` — `request()` has no origin check for token (tag `phase-02-gui-error-widgets`): security
  suggestion, not currently exploitable (no consumer wired yet).
- `pBoG` — Default token storage uses `localStorage` (tag `phase-02-gui-error-widgets`): security
  suggestion, mirrors existing `mod-users` behavior, no action needed now.

As noted above, followup `mJ5k` (the source finding for this plan) should be considered resolved
and is flagged for manager removal from `plan/followups.yaml`.
