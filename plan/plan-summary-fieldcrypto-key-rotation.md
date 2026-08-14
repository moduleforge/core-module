# Plan Summary: fieldcrypto-key-rotation

## What was planned and why

Give mod-core's `fieldcrypto` package real key-rotation support, closing followup item `iQt1`.
Today `fieldcrypto.Cipher` holds exactly one 32-byte key and stored ciphertext carries no key
identity, so a leaked `CORE_FIELD_KEY_HEX` cannot be retired without a from-scratch data migration.
This plan introduces a versioned ciphertext format, an indefinite set of decrypt-only retired keys
alongside one active encrypt key, transparent re-encryption on read, and the admin HTTP surface that
actually triggers a rotation — the behavior the package doc has promised since it was written but
never implemented.

Scope is mod-core only: `api/internal/fieldcrypto/`, its façade `api/fieldcrypto/`, the
`model/migrations/` and `model/queries/` changes needed for multi-key storage, `model/db/`
regeneration, mod-core's own call sites in `api/service/`, a new admin handler in `api/httpapi/`, and
the manifest and OpenAPI entries that expose it. No file outside
`/Users/zane/playground/moduleforge/mod-core` is edited by any task in this plan; cross-repo
consequences are recorded under [Deferred and flagged](#deferred-and-flagged) instead.

## Purpose and scope

Give mod-core's `fieldcrypto` package real key-rotation support, closing followup item `iQt1`.
Today `fieldcrypto.Cipher` holds exactly one 32-byte key and stored ciphertext carries no key
identity, so a leaked `CORE_FIELD_KEY_HEX` cannot be retired without a from-scratch data migration.
This plan introduces a versioned ciphertext format, an indefinite set of decrypt-only retired keys
alongside one active encrypt key, transparent re-encryption on read, and the admin HTTP surface that
actually triggers a rotation — the behavior the package doc has promised since it was written but
never implemented.

Scope is mod-core only: `api/internal/fieldcrypto/`, its façade `api/fieldcrypto/`, the
`model/migrations/` and `model/queries/` changes needed for multi-key storage, `model/db/`
regeneration, mod-core's own call sites in `api/service/`, a new admin handler in `api/httpapi/`, and
the manifest and OpenAPI entries that expose it. No file outside
`/Users/zane/playground/moduleforge/mod-core` is edited by any task in this plan; cross-repo
consequences are recorded under [Deferred and flagged](#deferred-and-flagged) instead.

## Current status

Planning is **complete**. All four open decisions are settled and recorded in `plan/notes/`; the
design is specified to the level of concrete DDL, SQL, Go signatures, route tables, and status
mappings, and the task documents consume that design rather than re-deriving it.

Five phases are registered with twelve tasks between them. No phase has begun. Phase 1 begins first and
has no pre-conditions beyond a working `sqlc` (v1.28.0, pinned in `model/Makefile`) and Docker for
`cd model && make lint`'s ephemeral shadow Postgres. Phases 3 and 4 each carry an integration task
that needs a real Postgres reachable through `DATABASE_URL`.

## Overview

Five phases, strictly sequential. Phase boundaries are drawn so each one leaves the module compiling
and its tests passing.

**Phase 1 — Multi-Key Storage Model Layer**
([`phases/key-store-model.md`](./phases/key-store-model.md)). Establishes the persistence contract
every later phase codes against: the replacement `field_crypto_keys` table, its seven queries, and
the narrow blob-only write-back queries re-encrypt-on-read needs.

- `001-replace-field-crypto-keys-schema` — rewrite migration `0017` in place with the multi-key DDL,
  update the blob-format comments in `0010`/`0011`, and rewrite `model/queries/field_crypto_keys.sql`
  with all seven queries.
- `002-add-blob-rotation-write-back-queries` — add the two `:execrows` compare-and-swap updates for
  `natural_persons.ssn` and `corporations.ein`. **Parallel-eligible with 001** — different files,
  different tables.
- `003-regenerate-model-db-and-querier-stubs` — `make gen`, commit `model/db/`, and repair all six
  in-repo `coredb.Querier` stub implementations. Depends on both 001 and 002.

**Phase 2 — Versioned Multi-Key Cipher**
([`phases/versioned-cipher.md`](./phases/versioned-cipher.md)). The security-critical centre: the
version-tagged, AAD-bound blob format and a reloadable keyring.

- `001-multi-key-cipher-core` — rewrite `api/internal/fieldcrypto` (key types, `keySet`, wire format,
  `Encrypt`/`Decrypt`/`DecryptWithRotation`/`Reload`/`BlobVersion`, bootstrap, staleness handling,
  package doc, all three test files), plus the mechanical `ctx` threading that keeps the api module
  building.
- `002-facade-key-store-adapter` — give `api/fieldcrypto` its real `coredb`-typed `FieldKeyQuerier`
  and the adapter that keeps the internal package model-free and the manifest's `cipher` block
  unchanged. Depends on 001.

**Phase 3 — Re-Encrypt-On-Read Call Sites**
([`phases/rotate-on-read.md`](./phases/rotate-on-read.md)). The user-visible half: reading a value
stored under a retired key leaves it re-encrypted under the active key.

- `001-rotating-cipher-helper` — the new `api/service/rotating_cipher.go`, which owns persistence,
  the write-back transaction, and the whole failure policy in one four-branch core.
- `002-wire-decrypt-call-sites` — convert the six decrypt sites and four encrypt sites, change three
  service field types and one variadic parameter, and compose the wrapper inside `service.New`
  without changing its exported signature. Depends on 001.
- `003-rotate-on-read-integration-test` — prove the headline success criterion against a real
  database, across both encrypted columns, including grace expiry and the compromised-key branch.
  Depends on 002.

**Phase 4 — Admin Key-Rotation API**
([`phases/admin-rotation-api.md`](./phases/admin-rotation-api.md)). Without this the machinery is
inert: nothing can create a second key version. HTTP only — mod-core produces no binary, so a CLI
would mean introducing its first executable and a second authorization story.

- `001-field-crypto-key-handler` — `FieldCryptoKeyHandler`, four wildcard-grant-admin-only routes,
  the rotation transaction, post-commit `Reload`, status mapping, and observer dispatch.
- `002-manifest-and-openapi-routes` — the `coreFieldCryptoKeyHandler` service block, its route entry,
  and the four documented routes. **Parallel-eligible with 001** — neither file participates in the
  Go build and the constructor signature is fixed by the design.
- `003-rotation-endpoint-integration-test` — the concurrency claim: two simultaneous rotations give
  one 201 and one 409, never two active keys. Depends on 001.

**Phase 5 — Documentation Updates.** One task, `001-update-architecture-docs`, bringing `AGENTS.md`
and the other in-repo reference docs in line with the delivered design — in particular the
operator-visible `CORE_FIELD_KEY_HEX` precedence change, the fact that the database now holds every
key version, and the laziness of rotation.

## Clarified requirements

### What must change

1. **Versioned ciphertext.** Every stored blob is `version(4) || nonce(12) || ciphertext || tag(16)`,
   with the 4 version bytes bound as AEAD additional authenticated data. New, breaking on-disk
   format.
2. **Multi-key cipher.** One active key encrypts everything new; an indefinite number of retired keys
   decrypt only and are never selected for encryption. The key set is reloadable at runtime.
3. **Re-encrypt on read.** A read of a non-active-version blob transparently re-encrypts it under the
   active key and persists the replacement, via a compare-and-swap update in its own short
   transaction.
4. **Per-key failure policy.** A standard rotation's retired key tolerates a failed write-back (log,
   return the plaintext); a key flagged compromised fails the read whenever the replacement cannot be
   durably persisted and the compromised ciphertext is not verifiably already gone.
5. **Replacement key storage.** `field_crypto_keys` is redesigned outright — versioned rows, a
   partial unique index enforcing one active key, an optional absolute grace deadline, and a
   compromise flag — by editing migration `0017` in place.
6. **An admin rotation API.** Four HTTP routes on a new handler, authorized as `manage` with a nil
   target (wildcard-grant admins only), supporting server-side key generation and operator-supplied
   `key_hex`, standard and compromised rotation, after-the-fact compromise flagging, and grace-window
   adjustment.

### What must not change, and what must not be built

- **No backfill or compat path.** Nothing is deployed on the current untagged format. Old blobs are
  not read, detected, or converted; no dual-format decode path is built. Existing databases are
  regenerated from scratch after this lands.
- **No files outside mod-core.** Not app-mfmanager, not app-mfdemo, not app-mftodo, not mod-users, and
  not `docs/mf-standards/` — which is a **git submodule** pointing at the separate
  `docs-mf-standards` repository, and is therefore cross-repo despite sitting under mod-core's
  `docs/`.
- **`moduleforge.module.yaml`'s `cipher` service block is untouched.** The façade adapter exists
  precisely so the constructor name, arg sources, and error return stay identical. Phase 4 adds a
  *separate*, additive handler service block; it does not modify this one.
- **`service.New`'s exported signature is fixed.** Every composing app calls it verbatim from
  generated code.
- **AAD row binding stays out of scope.** The version prefix is now bound as AAD, but binding
  ciphertext to a row id remains a separate, acknowledged gap — the package doc must narrow its
  existing claim to "no *row identity* is bound" rather than delete it.
- **Transport-level protection for the admin routes is out of scope.** App-level authz is what this
  plan delivers; a separate admin listener, ingress allow-list, or mTLS is a deployment concern,
  deferred below.
- **Followup `MnvB` (zeroing key byte slices) is not required.** Phase 2 rewrites
  `fromPersistedOrGenerated`, the exact function it concerns, so its task doc notes it as a natural
  incidental opportunity. It is not a blocking requirement and not an acceptance criterion.

### Success criteria

- A `Cipher` holding one active key and one or more retired keys decrypts blobs written under any of
  them, and encrypts new values only under the active key.
- Reading a value whose blob carries a non-active version leaves the stored blob re-encrypted under
  the active version, and a subsequent read finds it already current.
- A blob whose version matches no loaded key — unknown, or past its grace deadline — fails loudly
  rather than returning empty or wrong plaintext.
- A rotation performed through the admin endpoint retires exactly one key and activates exactly one
  replacement, and two concurrent rotations never leave two active keys.
- A read that cannot persist a re-encryption away from a **compromised** key fails; the same read
  under a standard-rotation key succeeds and logs.
- No response from any admin route contains key material.
- `cd model && make verify`, `cd model && make lint`, `cd api && make test`, `cd api && make lint`,
  and a whole-module `make build` all pass; `model/db/` is regenerated and committed.
- No code path in mod-core reads or writes the old untagged format.

## Settled decisions

All four originally-open decisions are resolved. Each note is the authoritative statement of its
decision; task documents consume them rather than restating the reasoning.

1. **Key-version tag wire format** →
   [`notes/key-version-wire-format.md`](./notes/key-version-wire-format.md). In-blob prefix, not a
   sibling column; bound as AEAD AAD.
2. **Key lifecycle and supply policy** →
   [`notes/key-lifecycle-policy.md`](./notes/key-lifecycle-policy.md). The database is the single
   source of truth for every key version. `CORE_FIELD_KEY_HEX` is a first-boot-only bootstrap seed
   and fails construction loudly on a later mismatch. Rotation is triggered by a new admin endpoint.
3. **Re-encrypt-on-read API shape and failure policy** →
   [`notes/rotation-api-shape.md`](./notes/rotation-api-shape.md). Per-key policy (standard grace
   period vs. compromised fail-fast); hybrid wiring — a pure rotation-aware decrypt in `fieldcrypto`
   plus a column-agnostic persistence helper in `api/service`; the full admin API design.
4. **Replacement key-table schema and migration mechanics** →
   [`notes/key-store-schema-design.md`](./notes/key-store-schema-design.md). Full replacement DDL,
   integer version via `GENERATED ALWAYS AS IDENTITY`, partial unique index for the one-active
   invariant and first-boot convergence, absolute `decryptable_until`, `compromised_at`, and an
   in-place edit of migration `0017` with no `0018`.

Background for all four is in
[`notes/fieldcrypto-current-state.md`](./notes/fieldcrypto-current-state.md) and
[`notes/rotation-on-read-call-sites.md`](./notes/rotation-on-read-call-sites.md).

## Deferred and flagged

Cross-repo and out-of-scope follow-ons this plan deliberately does not perform. Each needs to be
filed as a separate followup; none may become a task here.

### Compile breaks in other repositories — highest severity

**Deleting `fieldcrypto.NewFromEnv()` breaks compilation, not just documentation**, in three
composition roots outside this repository: `app-mfmanager/cmd/server/main.go:127`,
`app-mfdemo/cmd/server/main.go:112`, and `mod-users/api/cmd/server/main.go:264`. Each calls
`NewFromEnv()` directly even though mod-core's manifest declares `NewFromEnvOrGenerate`, and each
must move to `NewFromEnvOrGenerate(ctx, coredb.New(pool))` when it bumps its mod-core pin. Because
there is no backfill path, each must recreate its database at the same time. This is deliberately
recorded as a distinct, higher-severity item, separate from the documentation-staleness entries
below: those go stale silently, this one fails the build.

### Documentation and deployment follow-ons

- **app-mfmanager `deploy/README.md`, "Secret generation, loss, and rotation"** *(explicitly
  requested by the user)*. That section states `CORE_FIELD_KEY_HEX` is "irrecoverable-if-lost and
  unrotatable-if-leaked", quotes fieldcrypto's package doc on rotation being out of scope, and tells
  the operator that retiring a leaked key "would first require building that re-encrypt-on-read
  capability in mod-core and then running a data migration". Once this plan lands the
  "unrotatable-if-leaked" half becomes false and must be replaced with the real procedure. It must
  cover: the new admin rotation routes; that `CORE_FIELD_KEY_HEX` is now a first-boot-only seed that
  **fails startup loudly** if it later disagrees with the active key; that **rotation is lazy**, so a
  retired key can only be discarded once every stored blob has actually been read and re-encrypted,
  never after a fixed interval; that after a **compromised** rotation the fleet should be restarted
  rather than relying on the key-set TTL, if a hard guarantee is wanted that no process is still
  encrypting under the leaked key; and that a **database dump now carries every key version**, where
  an env-only operator previously kept key material out of the database entirely. The
  "irrecoverable-if-lost" half stays true and must not be softened. That section's separate "no AAD
  is bound to ciphertext" bullet also needs rescoping, since the version prefix is now AAD-bound
  while row identity still is not.
- **app-mfmanager's other `CORE_FIELD_KEY_HEX` references** — `deploy/prod/README.md`,
  `pre-prod-test.md`, and `docs/architecture/tier2-permissions-boundary-security-design-gcp.md` all
  name the variable and may carry the same unrotatable framing.
- **`docs-mf-standards` repository** (reached through mod-core's `docs/mf-standards/` **submodule**,
  so cross-repo despite the in-tree path). `manifest-spec.md` documents the `cipher` service's
  `NewFromEnvOrGenerate` constructor, and `architecture/secret-durability-design.md` documents the
  single-row `field_crypto_keys` table and the single-key constructor design in three places.
- **mod-users `.env.example:145`** ships a sample `CORE_FIELD_KEY_HEX`. Its accompanying comment
  needs to state that the variable is now a first-boot bootstrap seed only.
- **Composing apps' generated composition roots.** Phase 4 adds a manifest service and route entry
  (additively; the `cipher` block is untouched), so a composing app must regenerate against the new
  mod-core pin to expose the rotation routes. Because there is no backfill path, each app's database
  must also be recreated from scratch — coordinate the pin bump with that.
- **mfgen** — confirm the new `coreFieldCryptoKeyHandler` service block and its `register:` route
  entry render correctly.

### Deliberately deferred capability

- **Transport-level protection for the rotation routes** — a separate admin listener, ingress
  allow-list, or mTLS. Resolved as out of scope: app-level wildcard-grant-admin authz is what this
  plan specifies; network-level restriction is a deployment concern for the app-mfmanager deploy
  docs.
- **Envelope-encrypting `key_bytes` under a KEK supplied by the environment.** With the database as
  single source of truth, a dump compromises data encrypted under *every* key version retroactively.
  This is an accepted consequence of a settled decision, and the natural mitigation is its own piece
  of work.
- **A rotation-progress metric.** The read-path write-back's outcome (`persisted` / `stale` /
  `error`) is currently only a structured log line; a counter is the right observability primitive,
  but mod-core has no metrics facility today.
