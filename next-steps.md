# core-module — next steps

This file tracks pending manual verification and deferred work for `core-module`. Architecture decisions made across the cross-cutting framework rounds and the SQL access-function refactor are documented in `docs/architecture/` and indexed by `plan/report.phase-1-sql-access-fn-handoff.md` at the user-components root.

## Pending manual verification (needs live stack / DB)

- **`make dev.start` end-to-end smoke** — bring up the full stack, log in, edit profile at `/profile`, reload to confirm persistence; create a user via admin; edit an existing user.
- **`goose status` against a freshly-migrated local DB** — confirm migrations applied in order: core (`0001–0099`, ending at `0099_access_function_stubs.sql`) → audit-module (`0400_audit_log.sql`) → users-module (`0100–0107`) → tags-module (`0200–0299`) → contacts-module (`0300–0399`).
- **Audit log smoke** — after a profile edit, verify `SELECT op, resource, actor_entity_id FROM audit_log ORDER BY at DESC LIMIT 5` shows `op='update'`, `resource='natural_person'`, populated `actor_entity_id`, populated `before`/`after`. (Note: column is `actor_entity_id`, not `actor_user_id` — schema generalized in Phase 3.)
- **Access function inlining spot-check** — for one rewritten list query (e.g. `SearchTags`), `EXPLAIN ANALYZE` against the dev shadow with `setup.ApplyFuncs` applied; confirm the access function inlines (planner pushes predicates through; not a "Function Scan" with materialised rows).
- **Tax-id round-trip** — verify `Cipher.Decrypt` on an empty-but-non-NULL bytea cleanly yields `""` against live Postgres. Inspect raw column (`SELECT encode(ssn, 'hex') FROM natural_persons WHERE entity_id = <id>;`) to confirm the plaintext SSN bytes are NOT present.

## Pre-existing code-level gaps

- `EntityService.GetByID` returns `ErrNotFound` unconditionally — no caller uses it today; sqlc query was never added. Add when needed.
- `ServiceAccountService.UpdateByEntityUUID` returns `ErrInvalidInput` because no `UpdateServiceAccount` sqlc query exists. Wiring is in place; the sentinel is misleading (should be `ErrNotImplemented` → 501) until the query lands.
- Entity forms (`<NaturalPersonForm>`, `<CorporationForm>`, `<ServiceAccountForm>`) exist in core-gui but the admin user-edit page in users-module renders its own inline form. Consolidation deferred ("leave admin pages alone — we can consolidate later").
- `getCorporation` has no admin-or-self enforcement gate (pre-existing, not a regression). The tax_id response gate still correctly withholds `ein` for non-admin non-subject callers, but the general GET still leaks other fields.

## Tax-id encryption — future hardening

- **Key rotation.** No mechanism for rotating the AES-256-GCM key. A future hardening step would embed a key-id prefix in the blob and introduce a re-encryption migration.
- **AAD binding.** Ciphertext is not bound to the row's primary key, so a ciphertext could in principle be copied between rows. A source comment documents this as future work.

## Cross-cutting framework — observation/observer cosmetic

- `ObserverGroup.ObserveAfterCommit` returns `error` but always returns nil and every caller discards it. Consider dropping the return type. Lint warnings tolerable today.
- `ObserveAfterCommit` `before` parameter is now mandated nil by convention (doc-enforced, not type-enforced). Consider dropping the parameter entirely from the interface signature.
- `_ = eg.Wait()` discard pattern in `observer.go` swallow / post-commit branches is correct today (closures always return nil) but a future maintainer adding a `return err` inside would not get a compile warning. Consider switching those branches to `sync.WaitGroup`.
- `txhelper.QueuePostCommit` is implemented but unused in production service code. Either adopt in a service that needs in-closure post-commit queueing, or remove.
- `fake_tx_test.go` in `api/httpapi/` is vestigial dead code retained because a wrapper in `mock_test.go.buildTestDepsWithTx` still references it. Pure cleanup pass.

