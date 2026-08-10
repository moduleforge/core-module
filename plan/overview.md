# Catalog Readiness — mod-core: durable field-encryption key bootstrap

## Purpose and scope

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

## Current status

**`complete`.** Investigation of the real code (`api/internal/fieldcrypto/
fieldcrypto.go`, `api/fieldcrypto/fieldcrypto.go`,
`moduleforge.module.yaml`, `model/migrations/`, `model/db/`) and of a real
generated composition root (`app-mftodo/cmd/server/main.go`, read-only,
confirmed via the sibling plan's own research) resolved every open design
question with enough confidence to fully decompose the work — no research
delegation or user decision blocks task breakdown. Two design points worth
the user's attention are recorded under
[Deferred and flagged](#deferred-and-flagged) and returned via
`flagged_for_manager` rather than `user_questions`, since they don't block
starting this phase. Phase 1 (`durable-field-key`) begins first; its three
tasks are strictly sequential (schema → implementation → tests). A
`doc-updates` phase follows per the architectural-implications check (new
tracked state, new public API surface).

## Overview

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

## Deferred and flagged

Returned via `flagged_for_manager` (not `user_questions` — neither blocks
this phase's task breakdown, which is fully resolved):

- **Manifest default-behavior swap.** This plan changes
  `moduleforge.module.yaml`'s `cipher` service constructor unconditionally
  (`fieldcrypto.NewFromEnv` → `fieldcrypto.NewFromEnvOrGenerate`), so *every*
  ModuleForge app composing `mod-core` picks up auto-generate-and-persist by
  default the next time it regenerates its composition root against an
  updated `mod-core` version pin — not just `app-mftodo`. `CORE_FIELD_KEY_HEX`
  set explicitly still always wins (identical behavior to today), but an app
  that previously relied on the hard-fail as a "did the operator forget to
  set this" safety net will instead boot with a silently auto-generated key
  once it updates its pin. Worth the module owner's (or the user's)
  explicit sign-off, since no manifest-level per-app opt-out/opt-in
  mechanism exists today short of continuing to set the env var.
- **Cross-project pattern consistency.** The sibling `mod-users` project's
  analogous `JWT_SECRET` work faces a harder sequencing constraint (checked
  inside `config.Load()` before any DB pool exists) that this plan's
  `CORE_FIELD_KEY_HEX` work does not. If `mod-users` ends up needing a
  different mechanism as a result, the two secrets would durably persist via
  different patterns — worth an explicit decision on whether that is
  acceptable or whether `mod-users` should be re-sequenced to match this
  plan's owned-table pattern instead.
- **`docs-mf-standards` content update.** The user's answer to the sibling
  `app-mftodo` plan calls for a `docs-mf-standards` update documenting the
  new env-var-override/secret-bootstrap convention, plus bumping the
  submodule pointer in every consumer (`app-mftodo`, `mod-core`, `mod-users`).
  `docs/mf-standards` is a git submodule (`docs-mf-standards.git`) — a
  separate repository, not this project's `project_root` — so that content
  update is out of this plan's scope by the operating contract and needs its
  own dispatch against that repo directly (with this plan's own
  `moduleforge.module.yaml`/`AGENTS.md` changes as its primary source
  material). This plan does not bump the submodule pointer, since the
  submodule's own content hasn't changed yet.