- **Push-based key-set invalidation** (`LISTEN`/`NOTIFY`) and a configurable key-set TTL. The
  reload-on-unknown-version plus TTL-on-encrypt combination is sufficient; the constant is fine until
  an operator asks otherwise.
- **An `mfmanager`-side CLI wrapper** over the rotation routes. Reasonable, needs nothing from
  mod-core, and is explicitly not mod-core's to build — mod-core produces no binary.
- **A distinct grantable `rotate` operation.** Every admin route authorizes as `manage` because
  operation slugs are registered in mod-authz, another repository. Registering `rotate` there and
  changing one string here is a clean, isolated follow-on if operations wants the granularity.
- **`LegalEntityService` is dead code.** It has no constructor, is absent from the `service.Services`
  aggregate, and its `cipher` field is unexported, so two of the six decrypt call sites are provably
  unreachable. Phase 3 updates them for consistency; deciding whether the service should be
  constructed or deleted is separate work.

## What shipped

### Phase 01 — Multi-Key Storage Model Layer

1. **Replace Field Crypto Keys Schema And Queries** (`001-replace-field-crypto-keys-schema.md`, tier `sonnet-high`) — Replaced mod-core's single-row field_crypto_keys table with the versioned, multi-key design specified in plan/notes/key-store-schema-design.md: version GENERATED ALWAYS AS IDENTITY primary key, key_bytes with length CHECK and UNIQUE constraint, retired_at/decryptable_until/compromised_at lifecycle columns, two named CHECK constraints, the field_crypto_keys_one_active partial unique index, and an updated_at trigger — reproduced verbatim from the design note, in place in 0017 with no 0018. Rewrote model/queries/field_crypto_keys.sql with the seven specified queries. Updated blob-format comments in 0010/0011 to version||nonce||ciphertext||tag. Both make verify and make lint pass; manual Postgres sanity run confirmed the one-active invariant, rotation transaction ordering, and retired-only-flags CHECK. Two task-doc/design-note internal inconsistencies surfaced rather than silently resolved (see flagged_for_manager).
   Commit `080f065`, merged at `fc97d19`.

