# Wire GUI Tests Into Root Test Target

## Purpose and scope

Wires the new `gui/` `bun run test` script (added by task 001) into the root `Makefile`'s `test` target — today it only typechecks `gui/`, per the header comment `test     — run unit tests on model and api; typecheck gui` — and updates `AGENTS.md`'s "Build commands" and "Test commands" sections to describe the new gui test step, retiring the now-inaccurate "gui is typecheck-only" framing.

Depends on task 001 (the `gui/` `"test"` script must exist for `bun run test` to be a valid command). Benefits from, but does not hard-depend on, task 002's test files — a zero-test `bun test`/`bun run test` run exits `0`, which is a valid (if weaker) signal that the wiring itself is correct; if task 002 has already landed, prefer validating against its real test output.

This is a config/docs task, not covered by a standard skill — implement directly per the Requirements below.

## Requirements

1. **Root `Makefile`** (`Makefile` at the project root, *not* `model/Makefile` or `api/Makefile` — those are owned by `@sdlcforge/gen-make` and must not be touched):

   a. In the header comment block's target list, change:
      ```
      #   test     — run unit tests on model and api; typecheck gui
      ```
      to:
      ```
      #   test     — run unit tests on model, api, and gui; typecheck gui
      ```

   b. In the `.PHONY: test` / `test:` target, change:
      ```make
      .PHONY: test
      test: ## Test model and api; typecheck gui
      	@for d in $(GO_SUBPROJECTS); do \
      		echo "==> test: core-module/$$d"; \
      		$(MAKE) --no-print-directory -C $$d test; \
      	done
      	@echo "==> typecheck: core-module/$(GUI_DIR)"
      	@cd $(GUI_DIR) && bun run typecheck
      ```
      to:
      ```make
      .PHONY: test
      test: ## Test model, api, and gui; typecheck gui
      	@for d in $(GO_SUBPROJECTS); do \
      		echo "==> test: core-module/$$d"; \
      		$(MAKE) --no-print-directory -C $$d test; \
      	done
      	@echo "==> test: core-module/$(GUI_DIR)"
      	@cd $(GUI_DIR) && bun run test
      	@echo "==> typecheck: core-module/$(GUI_DIR)"
      	@cd $(GUI_DIR) && bun run typecheck
      ```
      i.e. add a `bun run test` step for `gui/` immediately before the existing `bun run typecheck` step, and update the target's `##`-comment (which doubles as the `make help` description, parsed by the `help` target's `awk` line) to match. Keep the existing `bun run typecheck` step — this task adds test execution, it does not replace typechecking.

2. **`AGENTS.md`** (project root):

   a. In the "Build commands" section's fenced `sh` block, change:
      ```
      make test            # unit-test api; typecheck gui (model has no unit tests)
      ```
      to:
      ```
      make test            # unit-test api; unit-test + typecheck gui (model has no unit tests)
      ```
      Leave the `make build` and `make clean` lines in that block unchanged.

   b. In the "Test commands" section's fenced `sh` block, change:
      ```
      make test                    # unit tests (api) + gui typecheck
      ```
      to:
      ```
      make test                    # unit tests (api) + gui test + gui typecheck
      ```
      and add two new lines for direct per-subproject invocation, matching the existing `cd api && ...` / `cd model && ...` style already present in that block (place them adjacent to the existing `cd api && make test` line, and align the `#` comment column with the surrounding lines in that block):
      ```
      cd gui && bun run test        # bun test (component/unit tests)
      cd gui && bun run typecheck   # tsc --noEmit
      ```

3. Do not modify any other section of `AGENTS.md`, `model/Makefile`, or `api/Makefile`.

## Validation

- `make help` (from the project root) — confirm the `test` target's listed description now reads `Test model, api, and gui; typecheck gui` (or your updated wording) rather than the old `Test model and api; typecheck gui`.
- `cd gui && bun install && bun run test` — confirm this succeeds standalone (should already be true from tasks 001/002; re-confirming here isolates any issue to the Makefile wiring itself if `make test` below fails but this step succeeds).
- `make test` (from the project root) — confirm the gui portion (`==> test: core-module/gui` followed by the `bun run test` output, then `==> typecheck: core-module/gui`) runs and passes. The `model`/`api` portions of `make test` are pre-existing and out of this task's scope — if they fail for reasons unrelated to this change (e.g. missing Go toolchain or Docker in the execution environment), note that in your task report rather than treating it as a validation failure of this task, but do not skip attempting the full `make test` run.
- `git diff -- Makefile AGENTS.md` — confirm the diff is limited to the lines described above; no unrelated reflow or whitespace changes.
- Grep sanity check: `grep -n "typecheck gui$" Makefile AGENTS.md` should no longer match the old "test-is-typecheck-only" framing in the header comment / Build commands line (a `typecheck gui` substring may still legitimately appear elsewhere, e.g. in the updated `test + typecheck gui` phrasing — verify by eye, don't over-trust a bare substring match).

## References

- Root `Makefile` — target of edit (a) and (b) above.
- `AGENTS.md` — "Build commands" (around the `make test` line) and "Test commands" sections — target of edit 2 above.
- [`plan/overview.md`](../overview.md) — plan-level context and the follow-up (`vr20`) this plan resolves.

## Metadata

architectural_impact: false

## Status

- **Outcome:** succeeded
- **Date:** 2026-07-18
- **Validation summary:** `make help` lists `test` as `Test model, api, and gui; typecheck gui`. Standalone `cd gui && bun install && bun run test` exits 0 (0 test files matched — sibling task 002 had not landed in this worktree at validation time; `--pass-with-no-tests` makes this a valid pass per the task doc's guidance). Full `make test` from the project root ran model (no unit tests — generated code), api (`go test ./...`, all packages ok), and gui (`bun run test` then `bun run typecheck`), exiting 0 end-to-end. `git diff -- Makefile AGENTS.md` is limited to the lines the Requirements describe. `grep -n "typecheck gui$" Makefile AGENTS.md` matches only the two updated `Makefile` lines (header comment and target's `##` description), both reading the new "...and gui; typecheck gui" phrasing, not the old typecheck-only framing.
- **Affected files:** [`Makefile`](../../Makefile), [`AGENTS.md`](../../AGENTS.md)
- **Assumptions:** none beyond what the task doc states (no `## Assumptions` section was present).
