# Update Architecture Docs

## Purpose and scope

Review and update mod-core's architecture and specification documentation to reflect the changes
planned and landed in this session: a new HTTP endpoint on the `/entities/*` surface, a new
service-layer component, a new dependency field and constructor on the `httpapi` package boundary,
and two new manifest-declared services in the composition graph.

This runs **after** every Phase 1 task has landed, so the docs are reconciled against what actually
shipped rather than against what was planned.

## Requirements

Follow the `update-architecture-docs` task-procedure at
`plugins/flow/task-procedures/update-architecture-docs/SKILL.md`.

`role_doc: plugins/flow/roles/architect-backend.md` — the implications are backend/API/component
boundary changes (a new HTTP route, a new service component, a widened package dependency surface),
with no data-model, cloud-topology, or frontend dimension.

### Implementation task documents that surfaced the architectural implications

All four carry `architectural_impact: true`:

- `plan/phase-01-display-name-endpoint/001-add-display-service.md` — new service component
  (`DisplayService`) and the registry constructor; gates on real `"read"` authorization via
  `az.Authorize`, the same pattern every other service method in the module already uses (no new
  or exceptional authorization rule was introduced).
- `plan/phase-01-display-name-endpoint/002-add-display-name-endpoint.md` — new public HTTP route
  `GET /v1/entities/{uuid}/display-name`; new exported field and constructor on `httpapi.Deps`.
- `plan/phase-01-display-name-endpoint/003-wire-display-registry-in-manifest.md` — two new nodes in
  the mfgen composition graph (`displayRegistry`, the display service) and a changed `coreDeps`
  constructor.
- `plan/phase-01-display-name-endpoint/004-document-display-name-surface.md` — the endpoint contract
  and composing-app wiring documentation, plus the drafted cross-repo amendment.

### Files to review, and the ownership constraint

mod-core has **no** `docs/architecture.md` and **no** `docs/*-spec.md` of its own — its
architecture-level documents are the module manifest, `AGENTS.md`, and the submodule-hosted
mf-standards set. Review each of the following and update only where the shipped change made it
stale or incomplete:

- `AGENTS.md` — the `display/`, `service/`, and `httpapi/` package rows and the `mountFromModule`
  narrative. No Conventions-section update is expected here: the display service follows
  "Authorization is checked first in every service method" exactly like every other service, rather
  than introducing an exception to it.
- `docs/architecture/display-name-http.md` — created in Phase 1 task 004; verify it matches what
  actually shipped (field names, status codes, constructor names, manifest entry names).
- `README.md` — the documentation links.
- `api/openapi.fragment.yaml` — the published HTTP contract.
- `moduleforge.module.yaml` — the composition-graph declaration and its explanatory comments.

**Read-only, never edited:** everything under `docs/mf-standards/` is a git submodule of the separate
`docs-mf-standards` repository. `docs/mf-standards/architecture.md`,
`docs/mf-standards/architecture/entity-typing.md`, and
`docs/mf-standards/architecture/api-response-design.md` are all relevant *sources* to check
conformance against, and `entity-typing.md` has a drafted amendment pending from Phase 1 task 004 —
but no file under that path may be edited, staged, or committed by this task. If this review surfaces
a further needed amendment there, extend the drafted text and record it as a followup, exactly as
task 004 does.

## Validation

- Each file named above has been read and either updated or explicitly confirmed still accurate; the
  task report states which, per file.
- Every statement added or amended matches the landed implementation — verify constructor names,
  manifest entry names, route path, and response field names by grep against the source, not against
  the plan documents.
- `git status --porcelain docs/mf-standards` reports no modification.
- Every document changed remains reachable by link from `README.md`.
- Any drafted-but-unapplied cross-repo amendment text is reproduced verbatim in the structured report
  and carries a followup entry.
- `cd api && make test` and `cd api && make lint` pass (regression guard; no source change expected
  from this task).

## References

