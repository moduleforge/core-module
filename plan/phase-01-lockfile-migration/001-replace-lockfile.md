---
phase: 1
task: "001"
slug: replace-lockfile
title: Replace package-lock.json with bun.lock
status: todo
tier: haiku-low
---

# Task: Replace package-lock.json with bun.lock

## Purpose and scope

Delete `gui/package-lock.json` (the npm lockfile) and run `bun install` inside `gui/` to generate `gui/bun.lock`. Commit the result to the working branch.

This is a mechanical file-swap with no logic changes.

## Requirements

1. `gui/package-lock.json` must be deleted (not just overwritten).
2. `bun install` must complete without errors in `gui/`.
3. `gui/bun.lock` must be present and committed.
4. `gui/node_modules/` content should be consistent with the new lockfile (bun installs to node_modules by default).
5. `.gitignore` at `gui/` level must not exclude `bun.lock` — verify and adjust if needed.

## Steps

```bash
cd /Users/zane/playground/moduleforge/core-module/gui
rm package-lock.json
bun install
# Verify bun.lock was generated
ls bun.lock
```

Then stage and commit:
```bash
git -C /Users/zane/playground/moduleforge/core-module add gui/bun.lock
git -C /Users/zane/playground/moduleforge/core-module rm gui/package-lock.json
git -C /Users/zane/playground/moduleforge/core-module commit -m "chore(gui): replace package-lock.json with bun.lock"
```

## Validation

- [ ] `gui/package-lock.json` does not exist in the repo
- [ ] `gui/bun.lock` exists and is tracked by git
- [ ] `bun install` exits 0 with no error output
- [ ] `node_modules/` was populated (spot-check: `ls gui/node_modules/.bin/tsup`)
