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

## Status

- **Outcome:** succeeded
- **Date:** 2026-07-15
- **Docs reviewed:** `AGENTS.md` and `README.md` (the only repo-owned architecture-relevant docs;
  confirmed at plan time there is no repo-owned `docs/architecture.md` or `docs/*-spec.md`).
- **Docs updated:**
  - `AGENTS.md` — "Key types and packages" table: added an `apiresp/` row (sentinel set, `WriteJSON`,
    `WriteError`, `InvalidInput`, nested envelope, 5xx `slog` logging with no raw-text leak); updated
    the `service/` row to note its sentinels (`ErrNotFound`, `ErrForbidden`, `ErrInvalidInput`,
    `ErrConflict`) alias the canonical `apiresp` sentinels; updated the `httpapi/` row to note
    success/error responses are written via `apiresp.WriteJSON`/`apiresp.WriteError`. Added a new
    "### gui/ error and toast toolkit" subsection (after the Model packages table, before
    "## mountFromModule special case") describing the client foundation (`api-client.ts` wire types,
    `ApiRequestError`, `request()`, `configureApiClient`), `<FieldError>`/`<ErrorBanner>`,
    `ToastProvider`/`useToast`, and `useApiError` — matching the existing table/prose style (no new
    top-level `##` sections added; new content nests under existing `##` headings as a `###`
    subsection, consistent with the doc's existing heading hierarchy).
  - `README.md` — "What it provides" list: extended the Go API library bullet to mention the shared
    `apiresp` response/error contract, and the GUI component library bullet to mention the error/toast
    widget toolkit; kept both additions to one clause each per the task doc's "keep it brief" guidance.
- **Docs intentionally left untouched:** `docs/mf-standards/architecture/api-response-design.md` and
  all other files under `docs/mf-standards/` — submodule-owned (the `docs-mf-standards` repo), out of
  scope per this plan. Verified `git status --short docs/mf-standards` shows no changes and the
  submodule pointer is unmodified (`git diff --stat` touches only `AGENTS.md` and `README.md`).
- **Validation:**
  - `AGENTS.md` "Key types and packages" table includes the `apiresp/` row; `service`/`httpapi` rows
    reflect the re-homing; the gui toolkit is described — confirmed by re-reading the edited file.
  - `README.md` reflects both new subsystems concisely — confirmed by re-reading the edited file.
  - `git status --short docs/mf-standards` — empty (no submodule changes).
  - Re-read both edited files in full: no dangling cross-references introduced (no new links added;
    all pre-existing links to `docs/mf-standards/manifest-spec.md`, `AGENTS.md`, and
    `.claude/settings.local.json` are untouched and still valid), no contradicted claims within or
    across the two files (the `service/` sentinel-aliasing note in `AGENTS.md` is consistent with the
    `apiresp/` row's description; the README's `apiresp` mention is consistent with the AGENTS.md
    detail).
- **Assumptions applied:** None beyond the task doc's own text — the task doc fully specified which
  table rows/sections to touch and which files were out of scope.
- **Flagged for manager:** None. The GUI toolkit note fit naturally as a new `###` subsection under the
  existing "Key types and packages" `##` heading (no structural rewrite needed); no stale or
  contradicted content was found elsewhere in either file.
- **Files touched:** `AGENTS.md`, `README.md` (in
  `worktrees/phase-03-task-01-update-architecture-docs/`), this task document (Status section).
