# Key Store Model Layer

## Purpose and scope

Phase summary for the model-layer half of fieldcrypto key rotation: the replacement multi-key
storage schema and the queries the cipher and its call sites need. Task breakdown deferred until
the [open decisions](../overview.md#open-decisions) are resolved.

## Goals

Establish, before any Go code changes, what a persisted key row looks like and how a rotated blob
is written back. This phase comes first because both later phases code against contracts it
defines: Phase 2's `FieldKeyQuerier` is generated from the queries written here, and Phase 3's
write-back has no query to call until the narrow blob-only updates exist here.

It replaces `model/migrations/0017_field_crypto_keys.sql` and
`model/queries/field_crypto_keys.sql` outright — a breaking schema change with no in-place
migration and no compat path, per the plan's no-backfill constraint. The single-row table's
`id = 1` `CHECK` is what makes concurrent first-boot inserts converge on one winner today; a
multi-row table has to re-establish that guarantee some other way, and doing so is a goal of this
phase rather than an afterthought.

## Inputs

- The existing `0017` migration and `field_crypto_keys.sql` queries, both being replaced.
- Existing `natural_persons` and `corporations` DDL and their `UpdateNaturalPerson` /
  `UpdateCorporation` queries, whose `COALESCE`-based "leave unchanged" idiom is unsuitable for a
  blob-only rotation write.
- Open decision 1 (wire format) — determines whether encrypted columns gain a sibling version
  column, and the version identifier's SQL type.
- Open decision 2 (key lifecycle) — determines whether the table needs an explicit active marker
  and whether env-supplied keys are persisted.
- Open decision 4 (schema and migration mechanics) — the concrete DDL and whether `0017` is edited
  in place or superseded.
- mod-core's module-local migration numbering (`goose_db_version_core`), so a new migration takes
  the next free number with no cross-module coordination.

## Outputs

- A replacement key-storage schema admitting an indefinite number of versioned key rows, exactly
  one of which is active, with first-boot convergence preserved.
- Replacement queries in `model/queries/field_crypto_keys.sql` covering: fetch the active key,
  fetch all keys for decryption, and insert-if-absent on first boot.
- Narrow blob-only update queries for `natural_persons.ssn` and `corporations.ein`, keyed by
  `entity_id`, that touch no other column — the write-back Phase 3 calls.
- Regenerated, committed `model/db/` (`cd model && make gen`), which is what fixes the concrete
  method set Phase 2's `FieldKeyQuerier` interface must be structurally satisfied by.
- `cd model && make verify` and `cd model && make lint` passing against the replacement schema.