- `plugins/flow/task-procedures/update-architecture-docs/SKILL.md` — the procedure to follow.
- `plugins/flow/roles/architect-backend.md` — the role doc for this task.
- The four Phase 1 task documents listed above.
- `plan/notes/docs-ownership-constraint.md` — the submodule rule and the `qxLX` precedent for a
  drafted cross-repo amendment.

## Status

**Outcome:** no_changes_needed — 2026-08-24.

Reviewed all five named files against the landed Phase 1 implementation (`api/service/display.go`,
`api/httpapi/display.go`, `api/httpapi/router.go`) and found every claim already accurate — no
stale sections. This is expected: Phase 1 task 004 wrote the doc/OpenAPI/README/AGENTS.md content
directly against the shipped code (not against the plan) in the same session, and task 003 kept
`AGENTS.md`'s `display/`/`service/`/`httpapi/` rows current when it wired the manifest. Per file:

- `AGENTS.md` — `display/`, `service/`, `httpapi/` rows and the `mountFromModule` narrative all
  confirmed accurate; no Conventions-section change needed (no exception to "authorization checked
  first" was introduced). No edit.
- `docs/architecture/display-name-http.md` — cross-checked field names (`uuid`, `display_name`),
  status codes (200/400/401/403, no 404), constructor names (`NewDisplayRegistry`,
  `NewDisplayService`, `NewDepsWithDisplay`), and manifest entry names (`displayRegistry`,
  `displayService`, `coreDeps`) against `api/service/display.go`, `api/httpapi/display.go`,
  `api/httpapi/router.go`, and `moduleforge.module.yaml` — all match. No edit.
- `README.md` — link to the new architecture doc present and resolves. No edit.
- `api/openapi.fragment.yaml` — `getEntityDisplayName` path entry's schema, examples, and error
  responses match the handler exactly. No edit.
- `moduleforge.module.yaml` — `displayRegistry`/`displayService` entries and the `coreDeps`
  three-argument constructor switch match the actual Go signatures (grep-confirmed against
  `api/service/display.go` and `api/httpapi/router.go`). No edit.

`docs/mf-standards/` review: the submodule is not checked out in this worktree (uninitialized), so
`entity-typing.md` and `api-response-design.md` were read read-only from the main checkout at
`/Users/zane/playground/moduleforge/mod-core/docs/mf-standards/` for conformance cross-checking
only — never written. `entity-typing.md`'s "Display-rendering pattern" section still has no mention
of the HTTP endpoint, confirming task 004's drafted-but-unapplied amendment remains pending and
unapplied, exactly as expected; this review surfaced no *further* amendment need beyond what task
004 already drafted, so no new draft text was produced and no new followup is being raised — the
existing `qxLX`-shaped followup task 004 flagged for the manager still stands unchanged.
`api-response-design.md`'s reserved core `error.code` set was cross-checked against the endpoint's
actual status/error mapping (401/403/400, no new top-level code) — conforms.

Validation:

- All five files read and confirmed accurate (no updates required) — per-file notes above.
- Every statement cross-checked against source (`api/service/display.go`, `api/httpapi/display.go`,
  `api/httpapi/router.go`, `moduleforge.module.yaml`) via grep/read, not against plan documents.
- `git status --porcelain docs/mf-standards` — empty (no modification; submodule not even checked
  out in this worktree).
- No documents were changed, so the link-chain-reachability check is unaffected; `README.md`'s link
  to `docs/architecture/display-name-http.md` was re-confirmed as resolving.
- No new/further cross-repo amendment was drafted (none was needed); the existing task-004 draft and
  its followup remain the only pending item.
- `cd api && make test` — passed (all 13 packages, including `service` and `httpapi`).
- `cd api && make lint` — passed (`go vet ./...`, no findings).

No `## Assumptions` section was present on this task doc, so none were relied on beyond what
`## Requirements` states directly.

Affected files: none (no source or doc edits made; this status note is the only change).
