# Masked-Lookup 403 Fix

## Purpose and scope

Address followup `mJ5k` (tag `apiresp-error-widgets`, title "Masked-miss may 500 not 403
(untraced)") recorded in `plan/followups.yaml`. The followup's author suspected that
`entity.Resolver.Resolve` (`api/entity/resolver.go`) returns package-local
`entity.ErrForbidden`/`entity.ErrNotFound` — distinct Go error values from
`apiresp.ErrForbidden`/`apiresp.ErrNotFound` with no `Unwrap()` relating them — and that no
translation step exists before those errors reach `apiresp.WriteError`'s `classify()`
(`api/apiresp/writer.go`), so a masked entity-lookup miss falls through `classify()`'s default
case and returns `500 internal_error` instead of the documented `403 forbidden`.

**Trace confirms the gap is real.** All five call sites of `entityResolver.Resolve(...)` in
`api/service/*.go` —

- `EntityService.GetByUUID` (`api/service/entity.go:52`)
- `EntityService.ResolveProfile` (`api/service/entity.go:107`)
- `CorporationService.GetByEntityUUID` (`api/service/corporation.go:167`)
- `NaturalPersonService.GetByEntityUUID` (`api/service/natural_person.go:223`)
- `ServiceAccountService.GetByEntityUUID` (`api/service/service_account.go:136`)

— return the resolver's error verbatim (`if err != nil { return ..., err }`), and every
`api/httpapi/*.go` handler calls `apiresp.WriteError(w, r, err)` with no intermediate
translation. `apiresp.WriteError`'s `classify()` only recognizes `errors.Is` matches against
apiresp's own five sentinels, so `entity.ErrForbidden`/`entity.ErrNotFound` fall through to the
default `internal_error`/500 case. Confirmed via `GET /entities/{uuid}` → `getEntity` handler →
`EntityService.ResolveProfile` → `entity.Resolver.Resolve` → unmapped `entity.ErrForbidden`.

**Root cause and prescribed fix, per the design doc itself.**mod-core's own
`docs/mf-standards/architecture/api-response-design.md` ("Go-layer ownership" section, "How this
subsumes the per-module shims") states the target design explicitly:

> `normalizeEntityResolveErr` disappears. The `core-api/entity` resolver returns the canonical
> `apiresp.ErrForbidden` / `apiresp.ErrNotFound` (per the masking policy) directly, and
> `WriteError` recognises them via `errors.Is`. There is nothing left to normalise.

`core-api/entity` is this repo's `api/entity` package. The doc-endorsed fix is therefore **not** a
translation shim scattered across the five `api/service/*.go` call sites (which the followup's
author considered), but a two-variable alias fix at the source: `api/entity/resolver.go`'s
`ErrForbidden`/`ErrNotFound` should be assigned `apiresp.ErrForbidden`/`apiresp.ErrNotFound`
directly, exactly as `api/service/errors.go` already does (`var ErrForbidden =
apiresp.ErrForbidden`). Once aliased, every one of the five call sites above starts mapping
correctly with zero changes to `api/service/*.go` or `api/httpapi/*.go`, because `errors.Is`
compares by identity and the "entity" sentinels literally become the apiresp sentinels. No import
cycle results: `api/apiresp` imports only `api/opctx`, not `api/entity`.

**Only test files reference `entity.ErrForbidden`/`entity.ErrNotFound` directly** (in
`api/entity/resolver_test.go` and `api/service/entity_test.go`), all via `errors.Is` — safe under
aliasing since identity-based checks still hold. No non-test code depends on the two sentinel sets
being distinct values.

**Test-coverage gap, also confirmed real.** Existing coverage of the five call sites'
missing-UUID path is either absent (`ServiceAccountService.GetByEntityUUID`,
`EntityService.ResolveProfile` have no not-found test at all) or asserts only `err != nil`
(`CorporationService.GetByEntityUUID`, `NaturalPersonService.GetByEntityUUID` — see
`corporation_test.go:90-98`, `natural_person_test.go:216-224`), which would not have caught this
regression. No `api/httpapi/*_test.go` test constructs a real `service.EntityService` — every
existing httpapi test injects a `fake*Service` that returns `service.ErrForbidden` directly,
bypassing the real resolver entirely, so the httpapi layer has zero coverage of the actual
resolver→service→`WriteError` chain that was broken.

**Out of scope:** `model/` (separate Go module), anything under `docs/mf-standards/` (submodule,
separate repo — the design doc there is authoritative reference only, not an edit target), and the
`gui/` package (unaffected — this is a server-side response-code fix).

## Current status

No prior work on this plan. Single phase, two sequential tasks: the fix (with tightened/added
unit-test assertions at the point of the bug) lands first, then a genuine httpapi-layer
integration test that exercises the real (non-fake) service chain proves the fix end-to-end and
guards against regression at the layer the original gap slipped through.

## Overview

### Phase 1 — Masked Lookup 403 Fix (`phase-01-masked-lookup-fix`)

1. **`001-alias-entity-resolver-sentinels.md`** — Alias `entity.ErrForbidden`/`entity.ErrNotFound`
   to `apiresp.ErrForbidden`/`apiresp.ErrNotFound` in `api/entity/resolver.go`, per the design
   doc's prescribed "Go-layer ownership" shape. Tighten the existing weak/missing not-found
   assertions in `api/service/entity_test.go` (`GetByUUID`, and add a new `ResolveProfile`
   not-found case), `api/service/corporation_test.go`, `api/service/natural_person_test.go`, and
   `api/service/extra_test.go` (service account) so each of the five `entityResolver.Resolve`
   call sites has a test asserting `errors.Is(err, ErrForbidden)` (the service package's own
   apiresp-aliased sentinel) rather than a bare `err != nil` check. No changes to
   `api/service/*.go` non-test files or `api/httpapi/*.go` are needed — the alias fix at the
   source is sufficient.
2. **`002-add-masked-miss-httpapi-test.md`** — Depends on task 1 (assertions must pass against
   the fixed code). Adds a new httpapi-layer test that wires a **real** `service.EntityService`
   (via `service.New`, with a real `entity.NewResolver()` and a minimal stub `coredb.Querier` +
   `authz.Authorizer` — no fakes standing in for the service layer) into `NewRouter`, sends a real
   `GET /entities/{uuid}` HTTP request for an unresolvable UUID, and asserts the response is
   `403` with top-level `error.code == "forbidden"` — closing the coverage gap the followup
   identified (service-layer `errors.Is` assertions alone let this regression through
   undetected).

No parallel-eligible group: task 2's test only passes once task 1's fix lands, so the two tasks
are strictly sequential.

No `doc-updates` phase: this change restores already-documented behavior (the masking policy in
`docs/mf-standards/architecture/api-response-design.md`, out of scope to edit) rather than
introducing a new subsystem, cross-cutting layer, public API/component-boundary change, or
tracked-state change. `mod-core` has no local `docs/architecture.md` to update.
