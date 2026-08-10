# Implement Auto-Generate Cipher

## Purpose and scope

Add `NewFromEnvOrGenerate` to `api/internal/fieldcrypto/fieldcrypto.go` and
re-export it through the public façade `api/fieldcrypto/fieldcrypto.go`, then
wire it into `moduleforge.module.yaml`'s `cipher` service declaration so
mfgen-generated composition roots pick it up automatically. Depends on
[`001-add-field-crypto-keys-table.md`](./001-add-field-crypto-keys-table.md)
(needs the generated `GetFieldCryptoKey`/`InsertFieldCryptoKeyIfAbsent`
method shapes, though this task deliberately does not import
`core-model/db` directly — see Requirement 1).

No standard skill covers this; follow the [`## Procedure`](#procedure) below.

## Requirements

1. **`api/internal/fieldcrypto/fieldcrypto.go` — new `FieldKeyQuerier`
   interface and `NewFromEnvOrGenerate` constructor.**

   Declare a minimal interface local to this package, satisfied structurally
   by `*coredb.Queries` (`github.com/moduleforge/core-model/db`) without this
   package importing that type directly — mirrors the precedent set by
   `api/types/types.go` (which *does* take the full `coredb.Querier`), except
   scoped down to exactly the two methods this feature needs, so a test fake
   only has to implement two methods instead of the ~15 the full `Querier`
   interface requires (see `api/types/types_test.go`'s `stubQuerier` for the
   contrast this avoids):

   ```go
   // FieldKeyQuerier is the minimal persistence contract
   // NewFromEnvOrGenerate needs. Satisfied structurally by *coredb.Queries
   // (github.com/moduleforge/core-model/db) — the same object every other
   // mod-core service constructs via coredb.New(pool) — so this package
   // does not need to import core-model/db at all; callers (and the
   // moduleforge.module.yaml queries:coredb arg-source) pass it directly.
   type FieldKeyQuerier interface {
       GetFieldCryptoKey(ctx context.Context) ([]byte, error)
       InsertFieldCryptoKeyIfAbsent(ctx context.Context, keyBytes []byte) ([]byte, error)
   }
   ```

   Factor `NewFromEnv`'s existing hex-decode-and-validate logic into a shared
   private helper (e.g. `fromHexKey(hexKey string) (*Cipher, error)`) so
   `NewFromEnv` and the env-var branch of `NewFromEnvOrGenerate` run the
   *identical* code path — this is what makes "explicit env var always wins,
   behavior identical to today" provably true rather than merely
   duplicated-and-hopefully-equivalent. `NewFromEnv` itself must not change
   its signature or observable behavior in any way.

   ```go
   // NewFromEnvOrGenerate reads CORE_FIELD_KEY_HEX if set — identical
   // behavior to NewFromEnv in every respect, including on invalid values.
   // Only when the env var is entirely unset does it fall back to q: fetch
   // the persisted key if one exists, or generate and durably persist a new
   // one. Concurrent first-boot callers are safe: the persistence layer
   // guarantees (via a DB-level uniqueness constraint, not a read-then-write
   // race) that every caller converges on the same key. A persisted key
   // that fails validation (wrong length) is a fail-loudly error, never
   // silently regenerated — regenerating over an existing key would make
   // data already encrypted under the old key unrecoverable.
   func NewFromEnvOrGenerate(ctx context.Context, q FieldKeyQuerier) (*Cipher, error) {
       if hexKey, ok := os.LookupEnv(envKeyName); ok {
           return fromHexKey(hexKey)
       }
       return fromPersistedOrGenerated(ctx, q)
   }

   func fromPersistedOrGenerated(ctx context.Context, q FieldKeyQuerier) (*Cipher, error) {
       key, err := q.GetFieldCryptoKey(ctx)
       switch {
       case err == nil:
           // NewFromKey fails loudly (wrong length) rather than regenerating.
           return NewFromKey(key)
       case errors.Is(err, pgx.ErrNoRows):
           // fall through to generate-and-persist
       default:
           return nil, fmt.Errorf("fieldcrypto: read persisted key: %w", err)
       }

       candidate := make([]byte, keySize)
       if _, rerr := rand.Read(candidate); rerr != nil {
           return nil, fmt.Errorf("fieldcrypto: generate key: %w", rerr)
       }

       inserted, err := q.InsertFieldCryptoKeyIfAbsent(ctx, candidate)
       switch {
       case err == nil:
           // We won the race; the DB echoes back exactly what we inserted.
           return NewFromKey(inserted)
       case errors.Is(err, pgx.ErrNoRows):
           // ON CONFLICT DO NOTHING skipped our row: another caller won the
           // race between our SELECT and our INSERT. Adopt their key.
           winner, rerr := q.GetFieldCryptoKey(ctx)
           if rerr != nil {
               return nil, fmt.Errorf("fieldcrypto: re-fetch key after lost race: %w", rerr)
           }
           return NewFromKey(winner)
       default:
           return nil, fmt.Errorf("fieldcrypto: persist generated key: %w", err)
       }
   }
   ```

   This introduces a new import, `"github.com/jackc/pgx/v5"` (for
   `pgx.ErrNoRows`), into `api/internal/fieldcrypto` — previously
   dependency-free beyond stdlib `crypto/*`. `pgx/v5` is already an
   `api/go.mod` dependency (used elsewhere in this module, e.g.
   `api/txhelper`), so this is a new intra-module import, not a new module
   dependency. Update the package doc comment's "Key management" section to
   describe the new auto-generate-and-persist path alongside the existing
   description.

2. **`api/fieldcrypto/fieldcrypto.go` — façade re-export.** Add the exported
   `FieldKeyQuerier` type alias and `NewFromEnvOrGenerate` wrapper, mirroring
   how `Cipher`, `NewFromEnv`, and `NewFromKey` are already re-exported:

   ```go
   type FieldKeyQuerier = fieldcrypto.FieldKeyQuerier

   func NewFromEnvOrGenerate(ctx context.Context, q FieldKeyQuerier) (*Cipher, error) {
       return fieldcrypto.NewFromEnvOrGenerate(ctx, q)
   }
   ```

3. **`moduleforge.module.yaml` — wire the new constructor.** Update the
   `cipher` service entry under `provides.services`:

   ```yaml
   # cipher loads the AES-256-GCM field encryption key from
   # CORE_FIELD_KEY_HEX, or — when unset — fetches-or-generates a durable
   # key from the field_crypto_keys table (added in migration 0017; see
   # api/internal/fieldcrypto.NewFromEnvOrGenerate). CORE_FIELD_KEY_HEX set
   # explicitly always wins, identical to the prior NewFromEnv-only
   # behavior.
   - name: cipher
     type: "*fieldcrypto.Cipher"
     constructor: fieldcrypto.NewFromEnvOrGenerate
     returnsError: true
     args:
       - context
       - queries:coredb
   ```

   This matches the existing `typeResolver` service entry's
   `args: [context, queries:coredb]` pattern exactly (same arg-source forms,
   same resulting `coredb.New(pool)` expression) — no new arg-source kind is
   needed. Every ModuleForge app that regenerates its composition root
   against an updated `mod-core` version pin picks up this constructor
   automatically; this is a deliberate design choice with a cross-app
   consequence flagged in `plan/overview.md`'s "Deferred and flagged"
   section — no action needed from this task beyond making the change as
   specified.

## Validation

- `cd api && go build ./...` succeeds.
- `cd api && make lint` (go vet + gofmt check) passes.
- `cd api && go test ./internal/fieldcrypto/... ./fieldcrypto/...` passes —
  the *existing* `TestNewFromEnv_Errors` subtests and every other existing
  test in `api/internal/fieldcrypto/fieldcrypto_test.go` continue to pass
  unmodified, proving `NewFromEnv`'s behavior is unchanged. (New tests for
  `NewFromEnvOrGenerate` itself are
  [`003-test-race-and-corruption-paths.md`](./003-test-race-and-corruption-paths.md)'s
  job, not this task's — this task only needs the existing suite to stay
  green plus the package to build and typecheck with the new code present.)
