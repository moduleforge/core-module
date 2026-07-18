# GUI Test Runner

## Purpose and scope

Resolves follow-up **vr20** (recorded 2026-07-15, tag `phase-02-gui-error-widgets`): `gui/` (`@moduleforge/core-gui`) has no test runner configured — no `vitest`/`jest`/`bun test` dependency and no `test` script — even though the React Developer role doc calls for component tests as a core responsibility, and sibling tasks 002/003 shipped without tests since neither task doc's Validation section required them. This plan adds a test runner and establishes the initial testing pattern for `gui/`. After this plan completes, the manager removes follow-up vr20 from `plan/followups.yaml`; that removal is not part of this plan's own scope.

In scope:
- Choose and add a test runner + supporting libraries appropriate to `gui/`'s existing React 19 + Vite 5 + Bun stack.
- Add the runtime/type config the chosen runner needs (DOM environment, matcher-type augmentation).
- Add a `test` script to `gui/package.json`.
- Write a small number of initial tests that establish the pattern (not exhaustive coverage).
- Wire the new gui test command into the root `make test` target, and update `AGENTS.md`'s "Test commands" / "Build commands" sections to match.

Out of scope (explicitly, per the originating request):
- Exhaustive test coverage of every existing `gui/` component.
- CI workflow changes — no CI config exists in this repo today; adding one is not part of this plan.
- Standardizing a test runner across sibling ModuleForge modules' `gui/` packages — none of them have one either, and there is no existing local precedent to copy from or align with. This plan establishes the pattern for `mod-core`'s own `gui/` only.

Hard constraints:
- Must not break `make build` / `bun run build` (tsup) or `bun run typecheck` (`tsc --noEmit`).
- Must not introduce dependencies incompatible with the `react`/`react-dom` `^19` peer ranges already pinned in `gui/package.json`.

## Current status

No active plan existed prior to this session. This plan starts at phase 1 with no preconditions beyond the current committed state of `mod-core` (gui/ has no test runner, root `Makefile`'s `test` target typechecks gui rather than testing it, `AGENTS.md`'s Test/Build commands sections describe gui as typecheck-only).

## Overview

Single phase: **`gui-test-infrastructure`**. The change is scoped to one coherent area (`gui/`'s tooling and its immediate consumers — the root `Makefile` and `AGENTS.md`), all three tasks can be fully specified from information already in hand (a hands-on feasibility spike was performed during planning — see [test runner choice](./notes/test-runner-choice.md) — so no further research or user decision is needed), and the task breakdown below is complete and sequential/parallel-annotated.

**Decision (see [test runner choice](./notes/test-runner-choice.md) for the full comparison and feasibility spike):** `bun test`, with `@happy-dom/global-registrator` supplying the DOM environment and `@testing-library/react` + `@testing-library/jest-dom` for component rendering/assertions. This was chosen over Vitest and Jest because `gui/`'s toolchain already standardizes on Bun as package manager and script runner, and `bun test` is a Jest-API-compatible runner already present in the `bun` binary this project already requires — it adds no new *test-framework* dependency, only DOM/testing-library support packages. The spike confirmed React 19 rendering, `@testing-library/jest-dom` matcher types under `gui/tsconfig.json`'s existing strict settings (no tsconfig changes needed), and `global.fetch` mocking for the `request()` wrapper all work cleanly with this combination on the Bun 1.3.14 toolchain already installed.

### Tasks

1. **[001-add-test-runner-dependencies-and-config.md](./phase-01-gui-test-infrastructure/001-add-test-runner-dependencies-and-config.md)** — Adds the devDependencies, the two Bun preload scripts (`gui/happydom.ts`, `gui/testing-library-setup.ts`), `gui/bunfig.toml`, the `src/test-support/matchers.d.ts` type augmentation, and the `gui/package.json` `"test"` script. No test files yet — this task's own validation is a zero-test `bun test` run plus an unaffected `bun run typecheck` / `bun run build`. No dependency on the other two tasks; must land first since both depend on it.

2. **[002-add-initial-component-and-lib-tests.md](./phase-01-gui-test-infrastructure/002-add-initial-component-and-lib-tests.md)** — Depends on task 001. Writes a small, pattern-establishing test set: `src/FieldError.test.tsx` and `src/ErrorBanner.test.tsx` (presentational component tests) and `src/lib/api-client.test.ts` (unit tests for the `request()` fetch wrapper covering its success, network-error, and non-2xx-envelope paths). Deliberately not exhaustive — no tests for `ProfileEditor`/`CorporationForm`/`NaturalPersonForm`/`ServiceAccountForm`, `useApiError`, or `ToastProvider`; those are left as follow-on work per the "establish the pattern, not full coverage" scope.

3. **[003-wire-gui-tests-into-root-test-target.md](./phase-01-gui-test-infrastructure/003-wire-gui-tests-into-root-test-target.md)** — Depends on task 001 (needs the real `bun run test` script to exist); benefits from, but does not hard-depend on, task 002's test files (a zero-test `bun test` run already exits 0 and is a valid, if weaker, validation signal). Updates the root `Makefile`'s `test` target to run `bun run test` in `gui/` instead of `bun run typecheck`, and updates `AGENTS.md`'s "Test commands" and "Build commands" sections to describe the new gui test step and retire the now-inaccurate "gui typecheck only" framing.

### Parallelism

Task 001 is the hard dependency root — both other tasks require its devDependencies/config to exist before their own validation (`bun test`, `bun run test`) can run at all. Tasks 002 and 003 touch disjoint files (002: `src/*.test.ts(x)`; 003: root `Makefile` and `AGENTS.md`) and can run in parallel once 001 has landed, though running 003 after 002 gives 003's `make test` validation step real (non-zero) test output to confirm against.
