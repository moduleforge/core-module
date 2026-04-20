# Phase 6, Task 5 — yalc publish

## Context
`yalc publish` stages the built package for local consumption.

## Acceptance
- `cd core-module/gui && yalc publish` succeeds.
- `~/.yalc/packages/@moduleforge/core-gui/` (global yalc store) contains the built package.

## How to verify
- `yalc installations show @moduleforge/core-gui` lists the expected version.
- No publish errors.

## Notes
- If version bumping is relevant, `yalc publish --push` re-publishes. For Phase 6, a single initial publish is enough.