- `grep -n "NewFromEnvOrGenerate" api/fieldcrypto/fieldcrypto.go api/internal/fieldcrypto/fieldcrypto.go moduleforge.module.yaml` shows the new symbol wired through all three files consistently.
- Manual read-through confirms `fromHexKey` (or equivalently-named shared
  helper) is called by both `NewFromEnv` and `NewFromEnvOrGenerate`'s
  env-var branch — no duplicated decode/validate logic between them.

## Metadata

architectural_impact: true

## Assumptions

- The manifest change (Requirement 3) is applied unconditionally — every
  composing app gets the new constructor on its next regen against an
  updated pin, with no per-app manifest-level opt-out beyond continuing to
  set `CORE_FIELD_KEY_HEX` explicitly. This default-behavior-swap question
  is flagged for the user/module owner in `plan/overview.md`; this task
  proceeds with the unconditional swap as specified above regardless, since
  it does not block correctly implementing the underlying capability.
- `mfgen` (the composition-root code generator) and every composing app's
  own `main.go` are out of this task's scope; this task's job ends at
  `mod-core`'s own manifest declaration.

## References

- `api/internal/fieldcrypto/fieldcrypto.go` — the file this task modifies;
  read it in full first (already read during planning; `NewFromEnv`,
  `NewFromKey`, `envKeyName`, `keySize` are all defined here and must not
  change).
