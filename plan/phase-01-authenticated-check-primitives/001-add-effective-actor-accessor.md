# Add Effective Actor Accessor

## Purpose and scope

Add one new exported accessor, `EffectiveActorEntityID`, to
`api/opctx/opctx.go`. It implements the sudo-first-then-actor policy: return the
sudo-actor entity ID when one is set on the context, otherwise fall through to
the authenticated actor's entity ID.

This promotes a policy that today exists only as an unexported duplicate inside
mod-users (`effectiveActor`, `mod-users/api/internal/authz/authz.go:188-196`)
into a single canonical, exported location. mod-users' own delegation to it is a
separate phase in a separate project worktree and is **not** part of this task.

No standard skill covers this; it is a small, fully-specified Go addition.
Follow the `## Procedure` below.

## Requirements

1. **Add the accessor** to `api/opctx/opctx.go`, placed immediately after
   `SudoActorEntityID` and before `RequestID` so the three actor-related
   accessors stay together and the file's existing ordering (setters, then
   accessors in `actor` / `sudo` / `request` order) is preserved:

   ```go
   func EffectiveActorEntityID(ctx context.Context) (int64, bool) {
       if id, ok := SudoActorEntityID(ctx); ok {
           return id, true
       }
       return ActorEntityID(ctx)
   }
   ```

   The `(int64, bool)` return shape is required — it mirrors both sibling
   accessors exactly, and mod-users' sibling phase will delegate to it in place
   of a function with that identical shape.

2. **Doc-comment the accessor** in the style of its two siblings: a short
   comment stating what it returns, which value wins, *why* sudo wins, and what
   the absent case yields. It must convey that when an admin has assumed another
   user's identity, the assumed identity is the subject of policy checks. State
   the `0, false` result when neither value is set. Model the wording on the
   existing sibling comments rather than inventing a new house style.

3. **Extend the package doc comment** at the top of `api/opctx/opctx.go`. It
   currently opens with "Three values are defined:" and enumerates
   `ActorEntityID`, `SudoActorEntityID`, and `RequestID`. `EffectiveActorEntityID`
   is **not** a fourth context value — it stores nothing, adds no context key,
   and has no `With*` setter. Add a short paragraph after the three-value list
   describing it as a derived accessor over the first two, so the "three values"
   framing stays literally true. Do not renumber the list to four.

4. **Do not** add a `WithEffectiveActor` setter, a new `contextKey`, or any new
   package-level variable. The accessor is pure derivation over existing keys.

5. **Do not** add any import. The package must continue to import only
   `context`.

6. **Add unit tests** to `api/opctx/opctx_test.go` (package `opctx_test`,
   matching the existing external-test-package convention). Add a
   `TestEffectiveActorEntityID` function using `t.Run` subtests, matching the
   file's existing subtest style. It must cover, at minimum:
   - **Sudo set** — both actor and sudo-actor set to distinct values; the
     sudo-actor's ID is returned with `ok == true`. Use distinct values so a
     wrong-branch implementation cannot pass by coincidence.
   - **Sudo set, no real actor** — only sudo-actor set; its ID is returned with
     `ok == true`. Guards against an implementation that requires an actor first.
   - **Real actor only** — only actor set; the actor's ID is returned with
     `ok == true`.
   - **Neither set** — a bare `context.Background()`; returns `0, false`.

7. **Update `AGENTS.md`.** In the "Key types and packages" table, the `opctx/`
   row currently reads:

   > Typed context accessors for `ActorEntityID`, `SudoActorEntityID`, and
   > `RequestID`. Set by HTTP middleware; consumed by service methods and the
   > `Authorizer`.

   Amend it to also name `EffectiveActorEntityID` and identify it as the derived
   sudo-first-then-actor accessor that is the canonical resolution of "who is the
   effective actor?" for every ModuleForge module. Keep the row a single table
   cell on one line, matching every other row in that table; do not restructure
   the table.

## Validation

- `cd api && gofmt -l opctx` prints nothing.
- `cd api && go vet ./opctx/...` is clean.
- `cd api && go test ./opctx/...` passes, including the new
  `TestEffectiveActorEntityID` and all pre-existing tests in the file.
