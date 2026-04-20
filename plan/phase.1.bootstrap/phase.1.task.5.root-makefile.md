# Phase 1, Task 5 — Root Makefile

## Context
The repo root needs an orchestrator Makefile that provides cross-module conveniences without replacing sub-project Makefiles.

## Acceptance
`user-components/Makefile` implementing:

- `make link-core` — in one shot:
  1. `cd core-module/gui && npm install && npm run build && yalc publish`
  2. `cd users-module/gui && yalc add @moduleforge/core-gui` (if not already added)
  3. `cd users-module/gui && yalc update`
  4. `go work sync` at repo root.

- `make unlink-core` — removes yalc linkage from users-module/gui (`yalc remove @moduleforge/core-gui`).

- `make build` — fan out to `$(MAKE) -C <sub> build` for each of core-module/model, core-module/api, core-module/gui, users-module/model, users-module/api, users-module/gui.

- `make test` — same fanout for `test`.

- `make clean` — fanout for `clean`.

- `make help` — lists the above.

- GNU make guard at the top:
  ```makefile
  ifeq ($(filter undefine override,$(value .FEATURES)),)
  $(error GNU make 4.x+ required; on macOS: brew install make && use gmake)
  endif
  ```

## How to verify
- `make help` lists commands.
- `make build` exits 0 (may be mostly no-op since modules are empty).
- `make link-core` publishes + consumes in users-module/gui.

## Notes
- If users-module has a similar root Makefile already, replace or merge carefully.
- Don't call `make dev.start` here — leave that to users-module/Makefile.
