# Add Require Authenticated

## Purpose and scope

Add one new exported **free function**, `RequireAuthenticated`, to
`api/authz/authz.go`. It answers the authentication-only question — "is there an
effective actor on this context at all?" — by delegating to
`opctx.EffectiveActorEntityID` (added by task 001) and returning the existing
`apiresp.ErrUnauthenticated` sentinel when there is none, `nil` otherwise.

This is deliberately **not** a new method on the `Authorizer` interface. Adding
one would force a change in every `Authorizer` implementation across the
workspace — mod-users, mod-repos, app-mfmanager, and test stubs spanning roughly
eight modules. A free function requires no implementer to change.

No standard skill covers this; it is a small, fully-specified Go addition.
Follow the `## Procedure` below.

## Requirements

1. **Depends on task 001.** `opctx.EffectiveActorEntityID` must already exist.
   Do not re-derive the sudo-first-then-actor policy inline here — the entire
   point of this plan is that the policy lives in exactly one place. If the
   accessor is absent, halt and report rather than reimplementing it.

2. **Add the free function** to `api/authz/authz.go`, placed **after** both the
   `Authorizer` and `OpResolver` interface declarations, so the file continues
   to read as "contracts first":

   ```go
   func RequireAuthenticated(ctx context.Context) error {
       if _, ok := opctx.EffectiveActorEntityID(ctx); !ok {
           return apiresp.ErrUnauthenticated
       }
       return nil
   }
   ```

3. **Return the existing sentinel bare** — `apiresp.ErrUnauthenticated`
   (`api/apiresp/errors.go:21`). Do **not** introduce a new sentinel, and do
   **not** wrap it with `fmt.Errorf`. `apiresp.WriteError` classifies via
   `errors.Is` so wrapping would still map correctly, but an unwrapped return
   keeps the value identity exact and matches how the existing sentinels are
   returned elsewhere.

4. **The name is fixed as `RequireAuthenticated`.** The original requester
   suggested `RequireAuthorization`; that name is rejected because it names the
   wrong security question — this checks *authentication* (is anyone there?),
   not *authorization* (may they do this?). Do not rename it.

5. **Do not modify the `Authorizer` interface** (`api/authz/authz.go:43-45`) or
   the `OpResolver` interface. Their declarations, method sets, and doc comments
   must survive this task byte-identical apart from any text explicitly required
   below.

6. **Add the two imports** `github.com/moduleforge/core-api/opctx` and
   `github.com/moduleforge/core-api/apiresp` alongside the existing `context`
   import. These edges introduce **no import cycle**: `apiresp` already imports
   `opctx` (in `apiresp/writer.go`) and neither `apiresp` nor `opctx` imports
   `authz`. Confirm with the build check in `## Validation`.

7. **Doc-comment the function.** It must state: that it is an
   authentication-only check and explicitly *not* an operation or grant check;
   that it resolves the effective actor via `opctx.EffectiveActorEntityID`, so a
   sudo-assumed identity counts as authenticated; that it returns
   `apiresp.ErrUnauthenticated` — which HTTP handlers map to 401 — when no
   effective actor is present; and that it is a free function rather than an
   `Authorizer` method, with the "callers need no `Authorizer` instance and no
   implementation changes" rationale stated in one clause.

8. **Extend the package doc comment** at the top of `api/authz/authz.go`. It
   currently asserts "this package defines only the contract" and "Authorizer is
   intentionally narrow: one method". Both remain true of `Authorizer` itself but
   the first is no longer true of the package. Amend the package doc so it
   accurately describes a package that defines the contracts **and** one
   contract-adjacent helper that needs no implementation, while preserving the
   existing "Authorizer is intentionally narrow: one method" claim about the
   interface. Do not delete the existing explanation of why `operation` and
   `target` are explicit parameters.

9. **Add unit tests** to `api/authz/authz_test.go` (package `authz_test`,
   matching the existing external-test-package convention). Add a
   `TestRequireAuthenticated` function using `t.Run` subtests covering:
   - **Sudo actor set** — returns `nil`.
   - **Real actor only** — returns `nil`.
   - **Neither set** — returns a non-nil error, and the test asserts the **exact
     sentinel**: `errors.Is(err, apiresp.ErrUnauthenticated)` must be true. A
     bare non-nil assertion is insufficient and does not satisfy this task.

   The failure subtest should additionally assert the returned error is not
   `apiresp.ErrForbidden`, so a copy-paste of the wrong sentinel is caught.
   `errors` is already imported by the test file; add `apiresp` and `opctx`
   imports as needed.

