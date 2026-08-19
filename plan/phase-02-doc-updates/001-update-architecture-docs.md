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
  (`DisplayService`) and the registry constructor; first production call site of
  `authz.RequireAuthenticated`, an authentication-only authorization rule distinct from every other
  service method in the module.
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

- `AGENTS.md` — the `display/`, `service/`, and `httpapi/` package rows, the `mountFromModule`
  narrative, and the Conventions section (does the authentication-only rule warrant a convention-level
  note alongside "Authorization is checked first in every service method"? It is an exception to it,
  and should be visible there or explicitly cross-referenced).
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
