# API Response Action-Required Writer And Conflict Constructor

## Purpose and scope

Extend `mod-core`'s shared `api/apiresp` package (`github.com/moduleforge/core-api`) to
implement the **action-required response contract** finalized in
`docs/mf-standards/architecture/api-response-design.md`, and to add the missing public
`Conflict(details ...FieldError) error` constructor. This is Wave 0 of a 3-wave, 3-repo effort
(followup `eiF8`, tag `users-apiresp-migration`): Wave -1 finalized the design doc (already
merged); this plan implements the Go layer; a not-yet-authored mod-users Wave-1 plan is the
downstream consumer that migrates its five deferred handler sites onto what this plan produces.

Two additive, PR-sized units of work, both confined to the `api/apiresp` package:

1. **Action-required machinery** — the third response kind alongside success (`WriteJSON`) and
   error (`WriteError`): the `action` envelope types, an `ActionCode{Code, Status}` value type
   that makes unregistered codes and off-status writes unrepresentable, and the
   `WriteActionRequired(...)` writer — a peer of `WriteJSON`/`WriteError`, **not** a variant of
   `WriteError` — that emits the code's bound status verbatim (including 503) and never remaps it.
2. **`Conflict()` constructor** — a public detail-carrying constructor mirroring the existing
   `InvalidInput()`, so `WriteError` can fully own conflict-with-details responses and downstream
   consumers collapse hand-rolled 409 envelopes onto `apiresp.WriteError(w, r, apiresp.Conflict(...))`.
   This folds in mod-users followup `ZVum` (tag `go-apiresp-foundation`).

**Out of scope (hard boundary):** no `mod-core/gui` changes (the design doc's GUI-facing
action-required widget is a deferred downstream `mod-core/gui` concern); no other repo; no changes
to the `mf-standards` submodule doc (it is the already-finalized authoritative spec this plan
implements against, and it lives in a separate repo). The downstream mod-users backend/GUI
migration is a separate Wave-1 plan.

The one genuine implementation judgment call — whether the design doc's normative constraints on
`action.path` (application-relative; no scheme, no `//`-authority) and `action.data`
(minimal, non-sensitive) warrant a runtime guard in the writer — is decided and recorded in
[action-required writer decisions](./notes/action-required-writer-decisions.md).

## Current status

Starting state: no active plan artifacts exist for this slug beyond this overview. The
`api/apiresp` package is built and merged on mod-core's main branch with `WriteJSON`,
`WriteError`, `InvalidInput`, the sentinel set, and matching test coverage in
`api/apiresp/apiresp_test.go`. The `mf-standards` submodule is hydrated in this worktree at the
pinned commit carrying the finalized action-required design.

The plan is a single phase (Phase 01) with two tasks that are **parallel-eligible** — they create
disjoint files and can be dispatched concurrently. Phase 01 is ready to begin immediately; both
tasks have all the information their implementing agents need.

## Overview

### Phase 01 — Action-Required Writer And Conflict Constructor

One coherent area (the `api/apiresp` package), two independent tasks:

- **001 — Add Action-Required Writer** *(parallel-eligible with 002)*. Adds `api/apiresp/action.go`
  with the `ActionBody`/`ActionEnvelope` wire types, the `ActionCode` value type, the
  `WriteActionRequired` writer implementing the doc's `## Go-layer ownership` item-4 sketch
  precisely, and the decided `action.path` structural guard. New test coverage in
  `api/apiresp/action_test.go`. Touches no existing package file.

- **002 — Add Conflict Constructor** *(parallel-eligible with 001)*. Adds
  `api/apiresp/conflict.go` (a `conflictError` type + `Conflict(...)` constructor mirroring
  `invalidinput.go` exactly) and extends the package's `fieldErrors` detail-recovery accessor so
  `WriteError` surfaces `Conflict()`'s details. New test coverage in `api/apiresp/conflict_test.go`.
  Touches `api/apiresp/invalidinput.go` only (the `fieldErrors` accessor); does not touch the
  files task 001 creates.

Both tasks validate with `cd api && make test` (and `cd api && make lint`) per `AGENTS.md`.