10. **Leave the existing tests intact.** `stubAuthorizer`, the compile-time
    assertion `var _ authz.Authorizer = (*stubAuthorizer)(nil)`, and the three
    existing test functions must all remain — they are the regression guard
    proving the interface signature did not change.

11. **Update `AGENTS.md`.** In the "Key types and packages" table, the `authz/`
    row currently reads:

    > `Authorizer` and `OpResolver` interfaces. Implementations are
    > consumer-supplied (e.g. mod-authz); this package defines only the
    > contracts.

    That last clause becomes false with this change. Amend the row to name
    `RequireAuthenticated`, describe it as an authentication-only free function
    returning `apiresp.ErrUnauthenticated`, and state that it is deliberately not
    an `Authorizer` method so existing implementations are unaffected. Keep the
    row a single table cell on one line, matching every other row; do not
    restructure the table. Task 001 amends the `opctx/` row in the same table —
    if that row already names `EffectiveActorEntityID`, leave it as-is.

## Validation

- `cd api && gofmt -l authz` prints nothing.
- `cd api && go vet ./authz/...` is clean.
- `cd api && go build ./...` succeeds. This is the import-cycle check — a cycle
  would fail here with an explicit `import cycle not allowed` diagnostic.
- `cd api && go test ./authz/...` passes, including `TestRequireAuthenticated`
  and the three pre-existing tests.
- `cd api && make test` passes (module-wide `go test ./...`). If dependency
  download is unavailable, the narrower package test above is the minimum bar;
  report the module-wide result either way rather than silently skipping it.
- `cd api && make lint` (`go vet ./...` plus the `gofmt` check) is clean.
- `git diff api/authz/authz.go` shows the `Authorizer` and `OpResolver`
  interface declarations unchanged: the `Authorize(ctx context.Context,
  operation string, target *int64) error` and `SatisfiedBy(slug string)
  ([]int32, error)` method lines must appear in no diff hunk as modified lines.
- `grep -rn "errors.New" api/authz/` returns no new sentinel declaration in
  non-test files.
- `grep -rn "RequireAuthenticated" api/` shows hits only in
  `api/authz/authz.go` and `api/authz/authz_test.go`.
- `git diff --stat` shows exactly three changed files: `api/authz/authz.go`,
  `api/authz/authz_test.go`, and `AGENTS.md`.
- Sanity-check that peer implementers still compile against the unchanged
  interface by confirming `api/authz/authz_test.go`'s
  `var _ authz.Authorizer = (*stubAuthorizer)(nil)` assertion is still present
  and the package still builds.

## Metadata

architectural_impact: true

## Assumptions

- Task 001 has landed and `opctx.EffectiveActorEntityID(ctx) (int64, bool)`
  exists in `api/opctx/opctx.go`.
- Go dependencies may not be installed in this checkout; a first build may need
  `go mod download`. `api/go.mod` carries a local `replace` for
  `github.com/moduleforge/core-model` pointing at `../model`, which is present.
- `docs/mf-standards/` is an **empty directory** in this worktree — an
  uninitialized git submodule pointing at a separate repository. Do not attempt
  to read or edit anything under it; everything needed from the canonical
  authorization design doc is reproduced in the references below.
- All pre-existing tests in `api/authz/authz_test.go` pass before this task
  starts.

## References

- `api/authz/authz.go` — the file being edited. Package doc lines 1-16;
  `Authorizer` at lines 43-45; `OpResolver` at lines 58-60.
- `api/authz/authz_test.go` — the test file being extended; its package comment
  explains that the existing tests exist to catch accidental mutation of the
  interface signature.
- `api/apiresp/errors.go` — the canonical sentinel block; `ErrUnauthenticated`
  is line 21.
- `api/apiresp/writer.go` — proves `apiresp` already imports `opctx`, which is
  what makes the `authz` → `apiresp` edge cycle-free.