2. **Add Blob Rotation Write-Back Queries** (`002-add-blob-rotation-write-back-queries.md`, tier `sonnet-med`) — Added the two narrow compare-and-swap write-back queries the Phase 3 rotation-on-read helper will use: UpdateNaturalPersonSSNBlob and UpdateCorporationEINBlob, both :execrows with old_ssn/old_ein CAS predicates matching the design note exactly. cd model && make verify passed. No make gen, model/db/, or api/ changes made, per scope boundary.
   Commit `856aa80`, merged at `3e56a1b`.

3. **Regenerate Model DB And Update Querier Stubs** (`003-regenerate-model-db-and-querier-stubs.md`, tier `sonnet-med`) — Regenerated model/db/ via sqlc generate (v1.31.1) against the schema/queries merged from tasks 001/002; the generated FieldCryptoKey struct and per-query row/reuse shapes match the design exactly, including the corrected three-full-row-query requirement. Updated all six api/-side coredb.Querier test-stub implementations to drop the two deleted key methods and add the nine new ones as no-op stubs. model/db/, api build, api test, and api lint all pass. api/internal/fieldcrypto left alone per Requirement 6; still builds under default tags but now fails under -tags integration as predicted, recorded as expected breakage for Phase 2 to repair.
   Commit `6743ced`, merged at `a576cd4`.

