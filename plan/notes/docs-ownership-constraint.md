# Documentation ownership constraint

## Purpose and scope

Records why the plan's documentation deliverable lands in mod-core-owned files rather than directly
in `docs/mf-standards/architecture/entity-typing.md`, and what the task is expected to do about the
entity-typing amendment instead.

## The constraint

`docs/mf-standards/` is a **git submodule** of a separate repository:

```
[submodule "docs/mf-standards"]
	path = docs/mf-standards
	url = git@github.com:moduleforge/docs-mf-standards.git
```

Every file under it — including `architecture/entity-typing.md`, whose "Display-rendering pattern"
section documents the mechanism this plan exposes — belongs to `docs-mf-standards`, not to mod-core.
A task agent working in a mod-core worktree cannot commit there as part of a mod-core plan.

## Established precedent in this repository

- `plan/plan-summary-masked-lookup-403.md` scoped `docs/mf-standards/` as "submodule, reference-only"
  and explicitly out of scope.
- `plan/plan-summary-require-authenticated.md` carries followup **`qxLX`** — "Cross-repo doc
  amendment awaiting explicit manager/user authorization before being applied to the
  docs-mf-standards repository (architecture/authorization-design.md) — NOT committed anywhere by
  this task" — with the drafted amendment text carried in the task's structured report.

That is the exact shape this plan follows.

## What the plan does instead

1. **A mod-core-owned document** carries the HTTP surface and the composing-app / downstream-module
   wiring guidance. `docs/architecture/` exists in mod-core and is currently empty (only a
   `CLAUDE.md`), so it is the natural home; the new doc is linked from `README.md` and `AGENTS.md`
   so it is reachable.
2. **`api/openapi.fragment.yaml`** — mod-core-owned — gains the new path entry. This is the
   authoritative HTTP-surface description consumers `$ref` into their own specs.
3. **`AGENTS.md`** — mod-core-owned — updates the `display/` and `httpapi/` package rows and the
   `moduleforge.module.yaml` narrative to mention the new service entries and route.
4. **The `entity-typing.md` amendment is drafted, not committed.** The task returns the verbatim
   proposed text (a short addition to the "Display-rendering pattern" section noting the HTTP
   exposure and pointing at mod-core's own doc for wiring) in its structured report, and records a
   followup in the same shape as `qxLX` so the manager can route it to the docs-mf-standards repo
   with explicit authorization.

No task in this plan may edit, stage, or commit anything under `docs/mf-standards/`.
