---
created: 2026.06.21
creators:
  - project-flow-check skill
notes: Session-binding manifest produced by `project-flow-check`. Regenerated on each run.
---

# Flow Skill Binding

## Project type

- language: typescript (gui/), go (api/, model/)
- framework: react (gui/)
- runtime: node (gui/ — migrating to bun); go toolchain (api/, model/)
- additional markers: Makefile present

## Build / test / run commands

| purpose | command | source |
|---------|---------|--------|
| build | `make build` | Makefile |
| test | `make test` | Makefile |
| clean | `make clean` | Makefile |
| dev / preview | `make preview` | Makefile |
| gui build | `cd gui && npm run build` | Makefile (delegates) |
| gui typecheck | `cd gui && npm run typecheck` | Makefile (delegates) |
| gui dev | `cd gui && npm run dev` | Makefile (delegates) |

## Layout conformance

| dimension | score | notes |
|-----------|-------|-------|
| standard doc set | partial | no root README.md; no AGENTS.md; docs/architecture.md present |
| docs/ discoverability | n/a | no docs/*-spec.md found |
| plan/ shape | n/a | no plan/ directory |
| make-layout | partial | build/test/clean targets present; no run/start target |

## Bound skill chain

- role docs: `references/role/developer-node.md`, `references/role/developer-go.md`
- doc-author skills: `write-readme`, `write-agents-md`, `write-architecture`
- implementation skills: `implement-task` (via `dispatch-implementation-task`)
- review skills: `review-changes-correctness`, `review-changes-style`, `review-changes-security`, `review-changes-efficiency`
- release skills: `package-release`, `coordinate-release`
- deploy / sunset / archive skills: none detected

## Link-chain status

- root: none (no README.md at project root)
- first-layer docs: n/a
- depth: 0
- orphans: next-steps.md, stories-next.md, docs/architecture.md, gui/README.md, model/README.md

## Open binding gaps

- README.md is absent at project root — link-chain root missing.
- AGENTS.md is absent — no documented build/run commands for agents.
- No `run`/`start` Makefile target (only `preview` available for gui).

## Active plans

| Slug | Branch | Worktree | Status |
|------|--------|----------|--------|
| bun-migration | plan/bun-migration | /Users/zane/playground/moduleforge/core-module/worktree/plan/bun-migration | healthy |