## Phase 1 (SQL access-fn) — Phase 1 followups

These surfaced during the round and are not blocking; capture before context-clear:

- **`AdminOrOwnGenerator` + slug-list maintenance.** `users-module/api/cmd/server/main.go` hardcodes the slug list passed to `setup.ApplyFuncs`; `core-module/api/authz/setup/admin_or_own.go.GenerateForResource` hardcodes per-slug bodies. Adding a new entity type requires updating both. Acceptable today (5 slugs); an easy mistake to make. A small registration helper could reduce the surface — defer until a real new-entity-type scenario.
- **`EntityResolver.AllowNotFound` is unused.** No resource has opted into 404 transparency. Reasonable default for a privacy-conservative system; flag if a UI starts relying on 404 to distinguish missing vs. forbidden.
- **`Pagination` is module-defined.** Each peer module redefines a tiny `Pagination` struct with a `normalize()` method (limit cap 200, default 50). Conventions are consistent; consolidate to a single `core-module/api/service.Pagination` once the shape has stabilised across enough call sites.

## Phase 2: `authz-module` + `grants` table

When richer authz than admin-or-own is needed (e.g. group membership, explicit per-entity grants), the architectural direction (recorded in `docs/architecture/authorization-design.md` "Future evolution") is to add a dedicated `authz-module` with a `grants(actor_entity_id, target_entity_id, operation, ...)` table. A `GrantTableGenerator` replaces `AdminOrOwnGenerator` at the composition root; function bodies query `grants` directly. **Peer-module query files do not change** — they still JOIN `accessible_<resource>_ids_for_actor`. Proposed migration range: `0500-0599`.

Concrete work for Phase 2:
1. Define `authz-module/{model,api}` mirroring `audit-module` shape.
2. `0500_grants.sql` migration with `grants` table.
3. `GrantTableGenerator` in `core-module/api/authz/setup/grant_table.go` (or in authz-module if cleaner).
4. `make compose` updates in each peer module to include authz-module migrations.
5. App composition root swaps generators.

Not blocked on anything; defer until a concrete grant-driven use case appears.

## Phase 3: trigger-maintained access tables

If indirect-grant fan-out (e.g. group-of-users grants on org-of-entities) creates measurable contention or perf issues at scale, per-resource `*_access` tables (e.g. `tag_access(actor_entity_id, tag_entity_id, can_read)`) materialise the grants for fast index-scan JOIN. Triggers on `grants` keep access tables in sync. The function bodies become simple `SELECT entity_id FROM <resource>_access WHERE actor_entity_id = $1 AND can_read` queries — still inlinable.

This is a Phase 3 evolution path; the Phase 2 generator is sufficient for the foreseeable simple-grant case.

## Mechanical CRUD (architect Proposal 6)

The architect's evaluation across rounds 4–5 suggested that several CRUD operations could be mechanized once the standard service-method shape stabilised:

- **Fully mechanizable today:** `Delete` (archive) for any registered Entity. The sequence `Authorize → txhelper.Run(ArchiveEntity → Observe) → ObserveAfterCommit` is purely entity-level with no per-type variation.
- **Mostly mechanizable:** `Get` (entity-level fields only); `List` (entity-level filter by type slug, archived status, time range) once Phase D's per-resource queries stabilise.
- **Partially mechanizable:** `Create` and `Update` have skeleton consistency but per-type variation (validation, encryption, multi-table writes, sub-type dispatch in CTI).
- **Out of scope:** Tags (two-principal model), contacts (not entities).

A speculative `EntityDescriptor` registry sketch is in the round-1 handoff report. Not in scope for any current phase; revisit when the service-method shape has been stable across a few new modules and the mechanical-vs-handcrafted boundary becomes obvious.

## Component workbench (Ladle)

See `stories-next.md` at this module's root for the deferred Ladle / Storybook follow-ups (story coverage gaps, Storybook migration path, a11y / theme / MSW addons, visual-regression options).