- `cd api && go build ./...` succeeds — no unintended breakage elsewhere in the
  module.
- `cd api && make test` passes (the module-wide `go test ./...`). If dependency
  download is unavailable, the narrower `go test ./opctx/...` above is the
  minimum bar; report the module-wide result either way rather than silently
  skipping it.
- `git diff --stat` shows exactly three changed files: `api/opctx/opctx.go`,
  `api/opctx/opctx_test.go`, and `AGENTS.md`.
- `grep -n "func ActorEntityID\|func SudoActorEntityID\|func RequestID\|func WithActor\|func WithSudoActor\|func WithRequestID" api/opctx/opctx.go` shows all
  six pre-existing signatures unchanged.
- `grep -rn "EffectiveActorEntityID" api/` shows hits only in `api/opctx/opctx.go`
  and `api/opctx/opctx_test.go` — nothing else in mod-core consumes it yet.

## Metadata

architectural_impact: true

## Assumptions

- Go dependencies are **not** yet installed in this checkout. A first
  `go build` or `go test` may need to download modules. `api/go.mod` carries a
  local `replace` for `github.com/moduleforge/core-model` pointing at `../model`,
  so no `go.work` is required for a plain `cd api && go build ./...`; the sibling
  `model/` directory is present in the worktree.
- `docs/mf-standards/` is an **empty directory** in this worktree — it is an
  uninitialized git submodule pointing at a separate repository. Do not attempt
  to read or edit anything under it; everything needed from the canonical
  authorization design doc is reproduced in the references below.
- All pre-existing tests in `api/opctx/opctx_test.go` pass before this task
  starts.

## References

- `api/opctx/opctx.go` — the file being edited. Package doc lines 1-20; the two
  sibling accessors at lines 54-66.
- `api/opctx/opctx_test.go` — the test file being extended; `TestActorEntityID`
  (lines 11-44) is the closest model for the new test's shape.
- `plan/notes/naming-and-placement.md` — the naming, signature, and placement
  decisions, with the supporting evidence from
  `docs/mf-standards/architecture/authorization-design.md` extracted verbatim.
  **Read this instead of the submodule doc**, which is not checked out here.
- `plan/notes/submodule-doc-constraint.md` — why the canonical architecture doc
  is unreadable from this worktree and must not be edited by this task.
- `AGENTS.md` — "Key types and packages" section, `opctx/` row.

## Procedure

1. Read `api/opctx/opctx.go` in full, then `api/opctx/opctx_test.go`.
2. Add `EffectiveActorEntityID` with its doc comment between `SudoActorEntityID`
   and `RequestID`.
3. Extend the package doc comment with the derived-accessor paragraph.
4. Add `TestEffectiveActorEntityID` with the four subtests.
5. Run `gofmt -l opctx`, `go vet ./opctx/...`, `go test ./opctx/...`, then
   `go build ./...` and `make test` from `api/`.
6. Update the `opctx/` row in `AGENTS.md`.
7. Run the `git diff --stat` and `grep` checks from `## Validation`.

## Status

- **Outcome:** succeeded
- **Date:** 2026-08-15
- **Validation summary:** `gofmt -l opctx` printed nothing; `go vet ./opctx/...`
  clean; `go test ./opctx/...` passed (all pre-existing tests plus the new
  `TestEffectiveActorEntityID` with its four subtests); `go build ./...`
  succeeded; `make test` (module-wide `go test ./...`) passed across all 13
  packages. `git diff --stat` showed exactly the three expected files
  (`AGENTS.md`, `api/opctx/opctx.go`, `api/opctx/opctx_test.go`). The six
  pre-existing accessor/setter signatures in `api/opctx/opctx.go` are
  unchanged. `EffectiveActorEntityID` appears only in
  `api/opctx/opctx.go` and `api/opctx/opctx_test.go` under `api/`.
- **Affected source files:**
  - `api/opctx/opctx.go`
  - `api/opctx/opctx_test.go`
  - `AGENTS.md`
- **Assumptions applied:** Go dependencies were already resolvable in this
  checkout (no network download was needed); `docs/mf-standards/` was not
  read, consistent with the task's assumption that it is an uninitialized
  submodule.
