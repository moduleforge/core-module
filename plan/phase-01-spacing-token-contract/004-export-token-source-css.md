# Export Token Source Css

## Purpose and scope

Publish `@moduleforge/core-gui`'s Tailwind-**source** token CSS as a consumable package export, so a
downstream app's own Tailwind build can see the `@theme inline` mapping and the new
`@utility container` block. Without this, the mechanism task `002` builds is unreachable outside
mod-core's own build.

Files touched: `gui/package.json` (the `build` script, `exports`, and possibly `files`), and
`gui/README.md`'s consumption section.

### Why this is needed, concretely

`@moduleforge/core-gui` today publishes exactly one stylesheet export, `./styles.css` →
`dist/index.css`. That file is **already-compiled** Tailwind output: it contains the `:root`
`--mf-*-default` declarations and the `@property` registrations, but **zero** `@theme` and **zero**
`@utility` at-rules, because Tailwind consumes and erases them during compilation.

The consequence is visible in the wild today. `app-mftodo/gui/src/styles.css` hand-mirrors mod-core's
*entire* `@theme inline` block — ~26 lines — under a comment warning that it must be re-copied
whenever mod-core's drifts. An `@utility container` block has the same problem, only worse: it is not
a mapping a consumer can copy once, it is a utility *definition* that only exists if the consumer's
own Tailwind pass sees the at-rule. And `dist/index.css` will not contain a compiled `.container`
rule to fall back on, because mod-core's own component source never uses the class and Tailwind
purges it.

So this task is a prerequisite for the new tokens being usable at all, and it simultaneously retires
the existing hand-mirroring hazard.

### Scope boundary

This is mod-core's **own package surface** — it is not downstream adoption, which is explicitly out
of scope for this plan. Do not edit `app-mftodo` or any other consumer. Document the intended
consumption pattern in `gui/README.md`; leave the act of adopting it to the follow-on.

## Requirements

1. **Make the source token CSS land in `dist/`.** `gui/package.json`'s `files` field is `["dist"]`
   and `gui/tokens/dist/` is gitignored, so the compiled bundle must be copied into `gui/dist/` as
   part of the build rather than exported from its build location. Extend the `build` script — which
   is currently `bun run build:tokens && tsup && bun run build:css` — with a copy step that places
   `tokens/dist/tokens.css` at `dist/tokens.css`. Prefer a small explicit step (its own
   `build:tokens-css` npm script, or an added command in the chain) over hiding the copy inside
   `build:tokens`, so the build's stages stay legible.

   Order matters: the copy must run after `build:tokens` (which produces the file) and must not be
   clobbered by `build:css` (which writes `dist/index.css`).

2. **Add the export.** In `gui/package.json`'s `exports` map, alongside the existing
   `"./styles.css": "./dist/index.css"`:

   ```json
   "./tokens.css": "./dist/tokens.css"
   ```

   Name it `./tokens.css` — it names the artifact, parallels the existing `./styles.css`, and reads
   correctly at the consumer's call site as `@import "@moduleforge/core-gui/tokens.css";`.

3. **Confirm `files` still covers it.** `["dist"]` already does, given the copy in step 1. Do not add
   `tokens/dist` to `files` — that would publish a gitignored build directory from a second location
   and give two competing paths to the same artifact.

4. **Document the two exports and when to use which** in `gui/README.md`, near the existing
   "Consuming this package from an app" and "Tailwind content glob requirement" sections:

   - `@moduleforge/core-gui/styles.css` — the compiled bundle. Baked `--mf-*-default` values,
     `@property` registrations, and mod-core's own compiled component utilities. What an app imports
     to get mod-core's components rendering.
   - `@moduleforge/core-gui/tokens.css` — the Tailwind **source** bundle. Carries the `@theme inline`
     key mapping and the `@utility container` definition, which only take effect when processed by
     the consumer's *own* Tailwind pass. An app that wants `bg-primary`, `rounded-md`,
     `max-w-content`, or the token-backed `container` utility in its own markup imports this **into
     its Tailwind entry point**, instead of hand-copying the `@theme` block.

   State plainly that hand-mirroring the `@theme inline` block is now unnecessary and discouraged, so
   the next consumer does not repeat `app-mftodo`'s pattern.

