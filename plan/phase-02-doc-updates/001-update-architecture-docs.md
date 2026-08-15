# Update Architecture Docs

## Purpose and scope

Reconcile mod-core's architecture and reference documentation with the
public-API addition made in Phase 01: the two new exported primitives
`opctx.EffectiveActorEntityID` and `authz.RequireAuthenticated`.

Both additions land in the cross-cutting authentication/authorization layer and
extend a public API boundary that peer modules consume, which is what triggered
this phase. Neither adds a subsystem, changes spec-defined behavior, or adds
tracked state, so the review is expected to be narrow.

Follow the `update-architecture-docs` task procedure at
`plugins/flow/task-procedures/update-architecture-docs/SKILL.md`.

## Requirements

`role_doc: plugins/flow/roles/architect-backend.md`

The implications are backend API/component-contract changes — new exported
functions in the Go authorization contract packages — so the backend architect
role applies rather than the data, cloud, or frontend variants.

### Implementation task documents that surfaced the implications

Both are Phase 01 tasks that will have completed by the time this phase runs:

- `plan/phase-01-authenticated-check-primitives/001-add-effective-actor-accessor.md`
- `plan/phase-01-authenticated-check-primitives/002-add-require-authenticated.md`

Read both, and read the code they produced (`api/opctx/opctx.go`,
`api/authz/authz.go`) rather than relying on the task docs alone.

### Files to review