### Phase 02 — Versioned Multi-Key Cipher

1. **Multi-Key Versioned Cipher Core** (`001-multi-key-cipher-core.md`, tier `opus-high`) — Rewrote api/internal/fieldcrypto as a reloadable multi-key cipher: immutable keySet behind atomic.Pointer, mutex-collapsed reload, version(4)||nonce(12)||ciphertext||tag(16) blob format with version bound as AAD. Encrypt/Decrypt/DecryptWithRotation/Reload take ctx; BlobVersion is package-level. NewFromKey (store-less) and NewFromEnvOrGenerate (guarded bootstrap, lost-race adoption, CORE_FIELD_KEY_HEX first-boot seed, fail-loudly on later mismatch) replace NewFromEnv, which is gone from both packages. Inline security review (review_focus) found the diff a net security improvement (AAD binding, key zeroing, boundary validation, reload rate limit); two hardening gaps fixed in-diff (all-zero key rejection), three suggestion-level findings recorded, none blocking. cd api && make build/test/lint, go test -race, go vet -tags integration, and a real-Postgres integration run (6/6 tests) all pass.
   Commit `0e0d058`, merged at `81ba702`.

2. **Facade Key Store Adapter** (`002-facade-key-store-adapter.md`, tier `sonnet-high`) — Replaced task 001's placeholder api/fieldcrypto/fieldcrypto.go with its real shape: a FieldKeyQuerier interface declared over coredb.FieldCryptoKey/coredb.Querier-shaped methods, an unexported keyStoreAdapter mapping rows onto fieldcrypto.KeyRecord with an explicit negative-version guard, and NewFromEnvOrGenerate(ctx, FieldKeyQuerier) wrapping the adapter — name/params/error-return unchanged from what moduleforge.module.yaml's cipher block already names, so the manifest required zero edits. Façade re-exports Rotation and BlobVersion alongside existing aliases. Full api build/test/lint green including -race; inline security review found no issues, confirming the adapter respects task 001's key-material zeroing contract.
   Commit `a467939`, merged at `b0d616e`.

### Phase 03 — Re-Encrypt-On-Read Call Sites

1. **Rotating Cipher Write-Back Helper** (`001-rotating-cipher-helper.md`, tier `opus-med`) — Added api/service/rotating_cipher.go: RotatingCipher (cipher + pool-backed txhelper.DB + querier factory + logger), ssnColumn/einColumn blobColumn descriptors, the four-branch decryptRotating policy core, writeBack (own short transaction, SET LOCAL lock_timeout first, CAS on the exact blob read), and verifyStale (compromised-key re-read inside the same transaction). Standard rotation within grace window tolerates a failed write-back and returns plaintext at Warn; compromised key fails the read unless compromised ciphertext is verifiably already gone. No observer dispatched, nothing retried, write-back deliberately not atomic with the read. persisted never survives a txhelper.Run error. Eleven tests cover all four design-flagged cases plus happy path, both columns, fast paths.
   Commit `971c8e2`, merged at `beb2d72`.

2. **Wire Rotation Through Decrypt Call Sites** (`002-wire-decrypt-call-sites.md`, tier `sonnet-high`) — Wired the RotatingCipher from task 001 into all six decrypt call sites (profile.go x2, legal_entity.go x2, natural_person.go, corporation.go); the four Encrypt sites pick it up automatically via field-type change alone. service.New's exported signature untouched — composes one RotatingCipher internally via NewRotatingCipher(cipher, db, nil), shared between NaturalPersonService and CorporationService. LegalEntityService retyped for compile-correctness only. All existing service tests updated via a new testRotatingCipher(t) helper; two new tests added exercising rotation of a blob written under a retired key through the service layer end-to-end. api build/test/lint and repo-root three-subproject build all green. Inline security review found no issues — authorization ordering, entity-ID scoping, no-plaintext-logging invariant all preserved.
   Commit `4fc2e58`, merged at `c05c2bb`.

3. **Rotate-On-Read Integration Test** (`003-rotate-on-read-integration-test.md`, tier `sonnet-high`) — Added api/service/rotating_cipher_integration_test.go (build-tagged integration), proving the rotate-on-read success criterion against real Postgres. Three tests: standard rotation persisting and idempotent on second read (table-driven over both columns); grace-window-expiry failing loudly; compromised-key branch's three sub-cases (working handle succeeds, nil handle fails, genuinely-uncommittable write-back via real competing row lock fails). Database genuinely available and exercised twice in a row, both green. make test/vet/lint and git diff --stat all passed as specified.
   Commit `4b16f27`, merged at `f04b1b7`.

### Phase 04 — Admin Key-Rotation API

