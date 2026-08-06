# Bump Mf Standards Submodule Pointer

## Purpose and scope

Advance the `docs/mf-standards` git submodule pointer to the commit that carries the sibling
`docs-mf-standards` project's updated `architecture/gui-design-tokens.md`, so mod-core consumers
actually see the architecture documentation for the token category this plan built.

This is a one-line change to a gitlink. No file content is edited.

## Sequencing — read this before starting

**This task is explicitly last in the plan, and it is blocked on an event outside this project's task
graph.** The `docs-mf-standards` project is the companion half of this same federated plan
(slug `gui-spacing-tokens`); its documentation-only work must be **merged to its own default branch**
before this task can run. Nothing in mod-core can detect or wait on that; the plan executor confirms
it and only then dispatches this task.

If the sibling's work has not landed, **halt and report** with that as the reason. Do not bump the
pointer to a commit that does not contain the update, and do not bump it "to latest" on the
assumption that it must be there by now.

## Requirements

1. **Confirm the current pin.** `git submodule status` should report
   `1ab046e0b1f710497dcf81013bf9ab8fea3b479f` for `docs/mf-standards` (the pin recorded at planning
   time). If it reports a different SHA, someone has moved it since; establish why before proceeding,
   and halt and report if the reason is not obvious and benign.

2. **Fetch and identify the target commit.**

   ```sh
   cd docs/mf-standards
   git fetch origin
   git log --oneline origin/HEAD -- architecture/gui-design-tokens.md | head
   ```

   The target is the commit on the submodule's default branch that contains the updated
   `architecture/gui-design-tokens.md`. Prefer the branch tip over an arbitrary intermediate commit —
   a submodule pin should sit on a merged, published commit, not a detached mid-history one.

3. **Verify the target actually contains the update** before pinning to it. Check out or inspect the
   file at that commit and confirm it describes the spacing / container-width token category — the
   `--mf-max-content-width` and `--mf-content-margins-*` roles, the per-band override mechanism, and
   the `@utility container` integration. If it does not, the sibling's work has not landed on that
   branch; halt and report.

4. **Move the pointer and stage only the gitlink.**

   ```sh
   cd docs/mf-standards && git checkout <target-sha>
   cd ../.. && git add docs/mf-standards
   ```

   `git diff --cached` must show a single `Subproject commit` line change and nothing else.

5. **Verify the working tree is consistent.** `git submodule status` reports the new SHA with no `+`
   or `-` prefix (no uncommitted submodule changes, not uninitialized).

6. **Check for newly-broken inbound links.** mod-core's own docs link into the submodule
   (`README.md` and `AGENTS.md` both link `docs/mf-standards/manifest-spec.md`;
   `gui/tokens/STYLE-PACKAGE-CONTRACT.md` links `../../docs/mf-standards/manifest-spec.md` and
   `../../docs/mf-standards/architecture.md`). A submodule bump can move or rename files. Verify
   every mod-core-side link into `docs/mf-standards/` still resolves at the new commit:

   ```sh
   grep -rn 'docs/mf-standards/[A-Za-z0-9._/-]*' --include='*.md' . \
     --exclude-dir=node_modules --exclude-dir=worktrees --exclude-dir=.git
   ```

   Check each distinct target path exists. Report any that no longer does; do not silently repoint
   it — a broken link after a submodule bump is a signal worth surfacing.

### Do not

- Do not edit any file inside `docs/mf-standards/`. This project may move the pointer; it may never
  change the submodule's content. Content changes belong to the `docs-mf-standards` project.
- Do not commit any change inside the submodule, or push from inside it.
- Do not bump the pointer to an unpushed local commit, or to a commit on a feature branch that has
  not merged.
- Do not modify `.gitmodules` — the URL and path are correct and unchanged.
- Do not stage anything besides the `docs/mf-standards` gitlink.

## Validation

1. `git submodule status` reports the new target SHA, with no `+`/`-`/`U` status prefix.
2. `git diff --cached --stat` shows exactly one changed path, `docs/mf-standards`, and
   `git diff --cached` shows only a `Subproject commit` line change.
3. `docs/mf-standards/architecture/gui-design-tokens.md`, read at the new pin, describes the spacing
   / container-width token category — `--mf-max-content-width`, `--mf-content-margins-lr` /
   `-tb`, the per-band override mechanism, and the `@utility container` integration.
4. Its account does not contradict `gui/tokens/CONTRACT.md`. Spot-check the role names and the
   precedence description. Report any contradiction as a flagged item; do not edit either side.
5. Every mod-core-side markdown link into `docs/mf-standards/` resolves at the new pin.
6. `git status` shows no modified or untracked file inside `docs/mf-standards/`.
7. `cd gui && bun run test` and `cd gui && bun run typecheck` still pass — a submodule bump should not
   affect them, and confirming that is cheap.

## Metadata

architectural_impact: true

## Assumptions

- The `docs-mf-standards` project's plan for slug `gui-spacing-tokens` has completed and merged. The
  plan executor has confirmed this; this task does not and cannot verify it from inside mod-core
  other than by inspecting the submodule's published history.
- The submodule remote (`git@github.com:moduleforge/docs-mf-standards.git`) is reachable from this
  environment. If `git fetch` fails on credentials or network, halt and report — do not work around
  it by pinning to a local commit.

## References

- `.gitmodules` — declares `docs/mf-standards` → `git@github.com:moduleforge/docs-mf-standards.git`.
- `docs/mf-standards/architecture/gui-design-tokens.md` — the doc whose update this bump makes
  visible. **Read-only.**
- [`plan/overview.md`](../overview.md), "This is one half of a federated plan" — the cross-repository
  relationship and the direction of dependency.
- `gui/tokens/CONTRACT.md` — the shipped contract the architecture doc is checked against.
