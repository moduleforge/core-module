# Update Architecture Docs

## Purpose and scope

Update mod-core's repo-owned documentation to reflect the new subsystems introduced in this session —
the `api/apiresp` shared response/error package and the `mod-core/gui` error/toast widget toolkit — so
the docs stay in sync with the code. Runs after the implementation phases land.

Skill/procedure: `update-architecture-docs` — see
`plugins/flow/task-procedures/update-architecture-docs/SKILL.md`.

role_doc: `plugins/flow/references/roles/architect-backend.md` (default; the primary architectural
implication is the new backend cross-cutting `apiresp` contract package. A substantial frontend
subsystem — the gui error/toast toolkit — is also introduced; cover it in `AGENTS.md` as well.)

## Requirements

The implementation task documents that surfaced these architectural implications (all completed by the
time this phase runs):

- `plan/phase-01-apiresp-go/001-create-apiresp-package.md` — new `api/apiresp` package (new cross-cutting
  contract subsystem; new public API surface; nested error-envelope shape).
- `plan/phase-01-apiresp-go/002-dogfood-migrate-mod-core.md` — mod-core's HTTP surface now emits the
  nested envelope via `apiresp`; `service` sentinels re-homed onto `apiresp`; `ErrConflict` added.
- `plan/phase-02-gui-error-widgets/001-client-foundation.md`,
  `plan/phase-02-gui-error-widgets/002-toast-provider.md`,
  `plan/phase-02-gui-error-widgets/003-error-widgets.md`,
  `plan/phase-02-gui-error-widgets/004-use-api-error-hook.md` — new gui error/toast subsystem and the
  promoted shared `request()` client helper.

Docs to review and update (repo-owned only):

- **`AGENTS.md`** — the "Key types and packages" table currently lists the `api/` packages. Add a row
  for **`apiresp/`** (the shared response/error contract: canonical sentinels, `WriteJSON`,
  `WriteError`, `InvalidInput`, `FieldError`; owner of the nested error envelope and the
  sentinel→status/code mapping). Note that `service/` sentinels now alias `apiresp` and that
  `ErrConflict` exists. For `gui/`, add a brief note (in the relevant section) describing the new error/
  toast toolkit: wire types + `ApiRequestError` + `request()` client helper, `<FieldError>`,
  `<ErrorBanner>`, `ToastProvider`/`useToast`, and `useApiError`. Keep additions consistent with the
  existing table/section style.
- **`README.md`** — the "What it provides" list mentions the Go API library and the GUI component
  library. Add a concise mention of the shared response/error contract (`apiresp`) and the GUI
  error/toast toolkit if it improves accuracy; keep it brief (README is a pitch, not a reference).

Explicitly **out of scope / do not modify**:

- **`docs/mf-standards/architecture/api-response-design.md`** and any other file under
  `docs/mf-standards/` — the canonical architecture docs and the design doc live in the out-of-scope
  `docs-mf-standards` submodule (a different repo). Do not patch the design doc (explicitly excluded by
  this plan's scope). Record in the task report that the canonical architecture/design docs were
  intentionally not touched because they are submodule-owned and out of scope.
- There is **no repo-owned `docs/architecture.md` or `docs/*-spec.md`** in mod-core (verified at plan
  time — only the submodule contains architecture docs), so there is no such in-repo file to update.

## Validation

- `AGENTS.md` "Key types and packages" table includes an `apiresp/` row; the gui error/toast toolkit is
  described; `service` sentinel re-homing + `ErrConflict` are reflected.
- `README.md` accurately reflects the new subsystems (if edited) and remains concise.
- No file under `docs/mf-standards/` is modified: `git status docs/mf-standards` shows no changes and
  the submodule pointer is unchanged.
- `AGENTS.md`/`README.md` render cleanly and internal links remain valid.
- The task report states explicitly which docs were reviewed, which were updated, and that the
  submodule-owned architecture/design docs were intentionally left untouched.

## References

- `plugins/flow/task-procedures/update-architecture-docs/SKILL.md` — the procedure to follow.
- `plugins/flow/references/roles/architect-backend.md` — role doc for this task.
- `AGENTS.md`, `README.md` — the repo-owned docs to update.
- `docs/mf-standards/architecture/api-response-design.md` — the (out-of-scope, submodule-owned)
  contract the new code implements; reference for accuracy, not a modification target.
- The Phase 01 and Phase 02 task documents listed above — the changes to document.
