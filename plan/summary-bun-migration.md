---
plan: bun-migration
created: 2026.06.21
status: completed
date_summary: 2026.06.21
---

# Plan Session Summary: Migrate core-module/gui from npm to bun

## What was planned and why

**Goal:** Replace npm as the package manager for `core-module/gui` with bun, eliminating `package-lock.json` in favour of `bun.lock`, updating all Makefile delegate commands from `npm` to `bun`, and verifying the build and typecheck pipeline remains green.

**Motivation:** Streamline JavaScript dependency management and build tooling by adopting bun as a faster, unified package manager and runtime for the GUI subproject.

**Scope:** Three distinct phases — lockfile migration, Makefile updates, and verification — covering only the TypeScript GUI portion; Go subprojects (`api/`, `model/`) intentionally excluded.

## What shipped

### Phase 1 — Lockfile migration

**Task:** `001-replace-lockfile` — Delete `gui/package-lock.json`, run `bun install`, commit `gui/bun.lock`

**Outcome:** Succeeded — merge `ef9fbc5`

- `gui/package-lock.json` deleted from git tracking
- `gui/bun.lock` generated and committed; 731 packages installed in 2.80s
- `gui/.gitignore` confirmed not to exclude `bun.lock`
- Pre-existing peer dependency warning on react@19.2.7 (not introduced by this task)
- **Note:** bun re-resolved 208 packages to newer semver-compatible versions (react 19.2.5→19.2.7, radix-ui 1.4.3→1.6.0, tailwindcss 4.2.4→4.3.1, shadcn 4.4→4.11). Reviewed and accepted as incidental upgrades within declared ranges.

### Phase 2 — Makefile update

**Task:** `001-update-npm-to-bun` — Replace all npm invocations in root Makefile with bun equivalents

**Outcome:** Succeeded — merge `2d85ee4`

- All 5 npm invocations replaced: `npm run` → `bun run`, `npm install` → `bun install`
- No Makefile logic or structure changes; all Go targets unaffected
- `grep -n 'npm' Makefile` returns no matches

### Phase 3 — Verify

**Task:** `001-verify-build-and-typecheck` — Run `make build` and `make test` to confirm migration is clean

**Outcome:** Succeeded (no code changes — verification only)

- `make build` exits 0: model and api build cleanly; `gui/` produces `dist/index.js`, `dist/index.mjs`, `dist/index.d.ts` via tsup
- `make test` exits 0: 11 Go test packages pass; `tsc --noEmit` passes with no TypeScript errors
- No npm references or warnings in output
- **Note:** Worktree required `cd gui && bun install` before verification could run; `node_modules` is gitignored and must be installed in fresh worktrees

## Key decisions

1. **Accept incidental version upgrades:** bun resolved packages to newer semver-compatible versions; this was reviewed and accepted rather than attempting to freeze exact npm-pinned versions.
2. **Makefile-only substitution:** Phase 2 changed only package manager invocations, preserving all Makefile structure and Go build targets.
3. **Observation-only Phase 3:** Verification was designed as a smoke test with no fixes; the dependency-install prerequisite gap was handled by the manager, not a task agent.

## Follow-up items

- **bun postinstall blocked script:** `bun install` in `gui/` reports one blocked postinstall script. Run `bun pm untrusted` in `gui/` to inspect if trust action is needed. (Pre-existing condition, not introduced by this migration.)
- **Fresh-worktree install note:** For mixed Go+Bun repos, `bun install` must be run inside `gui/` after provisioning a new worktree — the create-worktree script detects lockfiles at the repo root only.
