# Add Test Runner Dependencies And Config

## Purpose and scope

Adds the `bun test` + happy-dom + Testing Library test runner stack to `gui/` (`@moduleforge/core-gui`) and wires the runtime/type config it needs, per the decision recorded in [`plan/notes/test-runner-choice.md`](../notes/test-runner-choice.md) (read that note first — it documents the exact recipe below, why it's shaped this way, and two non-obvious pitfalls a naive implementation hits). This task adds **no test files** — it only makes `bun test` runnable (with zero tests, which is a valid passing state) and confirms the existing `build`/`typecheck` scripts are unaffected. Test files are added by task 002.

This is a config/dependency task, not covered by a standard skill — implement directly per the Requirements below.

## Requirements

Work entirely inside `gui/`. All paths below are relative to `gui/` unless stated otherwise.

1. **Add devDependencies** to `gui/package.json` (alongside the existing `devDependencies` block; do not touch `dependencies` or `peerDependencies`):
   - `@happy-dom/global-registrator` (`^20`) — do **not** also add a separate top-level `happy-dom` devDependency; `@happy-dom/global-registrator` already depends on `happy-dom@^20.11.0` and pulls it in transitively.
   - `@testing-library/react` (`^16`)
   - `@testing-library/dom` (`^10`) — `@testing-library/react`'s peer dependency; add it explicitly even though Bun auto-installs peers, matching Bun's own Testing Library guide.
   - `@testing-library/jest-dom` (`^6`)
   - `@types/bun` (`^1`)

   Use `bun add -D <pkg>@<range>` (run from `gui/`) rather than hand-editing the version strings, so `bun.lock` is regenerated consistently. Verify afterwards that `react`/`react-dom` in `gui/package.json` are untouched and that `@testing-library/react`'s installed `peerDependencies` (check `node_modules/@testing-library/react/package.json`) list `react`/`react-dom` ranges that include `^19` — they do (`^18.0.0 || ^19.0.0`) as of the version pinned above; if a future `bun add` resolves a version whose peer range excludes `^19`, halt and report rather than proceeding.

2. **Add two Bun preload scripts at the `gui/` package root** (not under `src/` — see the note's rationale):

   `gui/happydom.ts`:
   ```ts
   import { GlobalRegistrator } from '@happy-dom/global-registrator';

   GlobalRegistrator.register();
   ```

   `gui/testing-library-setup.ts`:
   ```ts
   import { afterEach, expect } from 'bun:test';
   import { cleanup } from '@testing-library/react';
   import * as matchers from '@testing-library/jest-dom/matchers';

   expect.extend(matchers);

   afterEach(() => {
     cleanup();
   });
   ```

   **These must stay in two separate files, loaded in this order.** A single combined file (registrator call + `@testing-library/react` import in the same module) fails at runtime with `"For queries bound to document.body a global document has to be available"` — ESM import hoisting evaluates `@testing-library/react`'s module-level code before any of that same file's own statements run, including a registrator call that appears earlier in the source. Splitting into two preload files makes Bun fully evaluate the first file (registrator call included) before the second file's imports even begin. See the note's "Feasibility spike" section for the full explanation.

3. **Add `gui/bunfig.toml`:**
   ```toml
   [test]
   preload = ["./happydom.ts", "./testing-library-setup.ts"]
   ```

4. **Add the `bun:test` matcher-type augmentation under `src/`** so it's automatically covered by `gui/tsconfig.json`'s existing `"include": ["src"]` (no tsconfig edit needed — confirmed in the spike). Create `gui/src/test-support/matchers.d.ts`:
   ```ts
   import type { TestingLibraryMatchers } from '@testing-library/jest-dom/matchers';
   import type { expect } from 'bun:test';

   declare module 'bun:test' {
     interface Matchers<T>
       extends TestingLibraryMatchers<ReturnType<typeof expect.stringContaining>, T> {}
   }
   ```
   Use `interface Matchers<T>` with **no default type parameter** — `bun-types`' own declaration is `interface Matchers<T = unknown> extends MatchersBuiltin<T> {}`, and a hand-written augmentation with a different default (e.g. `T = any`) throws `TS2428: All declarations of 'Matchers' must have identical type parameters` under some file arrangements. The no-default form above is what Bun's own docs use and what the feasibility spike confirmed type-checks cleanly against `gui/tsconfig.json`'s exact settings.

   This file has no runtime import/export need beyond the `declare module` augmentation itself — `export {}` is not required since the `import type` statements already make it a module.

5. **Add the `test` script** to `gui/package.json`'s `"scripts"` block: `"test": "bun test"`.

6. **Do not change `gui/tsconfig.json`.** The spike confirmed it needs no edits: TypeScript's default `types` auto-inclusion (no `"types"` array is set today, and none should be added) already picks up `@types/bun`'s ambient globals once it's a devDependency, and `src/test-support/matchers.d.ts` living under `src/` means `"include": ["src"]` already covers it. If, contrary to the spike, you find `bun run typecheck` actually does need a tsconfig change to pass cleanly, that's a live discrepancy from this task doc's expectation — make the minimal necessary change, and flag the discrepancy explicitly in your task report (do not silently diverge from what this document says was verified).

## Validation

Run all of the following from `gui/` (after `bun install` to pick up the new lockfile) and confirm each passes:

- `bun run test` (equivalently `bun test`) — expect a clean `0 fail` run. Zero test files matched is an acceptable, valid outcome for this task (task 002 adds the first real tests); a nonzero fail count, or a runtime error from the preload scripts (e.g. the `document`-not-available error described in Requirement 2), is not.
- `bun run typecheck` — must still pass with no new errors.
- `bun run build` — must still succeed; inspect `dist/` afterwards (`ls dist`) and confirm no `happydom.ts`, `testing-library-setup.ts`, `test-support/`, or `*.test.*` files appear in the output (tsup's `dts`/bundle only follows the import graph from `src/index.ts`, so nothing added by this task should be reachable from it).
- `git status` / `git diff --stat` — confirm the change set is limited to `gui/package.json`, `gui/bun.lock`, `gui/happydom.ts`, `gui/testing-library-setup.ts`, `gui/bunfig.toml`, and `gui/src/test-support/matchers.d.ts`. No other file should be touched by this task.

## Assumptions

- Bun 1.3.x (matching the toolchain already required by this project's `AGENTS.md`) is the Bun version available in the task's execution environment; the spike backing this task doc ran against Bun 1.3.14.
- `bun install` has network access to the npm registry to resolve the new devDependencies (confirmed reachable during planning).

## References

- [`plan/notes/test-runner-choice.md`](../notes/test-runner-choice.md) — full rationale, options comparison, and the feasibility spike this task's exact recipe is drawn from.
- `gui/package.json`, `gui/tsconfig.json`, `gui/tsup.config.ts`, `gui/vite.config.ts` — current config this task must not regress.
- [Bun's Testing Library guide](https://bun.com/docs/guides/test/testing-library) — upstream source for the two-file preload split and the `Matchers<T>` (no default) augmentation pattern.

## Status

- **Outcome:** succeeded
- **Date:** 2026-07-18
- **Validation summary:** `bun run test` (0 fail, 0 test files, exit 0 after fix below), `bun run typecheck` (pass, no tsconfig change needed — Requirement 6 confirmed), `bun run build` (success; `dist/` contains only `index.js`, `index.mjs`, `index.d.ts`/`.mts`, and their maps — no test/preload/support files leaked in). `git diff --stat` confirms the change set is limited to `gui/package.json`, `gui/bun.lock`, `gui/happydom.ts`, `gui/testing-library-setup.ts`, `gui/bunfig.toml`, `gui/src/test-support/matchers.d.ts`.
- **Affected source files:** `gui/package.json`, `gui/bun.lock`, `gui/happydom.ts`, `gui/testing-library-setup.ts`, `gui/bunfig.toml`, `gui/src/test-support/matchers.d.ts`.
- **Assumptions applied:** Bun 1.3.14 was the available toolchain (matches the Assumptions section exactly); `bun install` had registry access.
- **Discrepancy from this doc's spike-verified expectation:** Requirement 5's literal script (`"test": "bun test"`) does **not** produce a clean exit-0 zero-test run against the installed Bun 1.3.14 — `bun test` with zero matching test files exits 1 with `error: 0 test files matching ...`, not the 0-fail/exit-0 outcome this doc's Validation section describes as an acceptable passing state. This appears to be a gap in the spike (the spike's feasibility run always had 4 real test cases; it does not record having exercised the true zero-test-file case). Fixed in-scope by changing the `test` script to `"bun test --pass-with-no-tests"` (Bun's own flag for exactly this scenario, per `bun test --help`); `bun run test` now exits 0 with zero test files, satisfying the Validation section's literal expectation. Note this fix is script-scoped: bare `bun test` (bypassing the `package.json` script) still exits 1 on zero test files in this Bun version, since Bun has no `bunfig.toml`-level equivalent to `--pass-with-no-tests`. This will stop mattering once task 002 adds real test files (nonzero match count no longer hits this path).

## Metadata

architectural_impact: false
