# Plan Summary: catalog-readiness

## What was planned and why

This is `mod-core`'s leg of a three-project federated plan (shared slug
`catalog-readiness` across `mod-core`, `mod-users`, and `app-mftodo`). The
federating plan is `app-mftodo`'s own `catalog-readiness` plan, whose item 3
("auto-generate and durably persist `JWT_SECRET`/`CORE_FIELD_KEY_HEX` instead
of hard-failing at boot") hit a cross-repo wall: the actual read/validate/fail
logic for `CORE_FIELD_KEY_HEX` lives in `mod-core`'s own
`api/internal/fieldcrypto` package, not in `app-mftodo`'s repo. The user's
answer to that blocker (recorded in `app-mftodo`'s
`plan/notes/config-and-secrets-cross-repo-blocker.md`) chose the cross-repo
option: change `mod-core`'s (and separately, `mod-users`') own source
directly, so this plan spans `mod-core` as a dependent project rather than
shimming around it from `app-mftodo` alone.

**Scope, exactly:** give `api/internal/fieldcrypto` (and its public façade,
`api/fieldcrypto`) the ability to auto-generate and durably persist the
AES-256-GCM field-encryption key when `CORE_FIELD_KEY_HEX` is not supplied
via the environment, instead of only ever hard-failing. `NewFromEnv()`'s
existing signature and behavior are unchanged for every existing caller;
the new capability is additive (`NewFromEnvOrGenerate`), and this repo's own
`moduleforge.module.yaml` is updated so mfgen-generated composition roots
pick it up automatically. Nothing outside `mod-core`'s own `project_root`
is touched — the `mod-users` and `app-mftodo` legs of this federated plan are
handled by their own separate dispatches.