1. **`docs/mf-standards/architecture/authorization-design.md`** — the canonical
   Authorizer-contract document for this interface family, and the primary
   subject of this task. Assess it against the Phase 01 changes. The two
   sections most likely to need amendment:
   - *The `Authorizer` interface* — should record that
     `authz.RequireAuthenticated(ctx) error` now exists as a free function
     alongside the interface, that it is an authentication-only check distinct
     from the `Authorize` gate, and that it returns the already-documented
     `ErrUnauthenticated` sentinel the doc's 401 mapping rule covers. The
     interface block itself is unchanged and must be shown as unchanged.
   - *Operation context (`opctx`)* — the doc already states the sudo-first rule
     in prose ("the authorizer reads the **assumed** actor for policy
     purposes"). It should record that `opctx.EffectiveActorEntityID` is now the
     canonical executable form of that rule, and that modules should call it
     rather than re-deriving the sudo-then-actor fallback locally. Note that it
     adds no new context key and no new `With*` setter, so the doc's three-row
     context-value table stays correct as-is.

   **This file is a git submodule and is a cross-repository change.** See
   [Assumptions](#assumptions) and [Procedure](#procedure) below — do not perform
   an unsanctioned submodule commit and pointer bump.

2. **`AGENTS.md`** — the "Key types and packages" table. Phase 01 tasks 001 and
   002 each amended one row (`opctx/` and `authz/` respectively). Verify both
   landed, are accurate against the shipped code, and that the `authz/` row no
   longer claims the package "defines only the contracts". Correct anything
   stale or inconsistent between the two rows. Also check the `apiresp/` row —
   it enumerates the sentinel set and should still be accurate, since no
   sentinel was added.

3. **`README.md`** — its "Go API library" bullet summarizes what `core-api`
   provides, including "authorization and operation-context contracts". Confirm
   that phrasing still covers the package accurately; amend only if it has
   become misleading. A no-change outcome here is an acceptable and likely
   result — report it explicitly rather than editing for its own sake.

### Files confirmed absent — do not go looking for them

mod-core has **no** `docs/architecture.md` and **no** `docs/*-spec.md` at its
project root. `docs/` contains exactly one entry, the `mf-standards` submodule.
Record this in the report; do not create either file as part of this task.

## Validation

- Each of the three named files above has been read and explicitly assessed, and
  the report states, per file, one of: updated (with a summary of the edit),
  reviewed and no change needed (with the reason), or blocked (with the
  blocker).
- The `docs/architecture.md` / `docs/*-spec.md` absence is confirmed by
  `ls docs/` and `ls docs/*-spec.md 2>&1` and reported, with neither file
  created.
- `AGENTS.md`'s `opctx/` row names `EffectiveActorEntityID` and its `authz/` row
  names `RequireAuthenticated`, each described accurately against the shipped
  code. Verify with
  `grep -n "EffectiveActorEntityID\|RequireAuthenticated" AGENTS.md`.
- No source file under `api/` is modified by this task. Verify with
  `git diff --stat -- api/`, which must be empty.
- The `Authorizer` interface is confirmed unchanged from its pre-plan form —
  `grep -n "Authorize(ctx context.Context, operation string, target \*int64) error" api/authz/authz.go`
  matches, and any documentation asserting the interface changed is corrected.
- If the `authorization-design.md` update was not performed, the report includes
  the exact proposed edit (section, and the text to insert) so the manager can
  route it to the `docs-mf-standards` repository without re-deriving it.

## Assumptions

- Phase 01 has fully landed and merged; both new symbols exist in the working
  tree being reviewed.
- `docs/mf-standards/` is a **git submodule** (`.gitmodules` declares it,
  pointing at `git@github.com:moduleforge/docs-mf-standards.git`) and is an
  **empty directory** in a fresh worktree. Reading it requires
  `git submodule update --init docs/mf-standards`; the content is populated in
  the main checkout at
  `/Users/zane/playground/moduleforge/mod-core/docs/mf-standards/`.
- Editing that file means committing in a **different repository** and then
  advancing mod-core's submodule pointer. That is outside the ordinary in-repo
  task-agent change model. Treat it as a decision for the manager: prepare the
  exact proposed edit and report it as a flagged item, and perform the
  cross-repo commit only under explicit manager direction.

## References

- `plan/notes/submodule-doc-constraint.md` — the full record of the submodule
  constraint, why it changes how this phase must operate, and why the AGENTS.md
  row updates were folded into Phase 01 instead of deferred here.
- `plan/notes/naming-and-placement.md` — the naming/placement decisions and the
  statements from `authorization-design.md` that governed them; useful as the
  starting point for drafting the doc amendment.
- `plan/overview.md` — the plan's scope, hard constraints, and success criteria.
- `plugins/flow/task-procedures/update-architecture-docs/SKILL.md` — the task
  procedure to follow.

## Procedure

1. Read both Phase 01 task documents and the code they produced.
2. Verify and, if needed, correct the two `AGENTS.md` package-table rows.
3. Review `README.md`'s "Go API library" bullet; amend only if misleading.
4. Confirm and report the absence of `docs/architecture.md` and
   `docs/*-spec.md`.
5. Read `docs/mf-standards/architecture/authorization-design.md` from the main
   checkout (or after `git submodule update --init docs/mf-standards`), and
   draft the concrete amendments to its `Authorizer` interface and Operation
   context sections.
6. Do **not** commit into the submodule or bump the submodule pointer without
   explicit manager direction. Report the drafted amendment verbatim as a
   flagged item instead, naming the target repository and file.

## Status

- **Outcome:** succeeded
- **Date:** 2026-08-15
- **Validation summary:** All `## Validation` checks passed.
  - `docs/architecture.md` and `docs/*-spec.md` confirmed absent
    (`ls docs/architecture.md` → No such file or directory; `ls docs/*-spec.md`
    → No such file or directory; `ls docs/` → `mf-standards` only). Neither
    file was created.
  - `grep -n "EffectiveActorEntityID\|RequireAuthenticated" AGENTS.md` shows
    both rows present and accurate against the shipped code (`opctx/` row
    names `EffectiveActorEntityID`; `authz/` row names `RequireAuthenticated`
    and no longer claims the package "defines only the contracts"). No edit
    needed — Phase 01 tasks 001/002 already landed both corrections
    accurately.
  - `git diff --stat -- api/` is empty — no source file under `api/` was
    modified by this task.
  - `grep -n "Authorize(ctx context.Context, operation string, target \*int64) error" api/authz/authz.go`
    matches at line 50 — the `Authorizer` interface is confirmed unchanged
    from its pre-plan form.
  - `README.md`'s "Go API library" bullet ("authorization and
    operation-context contracts") reviewed and found still accurate — not
    misleading against the two new primitives. No edit made, per the task
    doc's own anticipated no-change outcome.
  - `docs/mf-standards/architecture/authorization-design.md` read from the
    main checkout
    (`/Users/zane/playground/moduleforge/mod-core/docs/mf-standards/architecture/authorization-design.md`,
    read-only). Amendments drafted for the `Authorizer` interface and
    Operation context (`opctx`) sections but **not applied** — that file is a
    git submodule (`docs-mf-standards`) backed by a separate repository; per
    `## Assumptions`/`## Procedure`, no cross-repo commit or submodule
    pointer bump was performed without explicit manager direction. The
    verbatim drafted amendment is reported in the structured report's
    `flagged_for_manager` field for the manager to route.
- **Files reviewed (per-file disposition):**
  - `AGENTS.md` — reviewed, no change needed (both rows already accurate).
  - `README.md` — reviewed, no change needed (bullet still accurate).
  - `docs/mf-standards/architecture/authorization-design.md` — blocked on
    manager direction; amendment drafted and reported, not applied (submodule
    boundary).
- **Affected source files:** none in `mod-core` proper. Only this task
  document was updated.
- **Assumptions applied:** Phase 01 has fully landed and merged into this
  worktree's base; both new symbols (`opctx.EffectiveActorEntityID`,
  `authz.RequireAuthenticated`) exist and are exercised by the review above.
  `docs/mf-standards/` was confirmed empty in this task worktree; the
  canonical doc was read instead from the main checkout as instructed, and
  not modified.