1. **Field Crypto Key Admin Handler** (`001-field-crypto-key-handler.md`, tier `opus-med`) — Added FieldCryptoKeyHandler with four admin-only routes (inventory, rotate, mark-compromised, grace) on /v1. Every route gates on Authorize(manage,nil) before any parsing or query. Rotation runs the two mandatory statements in one txhelper.Run, audits to observer group inside that transaction, reloads the cipher after commit (logging not failing on reload error). Status mapping 401/403/400/409/500 with 404-vs-409 split on version-scoped routes. Key material never crosses the boundary — every response/audit payload built from the material-free ListFieldCryptoKeyMetadata row type. 26 test functions cover authz gates, status mappings, idempotency, reload ordering/tolerance, and a sweep asserting no response body carries key bytes. Inline security review found no critical/major issue and directly confirmed Reload is unreachable without the admin credential and a committed rotation, via TestFieldCryptoKeyRotate_403_DoesNotReloadCipher — this answers the open followup MW1a.
   Commit `02d8505`, merged at `49ce33b`.

2. **Manifest And OpenAPI Route Entries** (`002-manifest-and-openapi-routes.md`, tier `sonnet-med`) — Added coreFieldCryptoKeyHandler service block and its /v1-group route entry to moduleforge.module.yaml, verbatim per design note, cipher block untouched. Documented all four admin rotation routes in api/openapi.fragment.yaml with new request/response schemas and a new Conflict reusable response. key_hex modeled as writeOnly on request schema only, never in any response schema. Both files parse as valid YAML, all task-specified grep checks pass, full make build at repo root succeeds.
   Commit `1343cca`, merged at `bf8843b`.

3. **Rotation Endpoint Concurrency Integration Test** (`003-rotation-endpoint-integration-test.md`, tier `sonnet-high`) — Added api/httpapi/field_crypto_keys_integration_test.go, a build-tagged integration suite proving the rotation endpoint's concurrency claim against real Postgres, following the TestMain/host-resolution/shadow-DB-reset pattern of the three existing sibling suites. Drives the real FieldCryptoKeyHandler over chi through httptest. Five tests: sequential happy path; headline concurrent-rotation race (exactly one 201, one 409, one active key, exactly one new row); unique-key_bytes 409 rejection; compromised-rotation effect on retired row; compromised+grace 400 rejected before any transaction runs. Key engineering finding: a plain channel-synchronized goroutine launch does not force genuine DB-level contention; the gatekeeper row-lock technique makes the race deterministic. All validation checks pass including a -race run of the concurrency test.
   Commit `eb0480d`, merged at `38301a9`.

### Phase 05 — Documentation Updates

1. **Update Architecture Docs** (`001-update-architecture-docs.md`, tier `sonnet-high`) — Updated AGENTS.md's fieldcrypto/, model/migrations/, and httpapi/ rows plus Conventions section to reflect the completed key-rotation work across phases 1-4: versioned multi-key storage, CORE_FIELD_KEY_HEX as first-boot-only seed (fails loudly on mismatch), lazy re-encrypt-on-read, FieldCryptoKeyHandler's four admin-only routes, and the fleet-wide TTL convergence caveat (partially addressing followup 8x4a). model/README.md and README.md confirmed accurate/unchanged; docs/architecture/ confirmed still absent. All validation checks pass; exactly one file changed.
   Commit `b36f200`, merged at `c8a740b`.

## Key decisions

_No `## Why this shape` section is recorded in `plan/overview.md`, so this plan's cross-task rationale was never written down. Per-task outcomes are under "What shipped" above._

## Follow-up items

- **`MEqf`** — **Task 002's coredb adapter must return freshly** — Task 002's coredb adapter must return freshly allocated KeyBytes it does not retain — the Cipher now zeroes key material a KeyStore hands it once AEADs are built (followup MnvB). A store that caches/reuses a returned slice would hand back zeros on its next call. Documented on the KeyStore interface, covered by TestKeyMaterialZeroedAfterLoad, backstopped by all-zero-key rejection — but 002's implementer needs to know explicitly.

- **`EoJ6`** — **fieldcrypto: cancelled-ctx reload budget** — reload() stamps lastLoadAttempt before issuing its query, so a decrypt driven by an already-cancelled request context burns the shared reload rate-limit budget (minReloadInterval) for no useful work — LoadUsableKeys returns ctx.Err() immediately. For the following window, encryptSet's TTL-driven refresh takes its early-return branch and silently keeps the stale snapshot (no log at all on that branch, unlike the reload-failure branch). This can delay a replica's convergence onto a rotated key set. Fix: don't charge a context-cancellation failure against the shared budget (restore lastLoadAttempt on ctx.Err()); log at warn from encryptSet's early-return branch when the snapshot is past keySetTTL but reload was rate-limited. Found by phase-2 boundary security review (opus-high), api/internal/fieldcrypto/fieldcrypto.go:335-347 and :404-424. Severity: minor, confidence: medium.

- **`pDiP`** — **fieldcrypto: missing retired-flags check** — buildKeySet re-validates most schema invariants (no/two active keys, duplicate version, version 0, wrong key length) but not the one enforced by the field_crypto_keys_retired_only_flags CHECK: that an active (retired_at IS NULL) row must not carry compromised_at or decryptable_until. Not reachable via the shipped SQL today (the CHECK constraint and the retired_at IS NOT NULL guards on the two mutation queries close it), but KeyStore is an interface — a non-DB implementation could violate it, silently producing MustPersist:false for a compromised-yet-active key, or an active key past its own grace deadline. Fix: reject such a record in buildKeySet's active-record branch with a loud error, mirroring the neighbouring checks. Found by phase-2 boundary security review (opus-high), api/internal/fieldcrypto/fieldcrypto.go:465-491. Severity: suggestion, confidence: high.

- **`0DI1`** — **fieldcrypto adapter: no zero on error path** — keyStoreAdapter zeroes key material via aliasing on the success path (records[i].KeyBytes shares memory with rows[i].KeyBytes, later zeroed by the cipher), but on a mapping failure (e.g. the negative-version guard) LoadUsableKeys returns nil, discarding both rows and records un-zeroed — asymmetric with the internal package's reload(), which defers zeroKeyMaterial before calling buildKeySet so a mid-build failure still wipes everything. Best-effort hygiene either way (pgx's wire buffers hold their own copies), but one corrupt row currently turns the hygiene property off for the whole key set. Fix: wipe rows[i].KeyBytes on the failure path in both LoadUsableKeys and InsertInitialKey before returning an error. Found by phase-2 boundary security review (opus-high), api/fieldcrypto/fieldcrypto.go:65-79. Severity: suggestion, confidence: high.

