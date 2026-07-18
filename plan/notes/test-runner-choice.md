# Test runner choice for gui/

## Purpose and scope

Research backing the architectural decision made in `plan/overview.md`: which test runner and supporting libraries `gui/` (`@moduleforge/core-gui`) should adopt. Documents the options considered, the feasibility spike performed, and the concrete integration recipe the resulting task documents implement.

## Options considered

| Option | Verdict | Why |
|---|---|---|
| `bun test` + happy-dom + Testing Library | **Chosen** | `gui/`'s package manager and script runner is already Bun (`bun.lock` present, all scripts invoked via `bun run`); `bun test` is Jest-compatible (`describe`/`test`/`expect`/`mock`), ships in the Bun binary already required by this toolchain, and adds zero new *test-framework* dependency — only DOM/testing-library support packages. Confirmed feasible by a hands-on spike (see below): renders React 19 components, supports `@testing-library/jest-dom` matchers under TypeScript's strict mode, and mocks `global.fetch` cleanly for the `request()` wrapper. |
| Vitest | Rejected (not chosen) | Would work equally well technically (native Vite integration, and `gui/` already has a `vite.config.ts` for the Tailwind plugin/Ladle), but introduces a second test-runner dependency graph (`vitest`, its own CLI/config surface) on top of a project that already standardizes its JS/TS toolchain on Bun as both package manager and (per this decision) test runner. No functional advantage over `bun test` was found for this package's current, modest testing needs. |
| Jest | Rejected (not chosen) | Foreign to this stack: needs a Babel or ts-jest transform pipeline that duplicates work Vite/tsup/Bun already do, and the project has no other Jest usage to amortize the setup cost against. |

**Decision: `bun test` + `happy-dom` (via `@happy-dom/global-registrator`) + `@testing-library/react` + `@testing-library/jest-dom`.**

## Feasibility spike

Performed in an isolated scratch directory (not committed to any branch) against the actual dependency versions this plan pins, using the installed toolchain's Bun 1.3.14:

- Installed `react@19`, `react-dom@19`, `@testing-library/react@16.3.2`, `@testing-library/dom` (auto-installed as `@testing-library/react`'s peer — its `peerDependencies` list `@testing-library/dom: ^10.0.0`, `react`/`react-dom`: `^18 || ^19`, confirming React 19 compatibility), `@testing-library/jest-dom@6.9.1`, `@happy-dom/global-registrator@20.11.0` (which itself depends on `happy-dom@^20.11.0` — no separate top-level `happy-dom` devDependency is needed), and `@types/bun@1.3.14`.
- Registered a DOM via `@happy-dom/global-registrator`'s `GlobalRegistrator.register()` in a Bun `preload` script, per [Bun's own Testing Library guide](https://bun.com/docs/guides/test/testing-library) (mirrored locally as `node_modules/bun-types/docs/guides/test/testing-library.mdx` in the spike's installed `bun-types`).
- Rendered a component with `@testing-library/react`'s `render`/`screen`, asserted with `@testing-library/jest-dom` matchers (`toHaveTextContent`, `toBeEmptyDOMElement`) extended onto `bun:test`'s `expect` via `expect.extend(matchers)`.
- Mocked `global.fetch` with `bun:test`'s `mock()` to exercise a `request()`-shaped success path and a non-2xx JSON-envelope path.
- Ran `bunx tsc --noEmit` against the spike's own tsconfig (deliberately mirroring `gui/tsconfig.json`'s exact `compilerOptions` — `target: ES2022`, `module: ESNext`, `moduleResolution: bundler`, `jsx: react-jsx`, `strict: true`, no explicit `"types"` array) to confirm the real `gui/tsconfig.json` needs **no changes** — TypeScript's default `types` auto-inclusion (no `"types"` array present) already picks up `@types/bun`'s ambient globals once it's a devDependency; no `include` broadening is needed either, provided the test/setup files live under `src/` (see below).

All spike checks passed (4/4 tests, 0 `tsc` errors) after two corrections, both worth recording so implementers don't rediscover them the hard way:

1. **Preload file split is required, not stylistic.** A single combined preload file that both imports `@testing-library/react` and calls `GlobalRegistrator.register()` fails at runtime (`"For queries bound to document.body a global document has to be available"`) — ESM import hoisting means `@testing-library/react`'s module-level code (which touches `document`) always evaluates before any of that same file's own top-level statements, including the registrator call placed textually above it. Splitting into two preload files — `happydom.ts` (registrator only) then `testing-library-setup.ts` (`expect.extend` + `afterEach(cleanup)`) — makes Bun evaluate the first file's module (registrator call included) to completion before the second file's imports even begin. This is exactly the two-file structure Bun's own docs use, and is required, not cosmetic.
2. **The `bun:test` `Matchers` interface-merge type parameter must match exactly.** `bun-types`' own declaration is `interface Matchers<T = unknown> extends MatchersBuiltin<T> {}`. A hand-written augmentation using a *different* default (e.g. `Matchers<T = any>`) throws `TS2428: All declarations of 'Matchers' must have identical type parameters` in some file arrangements. Declaring the augmentation with **no default at all** — `interface Matchers<T> extends TestingLibraryMatchers<...>` — is what Bun's official guide uses and what the spike confirmed type-checks cleanly; this is the pattern task 001 implements verbatim.

## Confirmed integration recipe (implemented by task 001)

- `gui/happydom.ts` — registrator-only preload file.
- `gui/testing-library-setup.ts` — `expect.extend(matchers)` + `afterEach(cleanup)` preload file.
- `gui/bunfig.toml` — `[test]` / `preload = ["./happydom.ts", "./testing-library-setup.ts"]`.
- `gui/src/test-support/matchers.d.ts` — the `declare module 'bun:test'` `Matchers<T>` augmentation (placed under `src/` so it's automatically covered by `gui/tsconfig.json`'s existing `"include": ["src"]` — no tsconfig edit needed).
- `gui/package.json` — new devDependencies (`@happy-dom/global-registrator`, `@testing-library/react`, `@testing-library/dom`, `@testing-library/jest-dom`, `@types/bun`) and a `"test": "bun test"` script.

Test files themselves (task 002) are colocated next to the source they test as `*.test.ts(x)` — the same colocation convention `gui/` already uses for `*.stories.tsx` — so no separate `test/` or `__tests__/` directory convention needs to be introduced, and `tsup`'s `dts`/bundle output (which only follows the import graph from `src/index.ts`) never picks them up, exactly as it already ignores the existing `*.stories.tsx` files.

## Why `gui/happydom.ts` and `gui/testing-library-setup.ts` live at the `gui/` package root rather than under `src/`

`bunfig.toml`'s `[test].preload` paths are resolved relative to `gui/` (where `bunfig.toml` itself lives), and these two files are Bun-runtime-only preload scripts, not part of the published library surface or its type-checked source graph in the way `src/test-support/matchers.d.ts` needs to be (that one has to sit under `src/` specifically so `tsc --noEmit`'s `"include": ["src"]` picks up its ambient module augmentation — see above). Keeping the two preload scripts at the package root mirrors Bun's own docs convention and keeps `src/` free of anything that isn't either shipped library source or a colocated test/story file.