- `api/fieldcrypto/fieldcrypto.go` — the façade this task modifies.
- `api/types/types.go` and `api/types/types_test.go` — precedent for the
  `ctx, q coredb.Querier`-shaped constructor and its test-fake pattern (this
  task's `FieldKeyQuerier` deliberately narrows that pattern to 2 methods).
- `moduleforge.module.yaml` — the `cipher` and `typeResolver` service
  entries under `provides.services`; the `typeResolver` entry
  (`args: [context, queries:coredb]`) is the exact pattern this task's
  `cipher` update follows.
- `docs/mf-standards/manifest-spec.md` §4 (Arg-source vocabulary) —
  confirms `queries:<import-alias>` resolves to `<import-alias>.New(pool)`;
  read-only reference (this file lives in the `docs/mf-standards` git
  submodule, out of this plan's scope to edit).
- `plan/notes/../overview.md`'s "Key research finding" section — the
  confirmed real-`main.go` call ordering (`pool` → migrations → cipher init)
  that justifies not needing a separate ad hoc DB connection here.

## Procedure

1. Implement Requirement 1 in `api/internal/fieldcrypto/fieldcrypto.go`.
2. Implement Requirement 2 in `api/fieldcrypto/fieldcrypto.go`.
3. Implement Requirement 3 in `moduleforge.module.yaml`.
4. Run the Validation commands; fix and re-run until green.
5. Commit all three files' changes together as this task's change.

## Checkpoint hints

- After Requirement 1 (internal implementation) compiles and the existing
  `internal/fieldcrypto` test suite still passes.
- After Requirement 2 (façade re-export) compiles.
- After Requirement 3 (manifest update), before running full Validation.

## Status

Implementation outcome: **validation failed** (blocked on an out-of-scope,
pre-existing defect — see below). Date: 2026-08-10.

- Implemented all three Requirements exactly as specified:
  - `api/internal/fieldcrypto/fieldcrypto.go` — factored `NewFromEnv`'s
    decode/validate logic into `fromHexKey`; added the `FieldKeyQuerier`
    interface, `NewFromEnvOrGenerate`, and `fromPersistedOrGenerated`
    verbatim per the task doc's code blocks; updated the package doc
    comment's "Key management" discussion to describe the new
    auto-generate-and-persist path.
  - `api/fieldcrypto/fieldcrypto.go` — added the `FieldKeyQuerier` alias and
    `NewFromEnvOrGenerate` wrapper.
  - `moduleforge.module.yaml` — updated the `cipher` service entry's
    `constructor` to `fieldcrypto.NewFromEnvOrGenerate` with
    `args: [context, queries:coredb]`, matching `typeResolver`'s pattern,
    plus the doc comment naming migration `0017` (confirmed present at
    `model/migrations/0017_field_crypto_keys.sql`).
- Validation:
  - `cd api && go build ./...` — **passed**.
  - `cd api && make lint` (go vet + gofmt) — **failed**, but not because of
    anything this task changed. `gofmt -l .` is clean. `go vet ./...`
    fails in five unrelated packages (`service`, `display`, `entity`,
    `types`, `httpapi`) whose test-only fake `Querier` implementations
    (`mockQuerier`, `stubQuerier`, `resolverStubQuerier`, `appsFakeQuerier`)
    don't implement the two methods
    (`GetFieldCryptoKey`/`InsertFieldCryptoKeyIfAbsent`) that task
    `001-add-field-crypto-keys-table.md` added to `db.Querier`. **Verified
    pre-existing**: `git stash`-ing every edit this task made and re-running
    `make lint` reproduces the identical failure list, confirming task 001
    left this gap and this task's diff neither causes nor worsens it.
    Scoped checks confirm this task's own code is clean:
    `go vet ./internal/fieldcrypto/... ./fieldcrypto/...` passes with no
    output.
  - `cd api && go test ./internal/fieldcrypto/... ./fieldcrypto/...` —
    **passed** (`ok  .../internal/fieldcrypto`; `fieldcrypto` façade package
    has no test files, as before). Every pre-existing subtest of
    `TestNewFromEnv_Errors` still passes unmodified.
  - `grep -n "NewFromEnvOrGenerate" api/fieldcrypto/fieldcrypto.go
    api/internal/fieldcrypto/fieldcrypto.go moduleforge.module.yaml` —
    **passed**; the symbol appears in all three files.
  - Manual read-through — **confirmed**: both `NewFromEnv` and
    `NewFromEnvOrGenerate`'s env-var branch call `fromHexKey`; no
    duplicated decode/validate logic.
- Assumptions from `## Assumptions` held as stated; no deviation.
- **Flagged for the manager**: five test-fake `Querier` implementations
  across `service/mock_test.go`, `display/registry_test.go`,
  `entity/resolver_test.go`, `types/types_test.go`, and `httpapi/apps_test.go`
  are missing `GetFieldCryptoKey`/`InsertFieldCryptoKeyIfAbsent`, breaking
  `cd api && go vet ./...` (and therefore `make lint`) module-wide. This
  predates this task (confirmed via `git stash`) and is outside this task's
  Requirements — fixing it means editing five files across five unrelated
  packages, past both the same-diff and single-file drift carve-outs this
  procedure allows a task agent to self-fix. Recommend a small follow-up
  task (chained to or amending
  [`001-add-field-crypto-keys-table.md`](./001-add-field-crypto-keys-table.md))
  that adds the two stub methods (mirroring the existing method style in
  each fake) to all five fakes.
