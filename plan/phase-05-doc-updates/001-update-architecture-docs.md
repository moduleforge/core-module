# Update Architecture Docs

## Purpose and scope

Update mod-core's own architecture and reference documentation to reflect the key-rotation changes
planned and implemented in this session. Every implementation task below carries
`architectural_impact: true`; by the time this phase runs they have all landed, and the docs that
describe the previous single-key design are stale.

Follow the `update-architecture-docs` task-procedure at
`plugins/flow/task-procedures/update-architecture-docs/SKILL.md`.

role_doc: plugins/flow/roles/architect-backend.md

## Requirements

1. **Implementation task documents that surfaced the architectural implications** (paths relative to
   the plan worktree, all completed by the time this phase runs):
   - `plan/phase-01-key-store-model/001-replace-field-crypto-keys-schema.md`
   - `plan/phase-01-key-store-model/003-regenerate-model-db-and-querier-stubs.md`
   - `plan/phase-02-versioned-cipher/001-multi-key-cipher-core.md`
   - `plan/phase-02-versioned-cipher/002-facade-key-store-adapter.md`
   - `plan/phase-03-rotate-on-read/001-rotating-cipher-helper.md`
   - `plan/phase-03-rotate-on-read/002-wire-decrypt-call-sites.md`
   - `plan/phase-04-admin-rotation-api/001-field-crypto-key-handler.md`
   - `plan/phase-04-admin-rotation-api/002-manifest-and-openapi-routes.md`
2. **Architecture and reference files to review and update where needed:**
   - `AGENTS.md` — the primary target. At minimum its `fieldcrypto/` row (which still describes
     `NewFromEnv`, a single key, and "CORE_FIELD_KEY_HEX set explicitly always wins"), its
     `model/migrations/` row (which still calls `field_crypto_keys` "private, single-row"), its
     `httpapi/` row (which must now name `FieldCryptoKeyHandler` alongside `AppsHandler`), and its
     Conventions section if a rotation-related convention now belongs there.
   - `model/README.md` — confirm the migration-reset note added in Phase 1 is still accurate and
     that the layout/prerequisites text needs nothing further.
   - `README.md` — check for any fieldcrypto or key-management claim that has gone stale.
   - `docs/architecture/` — currently holds no architecture document; confirm that is still the case
     and do **not** create one speculatively. If a `docs/*-spec.md` or `docs/architecture.md` has
     appeared since planning, review it too.
3. **Call out the operator-visible behavior changes prominently**, because they are the ones that
   will surprise someone:
   - `CORE_FIELD_KEY_HEX` is now a **first-boot-only bootstrap seed**. On a later boot it must either
     match the active database key exactly or construction fails loudly. This replaces "env always
     wins" and is a deliberate, visible change.
   - The `field_crypto_keys` table is now the single source of truth for **every** key version, so a
     database dump now carries all active and retired key material rather than none.
   - Rotation is **lazy**: a row that is never read is never re-encrypted, so a retired key can only
     be discarded once every stored blob has actually been read, not after any fixed interval.
   - Rotation is triggered exclusively through the new admin HTTP routes, which are
     wildcard-grant-admin-only.
4. **Do not edit anything under `docs/mf-standards/`.** It is a git submodule pointing at the
   separate `docs-mf-standards` repository — cross-repo despite its in-tree path — and its
   `manifest-spec.md` and `architecture/secret-durability-design.md` going stale is already recorded
   as a deferred cross-repo follow-on in the plan overview.
5. **Do not edit any file outside this repository.** The app-mfmanager deploy-doc rewrite, the
   `NewFromEnv` compile break in three composing repositories, and the mod-users `.env.example`
   sample are all deferred cross-repo follow-ons recorded in the plan overview, not work for this
   task.

## Validation

- `AGENTS.md` no longer describes `field_crypto_keys` as single-row, no longer references
  `NewFromEnv`, and no longer states that `CORE_FIELD_KEY_HEX` always wins.
- `grep -rn "NewFromEnv\b" --include='*.md' . | grep -v worktrees | grep -v mf-standards` returns
  nothing outside historical `plan/plan-summary-*.md` records.
- `grep -rn "single-row" AGENTS.md` returns nothing referring to `field_crypto_keys`.
- Every file named in requirement 2 has been read and either updated or explicitly confirmed
  unchanged, with the confirmation stated in the task report.
- `git status` shows no modification under `docs/mf-standards/` and no submodule pointer change.
- Any newly-described route, service, or table name in the docs matches the implementation exactly
  (spot-check `FieldCryptoKeyHandler`, `coreFieldCryptoKeyHandler`, and `/v1/field-crypto-keys`
  against the merged source).

## References

- `plugins/flow/task-procedures/update-architecture-docs/SKILL.md` — the procedure to follow.
- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md) and
  [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md) — the settled designs, including
  the "a database dump now leaks every key" consequence that belongs in the operator-facing prose.
- [`../notes/key-lifecycle-policy.md`](../notes/key-lifecycle-policy.md) — the
  `CORE_FIELD_KEY_HEX` precedence change, which its own text says must be called out prominently in
  `AGENTS.md`.
- [`../overview.md`](../overview.md) — the deferred cross-repo list, which bounds what this task must
  *not* touch.
