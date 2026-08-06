# Update Architecture Docs

## Purpose and scope

Reconcile mod-core's architecture-level documentation with the spacing / container-width token
category built in Phases 1 and 2, and verify — **read-only** — that the submodule-mounted
architecture doc the sibling project owns describes what was actually built.

Follow the `update-architecture-docs` task-procedure at
`plugins/flow/task-procedures/update-architecture-docs/SKILL.md`.

**role_doc**: `plugins/flow/references/roles/architect-frontend.md` — the implications are frontend /
browser architecture (a CSS custom-property contract surface, a Tailwind utility definition, and a
package stylesheet export), not backend, data, or infrastructure.

### The unusual shape of this task, and why

mod-core has **no** `docs/architecture.md` and **no** `docs/*-spec.md`. Its architecture-level
documentation for this system lives in a git submodule at
`docs/mf-standards/architecture/gui-design-tokens.md`, owned by the separate `docs-mf-standards`
project — which is the companion half of this same federated plan and is updating that file itself.

So this task does **not** author the architecture doc. It does two things: brings mod-core's own
top-level docs into line, and acts as the verification gate that the sibling's account is accurate
before the submodule pointer moves (task `002`).

## Requirements

### Architectural implications that triggered this phase

The implementation task documents that surfaced them, all of which will have been completed by the
time this phase runs:

- `plan/phase-01-spacing-token-contract/001-add-spacing-token-sources.md` — a new token tier/category
  in the DTCG source layout.
- `plan/phase-01-spacing-token-contract/002-emit-spacing-tokens-and-container-utility.md` — a new
  emitted artifact kind (`@utility`) in the compiled bundle, and a behavior change to Tailwind's
  built-in `container` utility.
- `plan/phase-01-spacing-token-contract/004-export-token-source-css.md` — a new public package
  export (`@moduleforge/core-gui/tokens.css`) and a new intended consumption path.
- `plan/phase-02-contract-documentation/002-document-per-band-override-for-style-packages.md` — a new
  style-package capability (per-band override) and a MINOR contract-version bump.

The checklist items that fired: **modifies a public API or component boundary** (the `--mf-*`
contract surface and the package `exports` map).

### 1. Files to review and update in this repository

- **`gui/README.md`** — already updated by Phase 1 task `004` with the two-export story. Re-read it
  and confirm it is accurate against what shipped; extend only if the spacing token category itself
  needs a mention it does not yet have.
- **`README.md`** (repo root) — its `gui/` bullet describes `@moduleforge/core-gui` as "shared
  TypeScript/React components … including an error/toast widget toolkit". Judge whether the design
  token system, and the new spacing/container category specifically, warrants a mention at this level
  of summary. It is a legitimate outcome to conclude it does not — record the judgment either way.
- **`AGENTS.md`** — its "gui/ error and toast toolkit" subsection enumerates what `gui/` ships.
  There is currently no design-token subsection at all, which is a gap independent of this change.
  Add a short one covering `gui/tokens/` (the DTCG sources), `gui/style-dictionary/build-tokens.mjs`
  (the compiler), the two package stylesheet exports and when to use each, and pointers to
  `gui/tokens/CONTRACT.md` and `gui/tokens/STYLE-PACKAGE-CONTRACT.md`. Keep it to the register
  `AGENTS.md` uses — orientation and pointers, not a duplicate of the contract.

### 2. Read-only verification of the submodule-mounted architecture doc

`docs/mf-standards/architecture/gui-design-tokens.md` is submodule-mounted and owned by the sibling
`docs-mf-standards` project. **Do not edit it. Do not edit any file under `docs/mf-standards/`.**

If the sibling project's work has landed and the submodule is on a commit that includes it, read the
doc and verify against the shipped reality:

- Does its account of the token shape match `gui/tokens/CONTRACT.md` and the emitted
  `gui/tokens/dist/tokens.css`? Specifically: the role names, the per-band resolution chain and its
  precedence, and the `@utility container` integration.
- Does it correctly state the two container behavior changes (built-in max-width ladder overridden;
  `padding-block` added)?
- Does its "Source-of-truth documents in `mod-core`" list still resolve — every named file exists?
- Does its enumeration of the semantic surface ("35 color roles, one radius lever … ") acknowledge
  the new category?

Report every discrepancy in the task report as a flagged item for the manager to route back to the
sibling project. Do not attempt to fix any of them here.

If the submodule is still pinned at the pre-existing commit
(`1ab046e0b1f710497dcf81013bf9ab8fea3b479f`) and the sibling's update is therefore not visible, say so
explicitly in the task report and skip this verification — do not fabricate a result, and do not
move the pointer yourself. Moving it is task `002`'s job.

### Do not

- Do not edit anything under `docs/mf-standards/`.
- Do not move the submodule pointer — that is task `002`, deliberately sequenced after this.
- Do not re-edit `gui/tokens/CONTRACT.md`, `README.md`, or `STYLE-PACKAGE-CONTRACT.md`; Phase 2
  settled them. If one is wrong, report it rather than fixing it here, so the correction is visible.
- Do not create a `docs/architecture.md` in this repo. mod-core deliberately delegates
  architecture-level documentation to the `docs-mf-standards` submodule; introducing a competing
  local file would be a real architectural change, out of scope, and should be raised as a flagged
  item if you think it is warranted.

## Validation

1. `AGENTS.md` contains a design-token subsection naming `gui/tokens/`,
   `gui/style-dictionary/build-tokens.mjs`, both package stylesheet exports, and both contract docs.
2. Every file path named in the new `AGENTS.md` content exists — verify each.
3. Every relative link added to any edited file resolves.
4. `git status` shows **no** modification under `docs/mf-standards/` and **no** change to the
   submodule pointer (`git submodule status` reports the same SHA as before this task).
5. `git diff --stat` covers only the repo-root docs this task legitimately edits (`AGENTS.md`, and
   `README.md` / `gui/README.md` if changed).
6. The task report states, explicitly: whether the submodule was on a commit containing the sibling's
   update; and, if it was, each discrepancy found or an explicit statement that none were.
7. The judgment call on `README.md` (updated, or deliberately not) is recorded in the task report
   with its reasoning.

## Metadata

architectural_impact: true

## Assumptions

- Phases 1 and 2 have fully landed.
- The submodule may or may not yet be on a commit containing the sibling project's update; both
  states are handled above and neither is an error.

## References

- `plugins/flow/task-procedures/update-architecture-docs/SKILL.md` — the task-procedure to follow.
- `plugins/flow/references/roles/architect-frontend.md` — the role doc for this task.
- `docs/mf-standards/architecture/gui-design-tokens.md` — the architecture doc under verification.
  **Read-only.**
- `gui/tokens/CONTRACT.md`, `gui/tokens/STYLE-PACKAGE-CONTRACT.md`, `gui/tokens/README.md` — Phase 2's
  output; the authority the architecture doc is checked against.
- `gui/tokens/dist/tokens.css` — the emitted bundle; regenerate with
  `cd gui && bun install && bun run build:tokens` if absent (the directory is gitignored).
- [`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md) — the design record the
  sibling project transcribed from; useful for judging whether its transcription is faithful.
