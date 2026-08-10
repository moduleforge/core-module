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

## Status

Implementation outcome: **succeeded**. Date: 2026-08-10.

- Updated `AGENTS.md`'s "Key types and packages" table's `fieldcrypto/` row
  to describe `NewFromEnvOrGenerate`'s env-var-always-wins,
  auto-generate-and-persist-on-first-boot, and fail-loudly-on-corrupt-key
  behavior, cross-referencing the new migration.
- Updated `AGENTS.md`'s "Model packages" table's `model/migrations/` row to
  list `0017_field_crypto_keys.sql` alongside the existing `0014`-`0016`
  entries, describing the table's purpose and cross-referencing the
  `fieldcrypto/` row.
- Verified the described behavior against the actually-shipped Phase 1
  source rather than the plan's stated intent: read
  `api/internal/fieldcrypto/fieldcrypto.go` (post-002,
  `NewFromEnvOrGenerate`/`fromPersistedOrGenerated`/`fromHexKey`),
  `api/fieldcrypto/fieldcrypto.go` (façade re-export), and
  `model/migrations/0017_field_crypto_keys.sql` (the `id = 1` and
  `octet_length(key_bytes) = 32` CHECK constraints). All three confirm the
  env-var-always-wins, auto-generate-and-persist-with-DB-level-race-safety,
  and fail-loudly-on-corrupt-length behavior described in the AGENTS.md
  edits above — this matches what task `002-implement-auto-generate-cipher.md`
  implemented (its own Status records `validation failed` on an unrelated,
  pre-existing `go vet` defect in five test-fake `Querier`s, not on anything
  affecting this doc's claims; all three Requirements it specifies were
  implemented and read as such in the source).
- Confirmed `moduleforge.module.yaml`'s `cipher` service entry already
  documents migration `0017` and `NewFromEnvOrGenerate` in its own inline
  comment (from task `002`) — consistent with, and not contradicted by, the
  `AGENTS.md` edits made here.
- Checked `README.md`, `next-steps.md`, and `stories-next.md` for claims
  about `fieldcrypto`'s boot-failure behavior: none exist. `README.md`
  describes the field cipher only generically ("AES-256-GCM field cipher");
  `next-steps.md`'s "Tax-id encryption — future hardening" section discusses
  key rotation and AAD binding only, not boot/env-var behavior. No edits
  needed to any of the three.
- Confirmed via `git submodule status` and `find` that no
  `docs/architecture.md` or `docs/*-spec.md` exists in this project's own
  `project_root` outside the `docs/mf-standards` submodule, matching this
  task doc's own claim; `docs/mf-standards/` was not touched.
- Validation:
  - `grep -n "field_crypto_keys\|NewFromEnvOrGenerate" AGENTS.md` — passed,
    one match in each of the two updated rows (lines 94 and 109 post-edit).
  - `git status` / `git diff --stat` — passed, shows only `AGENTS.md`
    modified; no changes under `docs/mf-standards/`.
  - Final read-through of the updated `AGENTS.md` sections — passed, confirmed
    against the real Phase 1 source per above, not just the plan's stated
    intent.
- **Flagged for the manager** (not acted on by this task, per its own
  Requirements/scope):
  - The `docs/mf-standards` submodule's own `architecture.md` /
    `manifest-spec.md` likely warrant an equivalent update describing the
    new `field_crypto_keys` table / `NewFromEnvOrGenerate` convention, per
    `plan/overview.md`'s "Deferred and flagged" section — this needs its
    own dispatch against that separate repository, as the plan already
    anticipates. This task did not judge the submodule content as newly
    stale beyond what the plan already flagged; it defers to the plan's
    existing note rather than re-flagging a fresh judgment.
  - Task `002-implement-auto-generate-cipher.md`'s own recorded
    `flagged_for_manager` item (five test-fake `Querier` implementations
    across `service/`, `display/`, `entity/`, `types/`, and `httpapi/`
    missing the two new interface methods, breaking `go vet ./...`
    module-wide) remains open; this doc-update task did not touch source
    code and could not have resolved it. Repeating it here only because
    this task's own Status/verification step is what surfaced the
    cross-task Phase 1 detail — the manager should not treat this as a new
    finding.
