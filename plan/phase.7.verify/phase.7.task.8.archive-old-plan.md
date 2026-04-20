# Phase 7, Task 8 — Archive prior schema-only plan content

## Context
`core-module/plan/summary.md` has been rewritten for the expanded scope. Any phase documents from the old schema-only plan should be cleaned up per `process.implement.md` ("Once a task is complete, task specific plans should be deleted").

## Acceptance
- After the phase is done, delete its phase directory (`core-module/plan/phase.N.*/`).
- `core-module/plan/` ends up containing only `summary.md`, `TODO.md`, and possibly `report.*.md` files noting anything worth remembering.
- Any surviving report files are prefixed with the phase number for searchability.

## How to verify
`ls core-module/plan/` after full cleanup shows only summary, TODO, and reports.

## Notes
- Alternatively, if the user prefers to keep the history, archive under `core-module/plan/archive/` instead of deleting. Default: delete, per process.implement.md.
