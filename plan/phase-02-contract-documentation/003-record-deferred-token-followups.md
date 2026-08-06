# Record Deferred Token Followups

## Purpose and scope

Record three deliberately-deferred items in `plan/followups.yaml` so they survive this plan's
teardown. This is a **documentation task only** — no design work, no implementation, no exploration
of any of the three.

The three items are all things this plan consciously chose not to do. They are recorded now because
`plan/overview.md`, this plan's task documents, and the plan worktree itself are all torn down when
the plan completes; `plan/followups.yaml` is the durable home.

This task is fully independent of the other Phase 2 tasks and of Phase 1's emitted output — it can
run at any point after Phase 1 lands, in parallel with everything else in Phase 2.

Use the flow-mcp `followups_add` tool. Do **not** hand-edit `plan/followups.yaml`; the tool commits
the file and generates the `id`.

## Requirements

Add exactly three items. Every item must carry a `type` tag (mandatory at creation), leave `priority`
and `effort` at `unspecified` (the manager's triage pass sets those), and carry the provenance pair
`plan/slug:gui-spacing-tokens` and `plan/phase:contract-documentation`.

Per the followups schema, `text` **must reference documentation and code — file paths, symbols,
behavior — and never plan, phase, or task names**, because the referenced plan is gone by the time
anyone reads the item. Write each `text` so it stands alone to someone with no memory of this plan.

### 1. Per-component-type radius overrides

- **title**: `Per-component radius override levers` (≤45 chars)
- **type**: `enhancement`
- **text** must convey: `gui/tokens/CONTRACT.md` documents `--mf-radius` as the *single* settable
  radius lever, with `--radius-sm`/`-md`/`-lg`/`-xl` derived from it via
  `calc(var(--mf-radius, var(--mf-radius-default)) * k)` and explicitly **not** individually
  settable. A brand that wants, say, pill-shaped buttons but square-cornered cards cannot express
  that — every primitive rescales together. The component-override tier
  (`gui/tokens/component/overrides.json`, `mf.component.<component>.<property>`) already exists as
  the convention for exactly this kind of escape hatch and is intentionally empty; it is the natural
  home. Note that the per-breakpoint-band lever family added for `--mf-content-margins-*` is a
  worked precedent for layering a second override axis onto a derived-`calc()` token, and could be
  adapted — the axis there is breakpoint, here it would be component. Not designed, not scoped.

### 2. Downstream adoption of the spacing tokens

- **title**: `Adopt spacing tokens in downstream GUIs` (≤45 chars)
- **type**: `enhancement`
- **text** must convey: `app-mftodo/gui/src/tasks/TaskListContainer.tsx`,
  `TaskDetailContainer.tsx`, and `TaskEditorContainer.tsx` hardcode `mx-auto max-w-2xl p-6` across
  eight call sites, and `mod-core/gui/src/ProfileEditor.tsx` hardcodes `p-6 max-w-xl`. These should
  consume `@moduleforge/core-gui`'s `container` utility (or `max-w-content`) via the
  `@moduleforge/core-gui/tokens.css` export, so page width and gutters become brand-overridable
  instead of per-component literals. This mirrors the existing adoption-gap follow-on already
  recorded for `mod-tags`, `mod-tasks`, `mod-contacts`, `mod-users`, and `app-mfdemo`. Note the
  concrete blocker an implementer will hit immediately: `--mf-max-content-width` defaults to `80rem`
  while these pages currently sit at `max-w-2xl` (`42rem`), so adoption is not a mechanical
  substitution — it needs a per-app `--mf-max-content-width` override or a narrower wrapper, and is a
  visible layout change either way.

### 3. Single max-content-width lever is not always enough

- **title**: `Narrow-measure content width token` (≤45 chars)
- **type**: `enhancement`
- **text** must convey: `--mf-max-content-width` is a single global scalar, so an application whose
  shell is wide (80rem) but whose reading-oriented pages want a narrow measure (~42rem) cannot
  express both through the token contract. The obvious extensions are a second role
  (`--mf-max-content-width-narrow`) or a `container-narrow` utility emitted alongside `container` by
  `gui/style-dictionary/build-tokens.mjs`. `gui/tokens/CONTRACT.md` records this as an accepted open
  limitation; this item is the tracked follow-up for closing it. Deliberately not designed here — the
  right shape depends on what downstream adoption actually needs.

### Consistency requirement

Item 3 must stay consistent with the "known limitation" paragraph Phase 2 task `001` adds to
`gui/tokens/CONTRACT.md`. Read that paragraph (if that task has landed) and make sure the two do not
contradict each other. If task `001` has not landed yet, that is fine — write item 3 from the
description above, which is what task `001` is working from too.

### Do not

- Do not hand-edit `plan/followups.yaml`; use `followups_add`.
- Do not add items to `plan/next-steps.yaml` — it is a `version: 1` file with a different schema,
  holding one stale item; `followups.yaml` (`version: 2`) is the live convention in this repo.
- Do not remove or modify any existing follow-up item.
- Do not set `priority` or `effort` to anything but `unspecified`.
- Do not begin designing or implementing any of the three items.
- Do not register a new followups qualifier. All tags used here are already registered
  (`type`, `priority`, `effort`, `plan/slug`, `plan/phase`).

## Validation

1. `followups_list` shows exactly three new items beyond what was present before this task.
2. Each new item carries a `type` tag, `priority:unspecified`, `effort:unspecified`,
   `plan/slug:gui-spacing-tokens`, and `plan/phase:contract-documentation`.
3. Each `title` is 45 characters or fewer.
4. **No `text` field mentions a plan, phase, or task name, or a task-document path.** Re-read all
   three; this is the schema rule most often violated. They must reference only file paths, symbols,
   and behavior.
5. Every file path named in the three `text` fields actually exists — verify each by `ls`, including
   the `app-mftodo` paths (that repository is a sibling checkout under the same playground root;
   verify by path, do not modify anything in it).
6. Every existing item in `plan/followups.yaml` is untouched; the diff is purely additive.
7. `plan/followups.yaml` still parses as valid YAML and still declares `version: 2`.

## Assumptions

- The flow-mcp `followups_add` tool is available and writes to the plan worktree's
  `plan/followups.yaml`, committing as it goes.
- `app-mftodo` is checked out as a sibling of `mod-core` under the playground root, so its paths can
  be verified. If it is absent, keep the paths in the `text` (they are the durable reference) and
  note in the task report that they could not be verified.

## References

- [`plan/notes/token-shape-decision.md`](../notes/token-shape-decision.md), "Known limitation — one
  width lever is not always enough" — the source for item 3.
- `gui/tokens/CONTRACT.md`'s `### Radius` section — the "single settable radius lever" rule item 1
  proposes relaxing.
- `gui/tokens/README.md`'s component-override tier description — the existing, intentionally-empty
  convention item 1 would populate.
- `docs/mf-standards/architecture/gui-design-tokens.md`, "The problem this replaces" — the existing
  adoption-gap follow-on item 2 mirrors. **Read-only**; this file is submodule-mounted and owned by
  another project.
- Project Plan Document Standards, `plan/followups.yaml` — the schema, the tag grammar, and the rule
  that `text` must reference docs and code rather than plan/phase/task names.
