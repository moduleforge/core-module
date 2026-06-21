---
phase: 3
task: "001"
slug: verify-build-and-typecheck
title: Verify build and typecheck pass with bun
status: todo
tier: sonnet-low
depends_on:
  - phase-02/001
---

# Task: Verify build and typecheck pass with bun

## Purpose and scope

Run the full build and typecheck pipeline via the root Makefile (which now delegates to bun) to confirm the migration is clean. No code changes — observation and validation only.

## Requirements

1. `make build` exits 0 with no errors.
2. `make test` (which runs `bun run typecheck` in `gui/`) exits 0 with no TypeScript errors.
3. `dist/` is produced in `gui/dist/` with expected output files (`index.js`, `index.mjs`, `index.d.ts`).
4. No npm-related warnings or errors appear in output.

## Steps

```bash
cd /Users/zane/playground/moduleforge/core-module
make build
make test
ls gui/dist/
```

If any step fails, capture the error output and report it for diagnosis before attempting fixes. Do not apply fixes in this task — open a follow-up task if needed.

## Validation

- [ ] `make build` exits 0
- [ ] `make test` exits 0
- [ ] `gui/dist/index.js` exists
- [ ] `gui/dist/index.mjs` exists
- [ ] `gui/dist/index.d.ts` exists
- [ ] No `npm` warnings in output (e.g. "npm warn" lines)

## Status

outcome: validation failed
date: 2026-06-21
worktree: /Users/zane/playground/moduleforge/core-module/worktree/phase-03-task-01-verify-build-and-typecheck-pas

### Validation summary

- `make build` — FAILED (exit 2). `gui/` build step fails: `tsup: command not found` (exit 127). Root cause: `gui/node_modules` is not installed; bun dependencies were never installed in the worktree.
- `make test` — FAILED (exit 2). Go tests for `model` and `api` all pass. `gui/` typecheck step fails: `tsc: command not found` (exit 127). Same root cause: `node_modules` absent.
- `gui/dist/index.js` — does not exist (no build ran).
- `gui/dist/index.mjs` — does not exist.
- `gui/dist/index.d.ts` — does not exist.
- No `npm` warnings — not applicable (build did not reach any npm invocation; no `npm warn` lines observed).

### Root cause

`gui/node_modules` is absent. `tsup` and `tsc` are devDependencies resolved from the local node_modules tree (via `bun.lock`). Without `bun install` having been run inside `gui/`, the bun shims for these binaries do not exist and both `make build` and `make test` fail at the `gui/` step with exit 127.

### Flagged for manager

- `gui/` bun dependencies must be installed (`cd gui && bun install`) before this verification can pass. The `dependencies_installed` pass-through for this task was `none`, which does not cover the bun/JavaScript side of the project.
- Consider whether `prepare-task-worktree` should install bun dependencies (run `bun install`) for mixed Go+Bun projects, or whether a follow-up task should explicitly install them as a prerequisite step.
- Go sub-projects (`model`, `api`) built and tested cleanly — no issues on that side.