**Explicitly out of scope:** any change to `mod-users`, `app-mftodo`, or
`mfgen` (a separate sibling tool repo); any change to the `docs/mf-standards`
git submodule (`docs-mf-standards`, a distinct repository) — the user's
answer to the sibling `app-mftodo` plan also calls for a `docs-mf-standards`
content update describing this new convention, but that repo is not this
project's `project_root` and needs its own dispatch (see
[Deferred and flagged](#deferred-and-flagged) below).

### Key research finding that reshapes the task (read this first)

The change request's own design constraints speculated that `fieldcrypto`'s
cipher-init call might run *before* any DB pool exists in a generated
`main.go`, requiring the new capability to open its own ad hoc connection.
Reading the real generated `app-mftodo/cmd/server/main.go` (read-only
reference; app-mftodo's own repo is not edited by this plan) disproves that
for `CORE_FIELD_KEY_HEX` specifically:

- `pool, err := localdb.New(ctx, cfg)` — line 71.
- Module migrations run next, core's own first — `coremigrations.Migrate(...)`
  — line 88.
- `cipher, err := fieldcrypto.NewFromEnv()` — line 127, well *after* both the
  pool and mod-core's own migrations.

(It is `mod-users`' `JWT_SECRET` — checked inside `config.Load()` at line 66,
*before* the pool exists — that has the harder sequencing problem; that is
`mod-users`' own leg of this federated plan, not this one.)

Because the pool (and mod-core's own migration-applied schema) is already
available by the time the cipher is constructed, this plan does **not** need
a separate ad hoc DB connection, and a **mod-core-owned table in mod-core's
own `model/migrations/`** is a clean, low-risk home for the persisted key —
consistent with `mod-core`'s existing ownership of small system tables
(`types`, `apps`) in its own dedicated 1-99 migration range and its own
`goose_db_version_core` tracking table. The alternative design floated in the
change request (an app-injected persistence callback, keeping `mod-core`
schema-agnostic) is not adopted: it would push migration/schema-ownership
complexity onto every composing app for no benefit, given the pool is already
available and mod-core already owns comparably-scoped tables. See
[Deferred and flagged](#deferred-and-flagged) for the one related
consistency question surfaced to the user (whether `mod-users`' analogous
`JWT_SECRET` work should follow the same owned-table pattern, given it faces
a harder sequencing constraint mod-core does not).

### Phase 1 — `durable-field-key` (Durable Field-Encryption Key Bootstrap)

Three strictly sequential tasks (each depends on the previous; no
parallel-eligible grouping):

1. [`001-add-field-crypto-keys-table.md`](./phase-01-durable-field-key/001-add-field-crypto-keys-table.md)
   — new mod-core-owned migration (`model/migrations/0017_field_crypto_keys.sql`)
   for a private, single-row `field_crypto_keys` table, plus the two sqlc
   queries (`model/queries/field_crypto_keys.sql`) and regenerated
   `model/db/` code the implementation task needs.
2. [`002-implement-auto-generate-cipher.md`](./phase-01-durable-field-key/002-implement-auto-generate-cipher.md)
   — the new `NewFromEnvOrGenerate` constructor in
   `api/internal/fieldcrypto/fieldcrypto.go` (env-var-always-wins, DB-level
   first-writer-wins race safety via `ON CONFLICT DO NOTHING`, fail-loudly
   corruption handling), its façade re-export in `api/fieldcrypto/
   fieldcrypto.go`, and the `moduleforge.module.yaml` `cipher` service
   declaration update that lets mfgen wire it into any composing app's
   generated `main.go` automatically.
3. [`003-test-race-and-corruption-paths.md`](./phase-01-durable-field-key/003-test-race-and-corruption-paths.md)
   — unit tests (fake-querier-backed, covering every branch: env-wins
   short-circuit, absent-then-generate, lost-race-adopts-winner,
   corrupt-length fails loudly, read-error fails loudly) plus a real-Postgres
   integration test (following the `api/authz/setup/
   grant_table_integration_test.go` precedent) proving the DB-level
   uniqueness constraint actually resolves a concurrent first-boot race to
   one winner.

### Phase 2 — `doc-updates` (Documentation Updates)

Registered per the architectural-implications check: this plan adds
significant new tracked state (a new table) and a new public API surface
(`fieldcrypto.NewFromEnvOrGenerate`). `mod-core`'s own `project_root` has no
`docs/architecture.md` or `docs/*-spec.md` of its own (mod-core's shared
architecture doc lives in the `docs/mf-standards` git submodule, a separate
repository out of this plan's scope — see
[Deferred and flagged](#deferred-and-flagged)); the in-scope, in-repo
document that inventories exactly this kind of package/migration/table
information is `AGENTS.md` (see its "Key types and packages" and "Database
migrations" sections). One task:

1. `001-update-architecture-docs.md` — update `AGENTS.md` to record the new
   migration, table, and `fieldcrypto` capability.

## What shipped

### Phase 01 — Durable Field-Encryption Key Bootstrap

1. **Add Field Crypto Keys Table** (`001-add-field-crypto-keys-table.md`, tier `sonnet-med`) — Added the field_crypto_keys migration and its sqlc query pair, then regenerated model/db/ via sqlc generate exactly as specified. All Validation checks pass except the literal grep pattern (sqlc emits PascalCase, not snake_case — verified via case-insensitive grep instead). make verify, make lint, and go build all pass. Also hand-verified the goose Down block reverses cleanly.
   Commit `b1f2ece`, merged at `044a9edd39dfcf4b84ddb80116e278bfc07bd5d3`.

2. **Implement Auto-Generate Cipher** (`002-implement-auto-generate-cipher.md`, tier `sonnet-high`) — Implemented NewFromEnvOrGenerate in api/internal/fieldcrypto/fieldcrypto.go per the task doc's design: shared fromHexKey helper, narrow FieldKeyQuerier interface, fromPersistedOrGenerated implementing fetch-or-generate-and-persist with correct pgx.ErrNoRows race-loss handling. Re-exported through api/fieldcrypto facade. Wired moduleforge.module.yaml's cipher service entry. Originally blocked by a pre-existing querier-fake conformance gap from task 001 (now fixed and merged separately); after rebasing onto the fix, all validation checks pass including module-wide make lint.
   Commit `33f4e87`, merged at `59c9372b028306291ad2a4b82b8a72030a7373e4`.

3. **Test Race And Corruption Paths** (`003-test-race-and-corruption-paths.md`, tier `sonnet-high`) — Added generate_test.go (unit, fake-Querier-based, full branch coverage) and generate_integration_test.go (real Postgres: fetch/persist round trip, 8-goroutine concurrent race proving the DB-level uniqueness constraint, and a raw-SQL proof the octet_length=32 CHECK rejects a corrupt key). All unit and integration validation passed including a real non-skipped integration run.
   Commit `e5feda6`, merged at `5521dc650c094ca824e2cbd4030b245cf63208a6`.

### Phase 02 — Documentation Updates

1. **Update Architecture Docs** (`001-update-architecture-docs.md`, tier `sonnet-high`) — Updated AGENTS.md's fieldcrypto/ row and model/migrations row to document the new field_crypto_keys table (migration 0017) and fieldcrypto.NewFromEnvOrGenerate auto-generate-and-persist capability, verified against real Phase 1 source rather than plan intent alone. Confirmed no in-repo doc has stale boot-failure claims. docs/mf-standards submodule untouched. All validation checks passed; committed at 187b68e.
   Commit `187b68e`, merged at `0e922d29d77a9460cc17917adaef135d60d3aa3e`.

## Key decisions

_No `## Why this shape` section is recorded in `plan/overview.md`, so this plan's cross-task rationale was never written down. Per-task outcomes are under "What shipped" above._

## Follow-up items

- **`NF5p`** — **Task doc's Validation grep pattern (snake_cas** — Task doc's Validation grep pattern (snake_case) never matches sqlc's PascalCase output — pre-existing task-doc-template inaccuracy, not an implementation defect, worth correcting in future templates.

- **`GzQ5`** — **make lint / shadow-db-lint.sh only exercises** — make lint / shadow-db-lint.sh only exercises goose up, never down, for any migration — task doc assumed it also validates Down; verified manually instead. Possible follow-up if down-migration regressions should be caught in CI.

- **`iuHr`** — **Sandbox-only note: a native non-Docker Postgr** — Sandbox-only note: a native non-Docker Postgres on localhost:5432 shadows the shared container in this environment, causing the same pre-existing localhost-resolution issue the already-landed authz/setup integration test also hits. Worked around via CORE_DEV_PG_HOST pointed at the LAN IP for validation only, no source change. Worth awareness if other integration suites on this host need the same workaround.

- **`nXOo`** — **Task 001 grep validation unsatisfiable** — [Surfaced by phase-01 correctness review lens] plan/phase-01-durable-field-key/001-add-field-crypto-keys-table.md's Validation section names a literal `grep -n "field_crypto_keys" model/db/querier.go` check that can never pass — sqlc emits PascalCase Go identifiers (GetFieldCryptoKey/InsertFieldCryptoKeyIfAbsent), never the literal snake_case table name, for any table in this file. Already self-diagnosed and worked around by the task-001 implementer (case-insensitive grep substituted, documented in that task doc's own Status section) — not a defect in the shipped code. Worth correcting the Validation wording itself (e.g. to `grep -n -i "FieldCryptoKey" model/db/querier.go`) for future accuracy if this task-doc pattern is reused.

- **`MnvB`** — **Optional: zero field-crypto key after use** — [Surfaced by phase-01 escalated security review lens, suggestion severity, low confidence] api/internal/fieldcrypto/fieldcrypto.go's fromPersistedOrGenerated never explicitly zeroes the randomly generated candidate key or persisted-key byte slices after they're consumed by NewFromKey — they remain reachable in memory until GC's normal schedule rather than being scrubbed immediately after use. Optional hardening only: consider zeroing candidate (and persisted-key slices) via a defer after consumption by NewFromKey. Go's GC/escape-analysis semantics mean this is best-effort, not a guarantee — reviewer explicitly scored this a suggestion, not a blocking finding.

- **`UncB`** — **The docs/mf-standards submodule's architectur** — The docs/mf-standards submodule's architecture.md/manifest-spec.md likely need an equivalent update for the new convention, per plan/overview.md's own Deferred and flagged section — requires a separate dispatch against that repository; out of this task's scope.

- **`DYb4`** — **Manifest key-swap needs owner sign-off** — catalog-readiness Phase 1 changed moduleforge.module.yaml's cipher service constructor unconditionally (fieldcrypto.NewFromEnv -> fieldcrypto.NewFromEnvOrGenerate), so every ModuleForge app composing mod-core picks up auto-generate-and-persist by default the next time it regenerates its composition root against an updated mod-core version pin -- not just app-mftodo. CORE_FIELD_KEY_HEX set explicitly still always wins (identical behavior to today), but an app that previously relied on the hard-fail as a "did the operator forget to set this" safety net will instead boot with a silently auto-generated key once it updates its pin. Worth the module owner's (or the user's) explicit sign-off, since no manifest-level per-app opt-out/opt-in mechanism exists today short of continuing to set the env var.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — Durable Field-Encryption Key Bootstrap

- [x] [001-add-field-crypto-keys-table.md](./phase-01-durable-field-key/001-add-field-crypto-keys-table.md) — tier `sonnet-med` · branch `plan/catalog-readiness-01-001` · commit `b1f2ece` · merge `044a9edd39dfcf4b84ddb80116e278bfc07bd5d3`
- [x] [002-implement-auto-generate-cipher.md](./phase-01-durable-field-key/002-implement-auto-generate-cipher.md) — tier `sonnet-high` · branch `plan/catalog-readiness-01-002` · commit `33f4e87` · merge `59c9372b028306291ad2a4b82b8a72030a7373e4`
- [x] [003-test-race-and-corruption-paths.md](./phase-01-durable-field-key/003-test-race-and-corruption-paths.md) — tier `sonnet-high` · branch `plan/catalog-readiness-01-003` · commit `e5feda6` · merge `5521dc650c094ca824e2cbd4030b245cf63208a6`

### Phase 02 — Documentation Updates

- [x] [001-update-architecture-docs.md](./phase-02-doc-updates/001-update-architecture-docs.md) — tier `sonnet-high` · branch `plan/catalog-readiness-02-001` · commit `187b68e` · merge `0e922d29d77a9460cc17917adaef135d60d3aa3e`
