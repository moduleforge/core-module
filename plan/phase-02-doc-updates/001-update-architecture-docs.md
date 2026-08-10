# Update Architecture Docs

## Purpose and scope

Update this repo's own architecture-relevant documentation to reflect the
new `field_crypto_keys` table and the new `fieldcrypto.NewFromEnvOrGenerate`
public API surface, per the architectural-implications check triggered by
Phase 1 (`plan/phase-01-durable-field-key/`): this plan adds significant new
tracked state (a new table) and a new public API surface. Follow the
`update-architecture-docs` task-procedure at
`plugins/flow/task-procedures/update-architecture-docs/SKILL.md` in the Flow
plugin install (a separate repo from this project; not a path inside this
project's `project_root`, hence referenced by plugin-relative path rather
than a repo-relative Markdown link).

`role_doc: plugins/flow/roles/architect-data.md` — the implications here are
primarily a new data-model/schema addition (`field_crypto_keys`), with the
new Go API surface as a secondary, tightly-coupled implication of that same
schema change.

## Requirements

- **Which planned implementation task documents surfaced the architectural
  implications** (both flagged `architectural_impact: true`):
  - `plan/phase-01-durable-field-key/001-add-field-crypto-keys-table.md` —
    new `field_crypto_keys` table (significant new tracked state).
  - `plan/phase-01-durable-field-key/002-implement-auto-generate-cipher.md`
    — new public API (`fieldcrypto.NewFromEnvOrGenerate`,
    `FieldKeyQuerier`) and a `moduleforge.module.yaml` service-declaration
    change affecting every composing app's generated composition root.
- **Which architecture and spec files need review:**
  - `AGENTS.md` (this project's own, in `project_root` — not the `docs/mf-standards`
    submodule) — specifically its "Key types and packages" table (the
    `fieldcrypto/` row's description) and "model/migrations" row (the list
    of migration files, currently ending at `0016_apps_updated_at.sql`).
    Update both to mention the new `0017_field_crypto_keys.sql` migration
    and the auto-generate-and-persist capability.
  - **No `docs/architecture.md` or `docs/*-spec.md` exists in this project's
    own `project_root`** — `mod-core`'s shared architecture documentation
    lives in `docs/mf-standards/architecture.md` and
    `docs/mf-standards/manifest-spec.md`, both inside the `docs/mf-standards`
    git submodule (a separate repository, `docs-mf-standards.git`). Per the
    plan's `plan/overview.md` "Deferred and flagged" section, editing that
    submodule's content is out of this project's `project_root` scope and
    needs its own dispatch against that repo directly — **do not edit any
    file under `docs/mf-standards/` from this task.** If, on review, you
    judge the submodule content genuinely needs updating, record that as a
    flagged item in this task's report rather than editing it.
- Confirm no other in-repo doc (e.g. `README.md`, `next-steps.md`,
  `stories-next.md`) makes claims about `fieldcrypto`'s boot-failure
  behavior that this change contradicts; update any such claim found.

## Validation

- `AGENTS.md`'s "Key types and packages" table's `fieldcrypto/` row and its
  "model/migrations" table row both mention the new capability/migration.
- `grep -n "field_crypto_keys\|NewFromEnvOrGenerate" AGENTS.md` returns at
  least one match in each of the two updated rows.
- `git status` / `git diff --stat` shows no changes under `docs/mf-standards/`
  (the submodule) from this task.
- A final read-through of `AGENTS.md`'s updated sections confirms they
  accurately describe the shipped behavior (env-var-always-wins,
  auto-generate-and-persist fallback, fail-loudly corruption handling) as
  implemented by Phase 1's tasks, not merely as planned.
