# Plan Summary: require-authenticated

## What was planned and why

Add two small, purely-additive exported primitives to `mod-core/api` so that the
"is there an effective actor on this context?" question has exactly one
canonical implementation that every ModuleForge module can share:

1. `opctx.EffectiveActorEntityID(ctx context.Context) (int64, bool)` — the
   sudo-first-then-actor policy, promoted from an unexported duplicate that
   currently lives only inside mod-users.
2. `authz.RequireAuthenticated(ctx context.Context) error` — a free function
   that returns the existing `apiresp.ErrUnauthenticated` sentinel when no
   effective actor is present, and `nil` otherwise.

This is **mod-core's slice of a federated, two-project plan** (mod-core,
mod-users). mod-core hosts the new primitives; mod-users refactors its existing
internal 401-detection logic to delegate to them, in a sibling phase living in
mod-users' own plan worktree and dispatched separately. mod-notifications
originally requested the capability but is not part of this plan, and its own
consumption of the new primitives is out of scope.

### What must change

- `api/opctx/opctx.go` — one new exported accessor plus a package-doc touch.
- `api/opctx/opctx_test.go` — unit coverage for the new accessor.
- `api/authz/authz.go` — one new exported free function plus a package-doc touch.
- `api/authz/authz_test.go` — unit coverage for the new free function.
- `AGENTS.md` — the `opctx/` and `authz/` rows of the "Key types and packages"
  table, which become factually stale once the symbols land.

### What must not change

- The `Authorizer` interface and its `Authorize` method signature
  (`api/authz/authz.go:43-45`). `RequireAuthenticated` is a **free function**,
  deliberately not an interface method, so that the roughly eight modules and
  test stubs implementing `Authorizer` across the workspace (mod-users,
  mod-repos, app-mfmanager, and others) keep compiling untouched.
- The `OpResolver` interface.
- The existing `opctx` accessors `ActorEntityID`, `SudoActorEntityID`,
  `RequestID`, and the three `With*` setters.
- The `apiresp` sentinel set. `ErrUnauthenticated` (`api/apiresp/errors.go:21`)
  is reused as-is; no new sentinel is introduced.
- Any file outside mod-core. No changes to mod-users, mod-repos,
  app-mfmanager, or mod-notifications.

### Success criteria

- `cd api && go build ./...` succeeds — in particular, the new `authz` →
  `apiresp` and `authz` → `opctx` import edges introduce no import cycle.
- `cd api && make test` passes, including new unit tests covering, for each new
  function, all three actor states: sudo-actor set, real actor only, and neither
  set.
- `RequireAuthenticated`'s failure test asserts the **exact sentinel** via
  `errors.Is(err, apiresp.ErrUnauthenticated)`, not merely a non-nil error.
- `cd api && make lint` (`go vet ./...` plus a `gofmt` check) is clean.
- Every existing `Authorizer` implementation in the workspace still satisfies
  the interface without modification — established by the interface source
  being byte-identical apart from surrounding additions.

### Hard constraints

- Purely additive. No existing exported symbol changes name, signature, or
  behavior.
- The canonical Authorizer-contract conventions in
  `docs/mf-standards/architecture/authorization-design.md` govern placement,
  naming, and doc-comment style. That document is a **git submodule** and is not
  checked out in this worktree; everything the implementing tasks need from it
  has been extracted into
  [naming and placement research](./notes/naming-and-placement.md), and the
  submodule mechanics are recorded in the
  [submodule doc constraint note](./notes/submodule-doc-constraint.md).
- This work must be functionally complete and mergeable **before** mod-users'
  delegating task in the same federated plan. That ordering is already recorded
  as a cross-project dependency in mod-users' `plan/manifest.yaml`.

Two phases. Phase 01 is the entire code change; Phase 02 is the standard
architecture-doc reconciliation triggered by the public-API addition.

### Phase 01 — Authenticated Check Primitives

The complete code change, in the single coherent area of `mod-core/api`'s
cross-cutting auth contract packages. Two tasks, run **in sequence** — task 002
calls the function task 001 creates, so they are not parallel-eligible.

- **001 — Add Effective Actor Accessor.** Add
  `opctx.EffectiveActorEntityID(ctx) (int64, bool)` to `api/opctx/opctx.go`,
  returning `SudoActorEntityID(ctx)` when set and otherwise falling through to
  `ActorEntityID(ctx)`. Extend the package doc comment to describe the derived
  accessor without implying a fourth context value exists. Add table-free
  subtests covering sudo-set, real-actor-only, and neither-set. Correct the
  `opctx/` row of the AGENTS.md package table.

