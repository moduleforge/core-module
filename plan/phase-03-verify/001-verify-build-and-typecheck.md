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