- **`pfuW`** — **fieldcrypto: MustPersist doc wording drift** — api/internal/fieldcrypto/fieldcrypto.go's package doc states MustPersist means "a caller that cannot durably store the replacement blob must fail the read", dropping plan/overview.md's qualifier "...and the compromised ciphertext is not verifiably already gone". The cipher's derivation is correct either way (the escape hatch is caller-side, owned by Phase 3's rotating-cipher-helper), but the wording should be reconciled before Phase 3 implements the caller so it doesn't implement the stricter reading by accident. Found by phase-2 boundary security review (opus-high).

- **`GPRN`** — **security-001 (followup material, not resolved** — security-001 (followup material, not resolved here): verifyStale at api/service/rotating_cipher.go:341 treats any stored key version other than the source version as already-rotated, so a row re-encrypted under a second compromised retired version by a process with a stale key snapshot lets this read succeed while compromised ciphertext remains at rest. Bounded and self-correcting on the next read. A safe refinement would be for fieldcrypto to expose whether an arbitrary version is compromised, letting verifyStale accept only a non-compromised replacement version — this needs new fieldcrypto API surface, not a trivial fix.

- **`qpPO`** — **Followup pfuW (MustPersist doc wording): read** — Followup pfuW (MustPersist doc wording): read plan/overview.md requirement 4 directly and implemented its full semantics including the 'verifiably already gone' qualifier via verifyStale — the implementation matches the overview. The api/internal/fieldcrypto package doc still carries the narrower wording; nothing in this task's diff touches that file, so pfuW remains open for the manager to dispose of.

- **`GTTY`** — **Note for task 002 (no action needed): service** — Note for task 002 (no action needed): service.New still constructs services with a bare *fieldcrypto.Cipher; nothing yet holds a RotatingCipher, so ssnColumn/einColumn are exercised only by tests until the call sites are wired.

- **`9l49`** — **rotating_cipher: narrow verifyStale check** — verifyStale accepts a lost CAS under a compromised key as benign whenever the re-read blob's version differs from the source version (`version != rot.FromVersion`), rather than requiring it equal the active version (`version == rot.ToVersion`). A process with a stale-but-within-TTL key snapshot could legitimately write under a second, also-compromised, retired version, and this check would wrongly treat that as "already rotated" — leaving compromised ciphertext at rest with the read reporting success. Bounded and self-correcting (needs two compromised rotations, a writer inside the stale-set window, and a lost CAS on the same row in the same instant; the next read by any non-stale process re-enters the compromised branch), but unpinned by any test in either direction — the one unit test covering this branch seeds the stored blob under the active version, and the integration suite exercises no lost-CAS path at all. Fix: replace `version != rot.FromVersion` with `version == rot.ToVersion` in verifyStale (strictly safer, same benign-race tolerance), and add a unit case seeding storedSSN under a third, retired, compromised version asserting the read fails. Originally flagged (low confidence) by phase-3 task-001's own inline security review; independently confirmed and confidence raised to medium by the phase-3 boundary security review (opus-high), which found no test distinguishes the two semantics. api/service/rotating_cipher.go:337-345.

- **`6hKu`** — **rotating_cipher: pool 2nd-conn deadlock risk** — RotatingCipher.writeBack opens its own transaction on the same pool service.New shares as the base querier. Services.Querier's own doc comment invites callers to derive a tx-scoped querier via coredb.New(tx) and pass it to GetByEntityUUID, so a caller already holding one pool connection inside its own transaction could trigger a rotating read that blocks acquiring a second connection from the same pool — with every connection held this way, the pool self-deadlocks until request contexts expire. SET LOCAL lock_timeout bounds the lock wait but not the connection-acquisition wait before the transaction begins. Reachability from mod-core itself is not established (mod-core's own httpapi handlers don't use this call shape), but out-of-repo composing apps might. Fix options: wrap the write-back's txhelper.Run in a context.WithTimeout on the order of the existing 250ms lock timeout so pool saturation degrades into an ordinary write-back failure the policy branches already handle; or give RotatingCipher a small dedicated pool distinct from the request pool. Either way, document on the db field that the handle must not be one a caller may already hold a connection from. Worth checking against mod-users and the composing apps (app-mfmanager, app-mfdemo) at the same time as the already-deferred NewFromEnv compile-break followup, since that followup already requires visiting all three composition roots. Found by phase-3 boundary security review (opus-high). api/service/rotating_cipher.go:248-256. Severity: minor, confidence: medium.

- **`PNVB`** — **rotating_cipher: lock-timeout 500s** — Under a compromised rotation, any competing row lock on the row being read (e.g. a legitimate concurrent update of the same entity's own subtype row) converts an ordinary read into a 500: the write-back's CAS blocks behind the lock, SET LOCAL lock_timeout='250ms' fires, and the compromised branch correctly fails the read (verified end-to-end by the new integration sub-case at rotating_cipher_integration_test.go:710-743). The failure direction is correct — the CAS never ran, so compromised ciphertext genuinely can't be shown to be gone — but this operator-visible consequence (sporadic 500s on reads of concurrently-edited rows during a compromise response) is not recorded anywhere. Two options: retry the write-back transaction once on a lock-timeout SQLSTATE 55P03 before adjudicating the policy branch (transient contention usually clears within a second attempt), or document the consequence explicitly. This should feed into the already-deferred app-mfmanager deploy/README.md 'Secret generation, loss, and rotation' rewrite (see plan/overview.md's Deferred and flagged section) alongside the existing guidance about restarting the fleet after a compromised rotation. Found by phase-3 boundary security review (opus-high). api/service/rotating_cipher.go:265-272. Severity: suggestion, confidence: high.

- **`fKHH`** — **rotating_cipher: missing std-rotation test** — The integration suite pins the compromised half of the per-key failure policy end-to-end against a real database (real field_crypto_keys rows, real RetireActiveFieldCryptoKey, real facade adapter), but never pins the standard half the same way — its only standard-rotation case has a write-back that succeeds. Every test exercising the tolerate-on-failure branch builds its fieldcrypto.KeyRecord by hand in Go with CompromisedAt left nil, so the SQL that actually derives that column (compromised_at = CASE WHEN compromised THEN now() ELSE NULL END in RetireActiveFieldCryptoKey) and the facade adapter's mapping of it are untested for the false case. An inversion or mis-mapping anywhere on that path would make every ordinary rotation silently adopt compromised-key policy — reads would start 500ing on any transient write-back failure (a read-only replica, a lock timeout, a permission error) while the entire test suite, including the standard integration case, stayed green. Fix: add a fourth sub-case to the standard-rotation integration test reproducing the compromised suite's competing-row-lock technique against a standard-rotated key, asserting the read succeeds and returns correct plaintext with the stored blob unchanged — the direct counterpart of the existing compromised sub-case at rotating_cipher_integration_test.go:710-743. Found by phase-3 boundary security review (opus-high). Severity: minor, confidence: medium.

- **`wXiC`** — **security-001 (suggestion): request bodies on** — security-001 (suggestion): request bodies on the two body-accepting routes are unbounded, matching every other api/httpapi handler — package-wide gap, not fixed here to avoid diverging from sibling handlers. Right fix is shared router middleware.

- **`aLt7`** — **security-002 (minor, low confidence): a pgcon** — security-002 (minor, low confidence): a pgconn.PgError's Detail can quote key_bytes; harmless under both stdlib slog handlers; only matters if a custom slog handler is ever installed.

- **`LO1V`** — **rotate: grace_period_days:0 convergence gap** — graceDaysParam accepts grace_period_days: 0 on the rotation route, resolving decryptable_until to the retirement instant itself — the key is already past its deadline the moment the transaction commits. Only the process that served the rotation reloads its cipher; every other process keeps the pre-rotation snapshot for up to the 60s key-set TTL and keeps sealing new blobs under the just-retired version, which are then unreadable on write (ListUsableFieldCryptoKeys filters the version out). Worse, since the version isn't in the loaded set at all, unusableVersionError takes the "unknown key version" branch instead of the one naming PUT /v1/field-crypto-keys/{version}/grace as the recovery action — least-actionable diagnostic for exactly the situation the grace route fixes. Recoverable (clearing the deadline via grace re-admits the version) but only by an operator who works out what happened. Every value >=1 is safe (a one-day window vastly exceeds the 60s convergence bound). Fix options need a design decision: reject grace_period_days:0 on the rotation route with a field error naming the convergence window, or floor the resolved deadline at retirement+margin exceeding the TTL, or keep 0 but document the consequence explicitly in the route's OpenAPI description. Found by phase-4 boundary security review (opus-high). api/httpapi/field_crypto_keys.go:617-629. Severity: minor, confidence: medium.

- **`vHIc`** — **Test: pin authz-before-parsing on 4 routes** — The fckRoutes 403 table covers all four routes but supplies a well-formed body and valid {version} in every case, so it establishes only that a denied request is refused, not that authorization is evaluated before parsing. The stronger invariant (a denied caller must never reach the body decoder or path-parameter parser) is unpinned. This matters because the sibling handler a maintainer would naturally copy from, AppsHandler.Create (api/httpapi/apps.go:138-162), does the opposite — decodes/validates before authorizing — so a future edit aligning this handler with that precedent would turn a 403 into a 400 on malformed input, silently disclosing input-validity information to an unauthorized caller, with every existing test still passing. Fix: extend the fckRoutes table with a deliberately invalid payload variant (malformed JSON on the two body-accepting routes, non-numeric {version} on the two version-scoped routes) and assert the response stays 403 rather than 400 when the authorizer denies. Found by phase-4 boundary security review (opus-high). api/httpapi/field_crypto_keys_test.go:406-471. Severity: suggestion, confidence: high.

- **`VZYK`** — **rotate: unzeroed active.KeyBytes in memory** — The file's own type doc states rotation is "the one place raw key material exists here, as a local that is zeroed on return" — true for the request-derived copy (deferred clear(keyMaterial)) but InsertActiveFieldCryptoKey's query also returns key_bytes among its result columns, so the returned coredb.FieldCryptoKey held in local `active` carries a second live copy of the brand-new active key material. Only Version and CreatedAt are read from it; KeyBytes is never zeroed and becomes garbage, readable in process memory until GC. Nothing is disclosed over the wire (activeKeyResponse is built from the two scalars, not the row) — this is memory hygiene, not a leak — but it's the one place the file's stated invariant is stronger than the code, misleading a future maintainer. Same class as existing followup MnvB (unzeroed key material in fromPersistedOrGenerated) — consider folding into one hardening item. Fix: either add a narrower sqlc query returning only version and created_at for the rotation insert, or clear(active.KeyBytes) immediately after Version/CreatedAt are copied out. Found by phase-4 boundary security review (opus-high). api/httpapi/field_crypto_keys.go:289-294. Severity: suggestion, confidence: high.

- **`8x4a`** — **Doc: compromised-rotation TTL window** — A rotation with compromised:true is the operator asserting the outgoing key has leaked, but post-commit convergence reloads only the process that served the request. Every other process continues encrypting new plaintext under the key just declared compromised for up to the 60s key-set TTL. PARTIALLY RESOLVED: phase 5's doc-updates task (001-update-architecture-docs) added this convergence-window caveat to AGENTS.md's Conventions section, covering the operator-runbook half. Remaining scope: the rotation route's OpenAPI description (api/openapi.fragment.yaml, POST /field-crypto-keys/rotations) still does not mention that a 201 response is not the point at which the compromised key stops being used fleet-wide — add a note there to fully close this out. Small, well-scoped, doc-only. Originally found by phase-4 boundary security review (opus-high). api/httpapi/field_crypto_keys.go:332-343. Severity: minor, confidence: medium.

- **`Wdku`** — **grace_period_days loose bound: 500 not 400** — graceDaysParam's upper bound is math.MaxInt32 (chosen to avoid wrap-to-negative), but that's not tight enough at the SQL sink: RetireActiveFieldCryptoKey and SetFieldCryptoKeyDecryptableUntil compute now() + grace_days * INTERVAL '1 day', and timestamptz tops out ~year 294277 (~1.07e8 days from now). Any accepted value above that (the range up to 2147483647 is wide open) raises a Postgres datetime overflow (SQLSTATE 22008) matching no apiresp sentinel, surfacing as a 500 with a server-side log entry rather than the 400 the input class deserves. No key material or internal detail reaches the client (admin-only route, apiresp fixes the public 5xx message), so impact is confined to error-class misreporting. Fix: tighten the upper bound to a value that cannot overflow timestamptz — a decade- or century-scale maximum as a named constant — and mirror it as a maximum on grace_period_days in both RotateFieldCryptoKeyRequest and SetFieldCryptoKeyGraceRequest in the OpenAPI fragment. Found by phase-4 boundary security review (opus-high). api/httpapi/field_crypto_keys.go:617-629. Severity: suggestion, confidence: medium.

- **`b4lw`** — **field-crypto-keys: full-scan not point query** — Rotate, MarkCompromised, and SetGrace each need the lifecycle state of exactly one already-known field_crypto_keys row, but all three resolve it via q.ListFieldCryptoKeyMetadata(ctx) — an unbounded, no-WHERE, ORDER-BY-version scan over every key version ever issued — then linear-scan the Go slice (findFieldCryptoKey) for the one row wanted, instead of a WHERE version = $1 point query against the PRIMARY KEY. field_crypto_keys is designed to grow indefinitely (retired keys never pruned), so every future admin call re-pays a cost that grows with the deployment's entire rotation history for information a single indexed lookup already has. Task 001's own status doc calls the single-read choice deliberate (avoids a second query, gives 404/409 off one consistent snapshot, supplies the observer's before-state) — that rationale justifies reading once, not reading the whole table. Separately for Rotate: RetireActiveFieldCryptoKey's UPDATE already computes retired_at/decryptable_until/compromised_at for the retiring row but its RETURNING clause discards them, returning only version — so the read-back is re-fetching values the write statement itself already computed. Fix: for Rotate, broaden RetireActiveFieldCryptoKey's RETURNING clause to include the three columns so the handler builds retiredState directly from the UPDATE's own result; for MarkCompromised/SetGrace, add a single-row query (e.g. GetFieldCryptoKeyByVersion) and have loadRetiredKey call that instead, preserving the same one-snapshot property while bounding the read. Found by phase-4 boundary efficiency review (sonnet-med). api/httpapi/field_crypto_keys.go (multiple sites), model/queries/field_crypto_keys.sql. Severity: minor, confidence: high.

- **`Vhhx`** — **Followup 8x4a is only partially addressed — t** — Followup 8x4a is only partially addressed — this task's scope (AGENTS.md, model/README.md, README.md) covers the operator-runbook half, but the followup also asks for a note in the rotation route's OpenAPI description (api/openapi.fragment.yaml), which is out of this task's file scope and was not touched. Manager should decide whether to close 8x4a now or keep it open pending the OpenAPI-description edit.

## Final Task State

# TODO

## Purpose and scope

Tracking document for the active plan.

## Tasks

### Phase 01 — Multi-Key Storage Model Layer

- [x] [001-replace-field-crypto-keys-schema.md](./phase-01-key-store-model/001-replace-field-crypto-keys-schema.md) — tier `sonnet-high` · branch `plan/fieldcrypto-key-rotation-01-001` · commit `080f065` · merge `fc97d19`
- [x] [002-add-blob-rotation-write-back-queries.md](./phase-01-key-store-model/002-add-blob-rotation-write-back-queries.md) — tier `sonnet-med` · branch `plan/fieldcrypto-key-rotation-01-002` · commit `856aa80` · merge `3e56a1b`
- [x] [003-regenerate-model-db-and-querier-stubs.md](./phase-01-key-store-model/003-regenerate-model-db-and-querier-stubs.md) — tier `sonnet-med` · branch `plan/fieldcrypto-key-rotation-01-003` · commit `6743ced` · merge `a576cd4`

### Phase 02 — Versioned Multi-Key Cipher

- [x] [001-multi-key-cipher-core.md](./phase-02-versioned-cipher/001-multi-key-cipher-core.md) — tier `opus-high` · branch `plan/fieldcrypto-key-rotation-02-001` · commit `0e0d058` · merge `81ba702`
- [x] [002-facade-key-store-adapter.md](./phase-02-versioned-cipher/002-facade-key-store-adapter.md) — tier `sonnet-high` · branch `plan/fieldcrypto-key-rotation-02-002` · commit `a467939` · merge `b0d616e`

### Phase 03 — Re-Encrypt-On-Read Call Sites

- [x] [001-rotating-cipher-helper.md](./phase-03-rotate-on-read/001-rotating-cipher-helper.md) — tier `opus-med` · branch `plan/fieldcrypto-key-rotation-03-001` · commit `971c8e2` · merge `beb2d72`
- [x] [002-wire-decrypt-call-sites.md](./phase-03-rotate-on-read/002-wire-decrypt-call-sites.md) — tier `sonnet-high` · branch `plan/fieldcrypto-key-rotation-03-002` · commit `4fc2e58` · merge `c05c2bb`
- [x] [003-rotate-on-read-integration-test.md](./phase-03-rotate-on-read/003-rotate-on-read-integration-test.md) — tier `sonnet-high` · branch `plan/fieldcrypto-key-rotation-03-003` · commit `4b16f27` · merge `f04b1b7`

### Phase 04 — Admin Key-Rotation API

- [x] [001-field-crypto-key-handler.md](./phase-04-admin-rotation-api/001-field-crypto-key-handler.md) — tier `opus-med` · branch `plan/fieldcrypto-key-rotation-04-001` · commit `02d8505` · merge `49ce33b`
- [x] [002-manifest-and-openapi-routes.md](./phase-04-admin-rotation-api/002-manifest-and-openapi-routes.md) — tier `sonnet-med` · branch `plan/fieldcrypto-key-rotation-04-002` · commit `1343cca` · merge `bf8843b`
- [x] [003-rotation-endpoint-integration-test.md](./phase-04-admin-rotation-api/003-rotation-endpoint-integration-test.md) — tier `sonnet-high` · branch `plan/fieldcrypto-key-rotation-04-003` · commit `eb0480d` · merge `38301a9`

### Phase 05 — Documentation Updates

- [x] [001-update-architecture-docs.md](./phase-05-doc-updates/001-update-architecture-docs.md) — tier `sonnet-high` · branch `plan/fieldcrypto-key-rotation-05-001` · commit `b36f200` · merge `c8a740b`
