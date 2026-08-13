# Key Store Model Layer

## Purpose and scope

Phase summary for the model-layer half of fieldcrypto key rotation: the replacement multi-key
storage schema and the queries the cipher, the read-path write-back, and the admin rotation API all
call. The design is settled in [`key-store-schema-design.md`](../notes/key-store-schema-design.md);
this phase implements it verbatim.

## Goals

Establish, before any Go code changes, what a persisted key row looks like, how a rotated blob is
written back, and how an operator-facing inventory reads key lifecycle state without touching key
material. This phase comes first because every later phase codes against contracts it defines:
Phase 2's `KeyStore` adapter is generated from the queries written here, Phase 3's write-back has no
query to call until the narrow blob-only updates exist, and Phase 4's admin routes are thin wrappers
over the rotation, mark-compromised, grace-window, and inventory queries.

It replaces `model/migrations/0017_field_crypto_keys.sql` **in place** and rewrites
`model/queries/field_crypto_keys.sql` outright — a breaking schema change with no in-place migration
and no compat path, per the plan's no-backfill constraint. The single-row table's `id = 1` `CHECK`
is what makes concurrent first-boot inserts converge on one winner today; the replacement
re-establishes that guarantee with a partial unique index on the one-active-key invariant.

## Inputs

- The settled DDL, query list, and rationale in
  [`key-store-schema-design.md`](../notes/key-store-schema-design.md), including the
  `ListFieldCryptoKeyMetadata` seventh query added by
  [`rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-task-requirements).
- The existing `0017` migration and `field_crypto_keys.sql` queries, both being replaced.
- The existing `natural_persons` and `corporations` DDL and their `UpdateNaturalPerson` /
  `UpdateCorporation` queries, whose `COALESCE`-based "leave unchanged" idiom cannot express
  "replace this blob" and so cannot serve the rotation write-back.
- The six in-repo `coredb.Querier` stub implementations, every one of which needs the added and
  removed methods reflected before the api module compiles again.
- mod-core's module-local migration numbering (`goose_db_version_core`) and the pinned sqlc
  version in `model/Makefile`.

## Outputs

- A replacement key-storage schema admitting an indefinite number of versioned key rows, exactly one
  of which is active, with first-boot convergence preserved by the
  `field_crypto_keys_one_active` partial unique index.
- Seven queries in `model/queries/field_crypto_keys.sql`: `ListUsableFieldCryptoKeys`,
  `InsertInitialFieldCryptoKey`, `RetireActiveFieldCryptoKey`, `InsertActiveFieldCryptoKey`,
  `MarkFieldCryptoKeyCompromised`, `SetFieldCryptoKeyDecryptableUntil`, and
  `ListFieldCryptoKeyMetadata` (no `key_bytes`, so "no key material crosses the admin boundary" is a
  compile-time property).
- Narrow compare-and-swap blob-only update queries for `natural_persons.ssn` and `corporations.ein`,
  keyed by `entity_id` and guarded on the old blob, that touch no other column.
- Regenerated, committed `model/db/`, which fixes the concrete method set Phase 2's façade adapter
  and Phase 4's handler are written against.
- All six in-repo `coredb.Querier` stub implementations updated so `cd api && make build` and
  `cd api && make test` still pass.
- `cd model && make verify` and `cd model && make lint` passing against the replacement schema.
- The `goose down-to 16 && goose up` (or full database recreate) recovery step recorded in
  `model/README.md` for anyone holding a database that already applied the old `0017`.
