---
phase: 2
task: "001"
slug: update-npm-to-bun
title: Replace npm with bun in root Makefile
status: todo
tier: haiku-low
depends_on:
  - phase-01/001
---

# Task: Replace npm with bun in root Makefile

## Purpose and scope

Update `core-module/Makefile` to replace all npm invocations with bun equivalents:
- `npm run build` → `bun run build`
- `npm run typecheck` → `bun run typecheck`
- `npm run clean` → `bun run clean`
- `npm install --silent` → `bun install --silent`
- `npm run dev` → `bun run dev`

## Requirements

1. All `npm run <script>` calls in the Makefile become `bun run <script>`.
2. `npm install` becomes `bun install` (preserve any flags such as `--silent`).
3. No other Makefile logic changes — only the package manager invocations.
4. The file must remain valid GNU make syntax.

## Steps

Edit `/Users/zane/playground/moduleforge/core-module/Makefile`:

Current lines to change:
```makefile
@cd $(GUI_DIR) && npm run build
@cd $(GUI_DIR) && npm run typecheck
@cd $(GUI_DIR) && npm run clean
@cd $(GUI_DIR) && npm install --silent
@cd $(GUI_DIR) && npm run dev
```

Replace with:
```makefile
@cd $(GUI_DIR) && bun run build
@cd $(GUI_DIR) && bun run typecheck
@cd $(GUI_DIR) && bun run clean
@cd $(GUI_DIR) && bun install --silent
@cd $(GUI_DIR) && bun run dev
```

Then commit:
```bash
git -C /Users/zane/playground/moduleforge/core-module add Makefile
git -C /Users/zane/playground/moduleforge/core-module commit -m "chore: replace npm with bun in root Makefile"
```

## Validation

- [ ] `grep -n 'npm' Makefile` returns no matches
- [ ] `grep -n 'bun' Makefile` shows all expected substitutions
- [ ] `make --dry-run build` prints bun invocations (not npm)
