# Overview

## Purpose and scope

Give mod-core's `fieldcrypto` package real key-rotation support, closing followup item `iQt1`.
Today `fieldcrypto.Cipher` holds exactly one 32-byte key and stored ciphertext carries no key
identity, so a leaked `CORE_FIELD_KEY_HEX` cannot be retired without a from-scratch data migration.
This plan introduces a versioned ciphertext format, an indefinite set of decrypt-only retired keys
alongside one active encrypt key, and transparent re-encryption on read — the behavior the package
doc has promised since it was written but never implemented.

Scope is mod-core only: `api/internal/fieldcrypto/`, its façade `api/fieldcrypto/`, the
`model/migrations/` and `model/queries/` changes needed for multi-key storage, `model/db/`
regeneration, mod-core's own call sites in `api/service/`, and the `cipher` service block in
`moduleforge.module.yaml`. No file outside `/Users/zane/playground/moduleforge/mod-core` is edited
by any task in this plan; cross-repo consequences are recorded under
[Deferred and flagged](#deferred-and-flagged) instead.

## Current status

Planning is **blocked on open decisions**. Three phases are registered with their goals, inputs,
and outputs drafted; task breakdown is deliberately deferred because the answers to
[Open decisions](#open-decisions) below determine the shape of the wire format, the replacement key
schema, and the call-site API — which is to say, they determine nearly every task in every phase.

No phase has begun. When the open decisions are resolved and recorded in the notes files named
against each one, this plan is re-invoked to produce the task breakdown.

## Overview

Three phases, strictly sequential: the model layer establishes what a key row and a rotation
write-back look like, the cipher core consumes that contract and defines the on-disk blob format,
and the call sites wire re-encrypt-on-read through mod-core's read paths. Phase boundaries were
drawn so each phase leaves the module compiling and its tests passing.

**Phase 1 — Multi-Key Storage Model Layer** ([`phases/key-store-model.md`](./phases/key-store-model.md)).
Replaces the single-row `field_crypto_keys` table outright — migration `0017` and
`model/queries/field_crypto_keys.sql` both — with a multi-row versioned design, adds the narrow
blob-only update queries rotation write-back needs, and regenerates `model/db/`. Its output is the
persistence contract the next phase codes against. Task breakdown deferred.

**Phase 2 — Versioned Multi-Key Cipher** ([`phases/versioned-cipher.md`](./phases/versioned-cipher.md)).
Defines the version-tagged blob format, reworks `Cipher` to hold one active key plus an indefinite
set of decrypt-only retired keys, reworks the constructors and the `FieldKeyQuerier` contract, and
rewrites the package doc that currently declares rotation out of scope. Depends on Phase 1. Task
breakdown deferred.

**Phase 3 — Re-Encrypt-On-Read Call Sites** ([`phases/rotate-on-read.md`](./phases/rotate-on-read.md)).
Re-exports the new surface through `api/fieldcrypto/`, updates the `moduleforge.module.yaml`
`cipher` service block if the constructor signature moved, and wires rotation through the six
decrypt sites in `api/service/`. Depends on Phase 2. Task breakdown deferred.

Documentation updates to `AGENTS.md` (its `fieldcrypto/` and `model/migrations/` rows both describe
the current single-key design) are expected to land in a `doc-updates` phase registered when this
plan completes, not as a fourth hand-authored phase.

## Clarified requirements

### What must change

1. **Versioned ciphertext.** Every stored blob carries a key-version tag identifying the key that
   encrypted it. This is a new, breaking on-disk format.
2. **Multi-key cipher.** One active key is used for all new encryption. An indefinite number of
   retired keys are held for decryption only and are never selected for encryption.
3. **Re-encrypt on read.** When a blob's key version is not the active version, the read path
   transparently re-encrypts it under the active key and persists the replacement.
4. **Replacement key storage.** The `field_crypto_keys` table is redesigned outright to carry
   multiple versioned key rows, replacing migration `0017` rather than migrating it in place.

### What must not change, and what must not be built

- **No backfill or compat path.** Nothing is deployed on the current untagged format. Old blobs are
  not read, detected, or converted; no dual-format decode path is built. Existing databases are
  regenerated from scratch after this lands.
- **No files outside mod-core.** Not app-mfmanager, not app-mftodo, not mod-users, and not
  `docs/mf-standards/` — which is a **git submodule** pointing at the separate `docs-mf-standards`
  repository, and is therefore cross-repo despite sitting under mod-core's `docs/`.
- **`CORE_FIELD_KEY_HEX` keeps winning when set.** The env-var-always-wins precedence over DB-held
  keys is existing, documented, deliberately-chosen behavior and is not up for revision here beyond
  whatever the multi-key model requires of it.
- **AAD row binding stays out of scope.** The pre-existing "no AAD is bound to ciphertext"
  limitation is a separate gap. It is adjacent — the version tag is a natural AAD candidate — but
  binding ciphertext to a row id is not a requirement of this plan.
- **Followup `MnvB` (zeroing key byte slices) is not required.** Phase 2 necessarily rewrites
  `fromPersistedOrGenerated`, the exact function `MnvB` concerns, so its task doc should note that
  as a natural incidental place to address it. It is explicitly not a blocking requirement and not
  an acceptance criterion.

### Success criteria

- A `Cipher` constructed with one active key and one or more retired keys decrypts blobs written
  under any of them, and encrypts new values only under the active key.
- Reading a value whose blob carries a non-active version leaves the stored blob re-encrypted under
  the active version, and a subsequent read finds it already current.
- A blob whose version matches no loaded key fails loudly rather than returning empty or wrong
  plaintext.
- `cd model && make verify` and `cd model && make lint` pass; `model/db/` is regenerated and
  committed; `cd api && make test` and `cd api && make lint` pass.
- No code path in mod-core reads or writes the old untagged format.

## Open decisions

Each decision below blocks task decomposition. The manager should resolve them with the user (or
via the named research delegation) and record the answers at the named notes path before this plan
is re-invoked.

1. **Key-version tag wire format** → [`notes/key-version-wire-format.md`](./notes/key-version-wire-format.md).
   Whether the tag lives inside the blob (a self-describing prefix ahead of the nonce) or in a
   sibling database column per encrypted field; what the version identifier is (a monotone integer
   from the key table, or a fingerprint derived from the key material); its width; and whether it is
   bound as AEAD additional authenticated data. The requirement's phrasing — a tag "alongside" each
   blob — admits both readings, and the choice cascades into the key-table schema, every encrypted
   column's DDL, and the `Encrypt`/`Decrypt` signatures.
2. **Key lifecycle and supply policy** → [`notes/key-lifecycle-policy.md`](./notes/key-lifecycle-policy.md).
   How retired keys reach the process in `CORE_FIELD_KEY_HEX` mode, whether an env-supplied active
   key is persisted into the key table, and — most importantly — what event makes a *new* key
   active. mod-core has no admin API, CLI, or operator surface that could trigger rotation today,
   and without a trigger the rotation machinery is never exercised.
3. **Re-encrypt-on-read API shape and failure policy** → [`notes/rotation-api-shape.md`](./notes/rotation-api-shape.md).
   Which of the three wiring options in
   [`notes/rotation-on-read-call-sites.md`](./notes/rotation-on-read-call-sites.md) to adopt, and
   what a read does when the write-back fails or the caller has no writable transaction.
4. **Replacement key-table schema and migration mechanics** →
   [`notes/key-store-schema-design.md`](./notes/key-store-schema-design.md). A research delegation,
   answerable once decisions 1 and 2 are settled: the concrete DDL, how first-boot convergence is
   preserved without the `id = 1` singleton that provides it today, and whether migration `0017` is
   edited in place or superseded by a new `0018` that drops and recreates.

Background for all four is in [`notes/fieldcrypto-current-state.md`](./notes/fieldcrypto-current-state.md)
and [`notes/rotation-on-read-call-sites.md`](./notes/rotation-on-read-call-sites.md).

## Deferred and flagged

Cross-repo follow-ons this plan deliberately does not perform. Each needs to be filed as a separate
cross-repo followup after this plan's rotation mechanism is designed; none may become a task here.

- **app-mfmanager `deploy/README.md`, "Secret generation, loss, and rotation"** *(explicitly
  requested by the user)*. That section currently states `CORE_FIELD_KEY_HEX` is
  "irrecoverable-if-lost and unrotatable-if-leaked", quotes fieldcrypto's package doc on rotation
  being out of scope, and tells the operator that retiring a leaked key "would first require
  building that re-encrypt-on-read capability in mod-core and then running a data migration". Once
  this plan lands, the "unrotatable-if-leaked" half becomes false and must be replaced with the
  real operational procedure: how a new active key is introduced, how the leaked key is demoted to
  retired, and — the non-obvious part — that **rotation is lazy**, so a retired key can only be
  discarded once every stored blob has actually been read and re-encrypted, not after any fixed
  interval. The "irrecoverable-if-lost" half stays true and must not be softened. That section's
  separate "no AAD is bound to ciphertext" bullet also needs review if the version tag ends up
  bound as AAD.
- **app-mfmanager's other `CORE_FIELD_KEY_HEX` references** — `deploy/prod/README.md`,
  `pre-prod-test.md`, and `docs/architecture/tier2-permissions-boundary-security-design-gcp.md` all
  name the variable and may carry the same unrotatable framing.
- **`docs-mf-standards` repository** (reached through mod-core's `docs/mf-standards/` **submodule**,
  so cross-repo despite the in-tree path). `manifest-spec.md` documents the `cipher` service's
  `NewFromEnvOrGenerate` constructor, and `architecture/secret-durability-design.md` documents the
  single-row `field_crypto_keys` table and the single-key constructor design in three places. Both
  go stale the moment this plan lands.
- **Composing apps' generated composition roots** — `app-mftodo` and `app-mfmanager` each carry a
  generated `cmd/server/main.go` built from mod-core's manifest. If Phase 3 changes the `cipher`
  service block's constructor or args, every composing app must regenerate against the new mod-core
  pin, and any app pinning an older mod-core keeps the old untagged format. Because there is no
  backfill path, each app's database must also be regenerated from scratch — coordinate the pin
  bump with that.
- **mod-users `.env.example:145`** ships a sample `CORE_FIELD_KEY_HEX`. If decision 2 introduces an
  env convention for retired keys, that example needs the new variables.
- **mfgen** — confirm the manifest service-block shape Phase 3 produces still renders correctly, if
  the `args:` list changes.
