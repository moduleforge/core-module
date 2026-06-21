---
plan: bun-migration
created: 2026.06.21
status: active
---

# Plan: Migrate core-module/gui from npm to bun

## Purpose and scope

Replace npm as the package manager for `core-module/gui` with bun. This eliminates `package-lock.json` in favour of `bun.lock`, updates all Makefile delegate commands from `npm` to `bun`, and verifies the build and typecheck pipeline remains green.

**In scope:**
- `gui/package-lock.json` → deleted; `gui/bun.lock` generated
- `core-module/Makefile` npm references → replaced with bun equivalents
- Smoke-testing build and typecheck with bun

**Out of scope:**
- Migrating `api/` or `model/` (Go subprojects, not affected)
- Changing any package versions or dependencies

## Current status

Not started.

## Overview

### Phase 1 — Lockfile migration

Remove `gui/package-lock.json` and run `bun install` inside `gui/` to generate `bun.lock`. Commit the result.

**Tasks:**
1. `001-replace-lockfile` — Delete `gui/package-lock.json`, run `bun install`, commit `gui/bun.lock`

### Phase 2 — Makefile update

Update `core-module/Makefile` to replace all `npm install` and `npm run <script>` invocations with their `bun` equivalents.

**Tasks:**
1. `001-update-npm-to-bun` — Replace `npm install` → `bun install` and `npm run` → `bun run` in root Makefile

### Phase 3 — Verify

Run `make build` and `make test` (which calls `npm run typecheck` via Makefile, now using bun) to confirm the migration is clean.

**Tasks:**
1. `001-verify-build-and-typecheck` — Run `make build` and `make test` against the migrated repo; confirm zero errors