- `plan/notes/naming-and-placement.md` — naming, signature, and placement
  decisions with the evidence extracted from
  `docs/mf-standards/architecture/authorization-design.md`. **Read this instead
  of the submodule doc**, which is not checked out here. Note in particular the
  design doc's rule that HTTP handlers map `ErrUnauthenticated`/`ErrForbidden`
  to 401/403.
- `plan/notes/submodule-doc-constraint.md` — why the canonical architecture doc
  is unreadable from this worktree and must not be edited by this task.
- `AGENTS.md` — "Key types and packages" section, `authz/` row.

## Procedure

1. Read `api/authz/authz.go` and `api/authz/authz_test.go` in full. Confirm
   `opctx.EffectiveActorEntityID` exists in `api/opctx/opctx.go`; halt and report
   if it does not.
2. Add the `opctx` and `apiresp` imports.
3. Add `RequireAuthenticated` with its doc comment, after both interfaces.
4. Amend the package doc comment per requirement 8.
5. Add `TestRequireAuthenticated` with its three subtests, including the
   `errors.Is(err, apiresp.ErrUnauthenticated)` assertion.
6. Run `gofmt -l authz`, `go vet ./authz/...`, `go build ./...`,
   `go test ./authz/...`, then `make test` and `make lint` from `api/`.
7. Update the `authz/` row in `AGENTS.md`.
8. Run the `git diff` and `grep` checks from `## Validation`.

## Checkpoint hints

- After adding `RequireAuthenticated` and confirming `go build ./...` succeeds
  (the import-cycle gate).
- After adding `TestRequireAuthenticated` and `go test ./authz/...` passes.
- After updating the `authz/` row in `AGENTS.md`.

## Status

**Outcome:** succeeded — 2026-08-15.

Added `RequireAuthenticated(ctx context.Context) error` to
`api/authz/authz.go`, placed after both the `Authorizer` and `OpResolver`
interface declarations, delegating to `opctx.EffectiveActorEntityID` (task
001) and returning the bare `apiresp.ErrUnauthenticated` sentinel when no
effective actor is present. The `authz`, `apiresp`, and `opctx` imports were
added as specified; no import cycle resulted. The package doc comment was
amended to describe the package as defining the `Authorizer`/`OpResolver`
contracts plus the new contract-adjacent free function, while preserving the
"Authorizer is intentionally narrow: one method" claim and the existing
explanation of why `operation`/`target` are explicit parameters. The
`Authorizer` and `OpResolver` interface declarations were not touched.

Added `TestRequireAuthenticated` to `api/authz/authz_test.go` (package
`authz_test`) with three subtests: sudo actor set, real actor only, and
neither set. The failure subtest asserts `errors.Is(err,
apiresp.ErrUnauthenticated)` and also asserts `!errors.Is(err,
apiresp.ErrForbidden)`. All three pre-existing tests and the
`stubAuthorizer`/compile-time-assertion regression guard remain unchanged.

Updated the `authz/` row of AGENTS.md's "Key types and packages" table to
name `RequireAuthenticated` and its rationale for being a free function
rather than an `Authorizer` method. The `opctx/` row was already corrected by
task 001.

**Validation:** all checks in `## Validation` passed — `gofmt -l authz`
(clean), `go vet ./authz/...` (clean), `go build ./...` (clean, confirming no
import cycle), `go test ./authz/...` (`TestRequireAuthenticated` and the
three pre-existing tests all pass), `make test` (module-wide `go test ./...`,
all packages pass), `make lint` (clean), the `git diff` interface-line check
(no `Authorize(ctx ...)` or `SatisfiedBy(slug ...)` lines appear as modified),
`grep -rn "errors.New" api/authz/` (no new sentinel in non-test files),
`grep -rn "RequireAuthenticated" api/` (hits only in `authz.go` and
`authz_test.go`), `git diff --stat` (exactly three files:
`api/authz/authz.go`, `api/authz/authz_test.go`, `AGENTS.md`), and the
`stubAuthorizer` compile-time assertion is present and the package builds.

**Affected source files:**
- `api/authz/authz.go`
- `api/authz/authz_test.go`
- `AGENTS.md`

No `## Assumptions` were overridden; all four listed assumptions held as
stated (task 001 landed, dependencies were already installed in this
worktree, `docs/mf-standards/` was not read, pre-existing tests passed before
this task began).