5. **Note the overlap honestly.** An app importing both exports gets the `:root` `--mf-*-default`
   declarations and `@property` registrations twice. This is harmless — identical values, last one
   wins, no visual effect — but it is real duplication and should be stated rather than discovered.
   Resolving it (splitting the compiled bundle so the two exports are disjoint) is a follow-on, not
   this task.

### Do not

- Do not modify `app-mftodo` or any other consuming project.
- Do not remove or repoint the existing `./styles.css` export — consumers depend on it.
- Do not commit anything under `gui/tokens/dist/` or `gui/dist/`; both are gitignored.
- Do not change `gui/package.json`'s `version`.

## Validation

1. `cd gui && bun install` (the checkout's `tailwindcss` symlinks are known broken) then
   `cd gui && bun run build` exits 0.
2. `gui/dist/tokens.css` exists after the build and is byte-identical to `gui/tokens/dist/tokens.css`:
   `cmp gui/dist/tokens.css gui/tokens/dist/tokens.css` reports no difference.
3. `gui/dist/tokens.css` contains an `@theme inline` block and an `@utility container` block —
   i.e. it is the *source* bundle, not a compiled one.
4. `gui/dist/index.css` still exists and is unchanged in character (still compiled output, still
   contains the `:root` `--mf-*-default` declarations).
5. `node -e "console.log(require.resolve('@moduleforge/core-gui/tokens.css'))"` resolves from a
   context where the package is linked — or, if no linked consumer is available in this worktree,
   confirm the `exports` map is well-formed JSON and the target path exists relative to
   `gui/package.json`.
6. **End-to-end proof that the export is actually consumable.** Compile a throwaway Tailwind entry
   that imports the built file by path and uses the class:

   ```sh
   cd gui
   printf '@import "tailwindcss";\n@import "./dist/tokens.css";\n' > /tmp/consumer-probe.css
   printf '<div class="container max-w-content bg-primary"></div>' > /tmp/consumer-probe.html
   ./node_modules/.bin/tailwindcss -i /tmp/consumer-probe.css -o /tmp/consumer-probe-out.css --content /tmp/consumer-probe.html
   ```

   Confirm the output contains a token-backed `.container` rule, a `.max-w-content` rule, and a
   `.bg-primary` rule — proving the `@utility`, the new `--container-content` theme key, and the
   pre-existing colour mapping all survive the export path.
7. `cd gui && bun run test` and `cd gui && bun run typecheck` pass.
8. `git status` shows `gui/package.json` and `gui/README.md` modified, and no new tracked files.

## Metadata

architectural_impact: true

## Assumptions

- Task `002` has landed, so `tokens/dist/tokens.css` contains the `@theme inline` and
  `@utility container` blocks this task's validation looks for.
- `bun` is available and `bun install` succeeds in `gui/`.
- No consumer currently imports a path that this change would shadow or break — the addition is
  purely additive to the `exports` map.

## References

- `gui/package.json` — the `build` script, `exports`, and `files` fields being edited.
- `gui/README.md` — the "Consuming this package from an app" and "Tailwind content glob requirement"
  sections the new documentation sits beside.
- `app-mftodo/gui/src/styles.css` — **read-only, as evidence.** Its hand-mirrored `@theme inline`
  block and the drift warning above it are exactly the problem this task removes. Do not edit it.
- [`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md), "Distribution gap this
  design surfaces" — the full statement of the problem.

## Checkpoint hints

- After the build-script copy step produces `gui/dist/tokens.css`.
- After adding the `exports` entry and proving it with validation step 6.
- After the `gui/README.md` documentation.
