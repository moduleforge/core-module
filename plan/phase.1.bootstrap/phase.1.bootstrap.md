# Phase 1 — Bootstrap core-module skeleton

## Goal
Create empty scaffolds for `core-module/model`, `core-module/api`, `core-module/gui`; stitch Go modules into a top-level `go.work`; set up root `Makefile` with `link-core` for yalc; land a root README. After this phase, all three core-module packages compile/build clean but are empty of business logic.

## Outputs
- `user-components/go.work` (committed)
- `user-components/Makefile` with `link-core`, `unlink-core`, `build`, `test`, `clean` aggregators
- `user-components/README.md`
- `core-module/model/{go.mod, atlas.hcl, sqlc.yaml, Makefile, .gitignore}`
- `core-module/api/{go.mod, Makefile, audit/doc.go, service/doc.go, httpapi/doc.go}`
- `core-module/gui/{package.json, tsconfig.json, tsup.config.ts, src/index.ts, .gitignore, README.md}`

## Hard rules
- Module paths:
  - Go: `github.com/moduleforge/core-model`, `github.com/moduleforge/core-api`.
  - Node: `@moduleforge/core-gui`.
- `core-module/model/go.mod` pins same Go version as `users-module/api/go.mod`.
- `go.work` includes: `./core-module/model`, `./core-module/api`, `./users-module/api`, `./users-module/model`.
- `core-module/gui/package.json` declares `peerDependencies` on `react`, `react-dom` (same versions as users-module/gui).
- No runtime code in this phase — only scaffolding. Placeholder `doc.go` files per Go package.

## Tasks
- 1.1 Create top-level `go.work`
- 1.2 Scaffold `core-module/model/`
- 1.3 Scaffold `core-module/api/`
- 1.4 Scaffold `core-module/gui/`
- 1.5 Root Makefile
- 1.6 Root README

## How to verify
- `go work sync` succeeds at repo root.
- `cd core-module/model && go build ./...` → clean (no packages).
- `cd core-module/api && go build ./...` → clean (empty packages).
- `cd core-module/gui && pnpm install && pnpm run build` → produces `dist/index.js`.
- `make link-core` from repo root succeeds (yalc publish + add into users-module/gui).

## Notes
- yalc must be installed globally (`npm i -g yalc`) — document in root README.
- `core-module/gui` uses plain npm (not pnpm) since we're staying off the pnpm workspace. Doc the command users should run.