- **002 — Add Require Authenticated.** Add
  `authz.RequireAuthenticated(ctx) error` to `api/authz/authz.go`, delegating to
  the accessor from task 001 and returning `apiresp.ErrUnauthenticated` when no
  effective actor is present. Leave the `Authorizer` and `OpResolver` interfaces
  untouched. Extend the package doc comment so its "defines only the contract"
  framing stays accurate alongside a concrete helper. Add unit tests for the
  same three actor states, asserting the exact sentinel on the failure case.
  Correct the `authz/` row of the AGENTS.md package table.

### Phase 02 — Documentation Updates

Reconciles the canonical architecture documentation with the public-API
addition Phase 01 makes. One task, dependent on Phase 01 having landed.

- **001 — Update Architecture Docs.** Review
  `docs/mf-standards/architecture/authorization-design.md` — the canonical
  Authorizer-contract document — against the two new primitives, and verify the
  AGENTS.md package-table corrections from Phase 01 are accurate and complete.
  The architecture doc lives in a git submodule backed by a separate repository,
  so any edit to it is a cross-repo change; the task reports that as a flagged
  item for manager direction rather than performing an unsanctioned submodule
  commit and pointer bump.

## What shipped

### Phase 01 — Authenticated Check Primitives

1. **Add Effective Actor Accessor** (`001-add-effective-actor-accessor.md`, tier `sonnet-med`) — Added exported EffectiveActorEntityID(ctx) (int64, bool) to api/opctx/opctx.go implementing sudo-first-then-actor policy. Extended package doc. Added TestEffectiveActorEntityID with four subtests. Updated AGENTS.md opctx/ row. No pre-existing signatures changed. All validation checks pass including module-wide make test.
   Commit `5cbe3d8`, merged at `681554fbba66d8c10d5b0fb2f6576c154311beab`.

2. **Add Require Authenticated** (`002-add-require-authenticated.md`, tier `sonnet-med`) — Added the free function authz.RequireAuthenticated(ctx) error to api/authz/authz.go, delegating to opctx.EffectiveActorEntityID and returning the bare apiresp.ErrUnauthenticated sentinel when no effective actor is present, nil otherwise. Deliberately a free function, not an interface method, so no Authorizer implementer needs to change. Package doc extended; AGENTS.md authz/ row updated. Added TestRequireAuthenticated with three subtests asserting the exact sentinel. All validation checks pass including module-wide make test and make lint.
   Commit `d373479`, merged at `c35e0865ba5cb56de948ba237654c15d71246b25`.

### Phase 02 — Documentation Updates

1. **Update Architecture Docs** (`001-update-architecture-docs.md`, tier `sonnet-high`) — Reviewed mod-core's docs against Phase 01's two public-API additions. AGENTS.md and README.md already accurate, no edit needed. docs/architecture.md and docs/*-spec.md confirmed absent, neither created. Drafted two concrete amendments to the canonical docs-mf-standards submodule doc (Authorizer interface section, Operation context section) but did NOT commit into that submodule or bump mod-core's pointer, per hard constraint — drafted text reported for manager routing. No api/ source file touched; Authorizer interface confirmed byte-identical.
   Commit `24e556f`, merged at `13e98107cb67dba442d6278233333ea9cad91ce4`.

## Key decisions

_No `## Why this shape` section is recorded in `plan/overview.md`, so this plan's cross-task rationale was never written down. Per-task outcomes are under "What shipped" above._

## Follow-up items

- **`qxLX`** — **Cross-repo doc amendment awaiting explicit ma** — Cross-repo doc amendment awaiting explicit manager/user authorization before being applied to the docs-mf-standards repository (architecture/authorization-design.md) — NOT committed anywhere by this task. Drafted amendment: (a) new bullet in "The Authorizer interface" section documenting RequireAuthenticated as a free function distinct from Authorize; (b) new paragraph in "Operation context (opctx)" section documenting opctx.EffectiveActorEntityID as the canonical sudo-first-then-actor accessor. Full verbatim text is in this task's structured report.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — Authenticated Check Primitives

- [x] [001-add-effective-actor-accessor.md](./phase-01-authenticated-check-primitives/001-add-effective-actor-accessor.md) — tier `sonnet-med` · branch `plan/require-authenticated-01-001` · commit `5cbe3d8` · merge `681554fbba66d8c10d5b0fb2f6576c154311beab`
- [x] [002-add-require-authenticated.md](./phase-01-authenticated-check-primitives/002-add-require-authenticated.md) — tier `sonnet-med` · branch `plan/require-authenticated-01-002` · commit `d373479` · merge `c35e0865ba5cb56de948ba237654c15d71246b25`

### Phase 02 — Documentation Updates

- [x] [001-update-architecture-docs.md](./phase-02-doc-updates/001-update-architecture-docs.md) — tier `sonnet-high` · branch `plan/require-authenticated-02-001` · commit `24e556f` · merge `13e98107cb67dba442d6278233333ea9cad91ce4`
