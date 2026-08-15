# Submodule constraint on the canonical architecture doc

## Purpose and scope

Records a structural constraint discovered during planning that the
`doc-updates` phase must work within: mod-core's canonical architecture
documentation is a git submodule pointing at a different repository, and it is
not checked out inside this plan worktree.

## Findings

- `mod-core` has **no** `docs/architecture.md` and **no** `docs/*-spec.md` at
  its own project root. `docs/` contains exactly one entry: `mf-standards`.
- `.gitmodules` declares:

  ```
  [submodule "docs/mf-standards"]
  	path = docs/mf-standards
  	url = git@github.com:moduleforge/docs-mf-standards.git
  ```

- In the main checkout (`/Users/zane/playground/moduleforge/mod-core`) the
  submodule is populated: `docs/mf-standards/architecture/authorization-design.md`
  exists and is the canonical Authorizer-contract document read during planning.
- In this plan worktree, `docs/mf-standards/` is an **empty directory** — a
  fresh `git worktree` does not check out submodule content. The same will be
  true of any task worktree provisioned from this repository unless
  `git submodule update --init docs/mf-standards` is run inside it.

## Consequences for the plan

1. No phase-01 implementation task may depend on reading a file under
   `docs/mf-standards/` from inside its own worktree. Every fact the
   implementing agents need from `authorization-design.md` has been extracted
   into [`naming-and-placement.md`](./naming-and-placement.md) instead.
2. Editing `authorization-design.md` is a **cross-repository** change: it means
   committing in `docs-mf-standards` and then advancing mod-core's submodule
   pointer. That is materially different from an ordinary in-repo doc edit and
   is not something a task agent should attempt unprompted.
3. mod-core's own in-repo documentation surface for these packages is
   `AGENTS.md`'s "Key types and packages" table. Its `opctx/` and `authz/` rows
   both become factually stale the moment the new symbols land — in particular
   the `authz/` row's claim that the package "defines only the contracts".
   Those row updates are therefore folded into the phase-01 tasks that create
   the symbols, so mod-core never sits in a self-inconsistent state, rather
   than being deferred to the `doc-updates` phase.

The `doc-updates` phase consequently scopes to: verifying the AGENTS.md rows
landed correctly in phase 01, and assessing — then either performing under
explicit manager direction, or reporting as a flagged cross-repo item — the
`authorization-design.md` update.
