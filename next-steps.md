# core-module — next steps

All 8 planned phases (bootstrap → extract model → wire model → core API → wire API → extract UI → verification → tax-id encryption) have been implemented. Items below are pending manual verification or deferred work that surfaced during implementation. Original phase reports were in `plan/` (now removed); this file is the forward-looking residue.

## Pending manual verification (needs live stack / DB)

- **`make dev.start` end-to-end smoke** — bring up the full stack, log in, edit profile at `/profile`, reload to confirm persistence; create a user via admin; edit an existing user.
- **`atlas migrate status`** against a freshly-migrated local DB — confirm all migrations applied in order: `0000–0005` (core) → `0100–0109` (users-module) → `0200–0201` (tags-module).
- **Audit log** — after a profile edit, verify `SELECT op, resource, actor_user_id FROM audit_log ORDER BY at DESC LIMIT 5` shows `op='update'`, `resource='natural_person'`, correct `actor_user_id`, populated `before`/`after`.
- **Phase 8 tax-id round-trip** — verify `Cipher.Decrypt` on an empty-but-non-NULL bytea cleanly yields `""` against live Postgres. Inspect the raw column (`SELECT encode(ssn, 'hex') FROM natural_persons WHERE entity_id = <id>;`) to confirm the plaintext SSN bytes are NOT present.

## Known open items (code-level, deferred)

- `EntityService.GetByID` in core-api returns `ErrNotFound` unconditionally — sqlc didn't emit `GetEntityByID` (only `GetEntityByUUID`). No caller uses it today; add the sqlc query when needed.
- `ServiceAccountService.UpdateByEntityUUID` returns `ErrInvalidInput` because no `UpdateServiceAccount` sqlc query exists. Wiring is in place; add the query when service-account updates become a real use case.
- Entity forms (`<NaturalPersonForm>`, `<CorporationForm>`, `<ServiceAccountForm>`) exist in core-gui but the admin user-edit page still renders its own inline form. Consolidation was intentionally deferred ("leave admin pages alone — we can consolidate later").
- Integration test for `PUT /v1/self` is unit-level (mocked) — users-module/api has no testcontainer harness. If a DB-backed harness is added later, upgrade the test.
- `getCorporation` has no admin-or-self enforcement gate (pre-existing, not a Phase 8 regression). The tax_id response gate still correctly withholds `ein` for non-admin non-subject callers, but the general GET still leaks other fields. Worth a targeted fix.

## Tax-id encryption — flagged for future hardening

- **Key rotation.** No mechanism for rotating the AES-256-GCM key. A future hardening step would embed a key-id prefix in the blob and introduce a re-encryption migration.
- **AAD binding.** Ciphertext is not bound to the row's primary key, so a ciphertext could in principle be copied between rows. A source comment documents this as future work.

## Component workbench (Ladle)

See `stories-next.md` at this module's root for the deferred Ladle / Storybook follow-ups (story coverage gaps, Storybook migration path, a11y / theme / MSW addons, visual-regression options).
