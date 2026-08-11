# Fieldcrypto Current State

## Purpose and scope

Baseline survey of how `fieldcrypto` handles keys, ciphertext, and persistence today, gathered
during initial planning for the key-rotation feature. Records what exists so later tasks do not
re-derive it. Nothing here is a design proposal.

## The cipher package

`api/internal/fieldcrypto/fieldcrypto.go` (201 lines) is the whole implementation.

- `Cipher` wraps a single `cipher.AEAD` (AES-256-GCM) built from exactly one 32-byte key. No key
  identity, version, or fingerprint is retained — once `NewFromKey` builds the AEAD the key bytes
  are unreachable from the `Cipher` value.
- `Encrypt(plaintext string) ([]byte, error)` returns `nonce(12) || ciphertext || tag(16)`. Empty
  plaintext returns `[]byte{}` (stored as NULL by the DB layer). No AAD is bound.
- `Decrypt(blob []byte) (string, error)` reverses it; empty/nil blob returns `("", nil)`; a blob
  shorter than `nonceSize + 16` is rejected before `Open`.
- Constructors: `NewFromEnv` (reads `CORE_FIELD_KEY_HEX`, 64 hex chars), `NewFromKey` (raw 32
  bytes), `NewFromEnvOrGenerate(ctx, q FieldKeyQuerier)` (env var always wins; only when entirely
  unset does it fall back to the DB).
- `FieldKeyQuerier` is a two-method structural interface satisfied by `*coredb.Queries`:
  `GetFieldCryptoKey(ctx) ([]byte, error)` and `InsertFieldCryptoKeyIfAbsent(ctx, []byte) ([]byte,
  error)`. It deliberately avoids importing `core-model/db`.
- The package doc already promises the rotation behavior this plan implements: "if rotation is
  required it must be handled at the call site (e.g. re-encrypt on read when the active key differs
  from the key that produced the stored blob)." No such call site exists.

The public façade `api/fieldcrypto/fieldcrypto.go` is a thin 35-line re-export: type aliases
`Cipher` and `FieldKeyQuerier`, plus `NewFromEnv`, `NewFromKey`, `NewFromEnvOrGenerate`. Every
symbol added to the internal package that outside callers need must be re-exported here.

## Key persistence

`model/migrations/0017_field_crypto_keys.sql` creates a deliberately single-row table:

```sql
CREATE TABLE field_crypto_keys (
  id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  key_bytes  BYTEA NOT NULL CHECK (octet_length(key_bytes) = 32),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

The `CHECK (id = 1)` plus `PRIMARY KEY` is what makes concurrent first-boot inserts converge: the
`ON CONFLICT (id) DO NOTHING` in `model/queries/field_crypto_keys.sql`'s
`InsertFieldCryptoKeyIfAbsent` returns no rows to the loser, who then re-fetches the winner's key.
Any replacement schema that admits multiple key rows must re-establish an equivalent
converge-on-one-winner guarantee for first boot, since the `id = 1` singleton is doing that job
today. `model/db/` is sqlc-generated from `model/queries/`; regenerate with `cd model && make gen`.

Migration numbering is module-local (`goose_db_version_core`), so a new migration simply takes the
next free number after `0017`. `0099_access_function_stubs.sql` sits above the numbered block.

## Module wiring

`moduleforge.module.yaml`'s `provides.services` declares:

```yaml
    - name: cipher
      type: "*fieldcrypto.Cipher"
      constructor: fieldcrypto.NewFromEnvOrGenerate
      returnsError: true
      args:
        - context
        - queries:coredb
```

mfgen renders this into each composing app's generated `cmd/server/main.go`. Any change to the
constructor's name, signature, or arg sources is a manifest change that every composing app picks
up when it regenerates against a new mod-core pin.

## Existing tests

- `fieldcrypto_test.go` — round-trip, nonce uniqueness, empty-input handling, tamper detection,
  `NewFromEnv` env-var validation, a `-race` concurrency exercise.
- `generate_test.go` — fake-`FieldKeyQuerier` unit tests for every `NewFromEnvOrGenerate` branch
  including the lost-race path.
- `generate_integration_test.go` — build-tag `integration`, real Postgres, asserts the
  `ON CONFLICT DO NOTHING` convergence across concurrent goroutines.

All three will need rework: the round-trip tests encode the current untagged blob layout, and the
generate tests encode the two-method querier contract.

## Consumers outside mod-core

None of these are in scope for this plan; they are recorded so the cross-repo follow-ons in
`plan/overview.md` are accurate.

- `moduleforge/app-mftodo` and `moduleforge/app-mfmanager` — compose mod-core; each has a generated
  `cmd/server/main.go` carrying the `cipher` service construction, plus deploy docs naming
  `CORE_FIELD_KEY_HEX`.
- `moduleforge/mod-users` — `.env.example:145` ships a sample `CORE_FIELD_KEY_HEX`. No Go call
  sites found.
- `docs/mf-standards/` is a **git submodule** (`git@github.com:moduleforge/docs-mf-standards.git`),
  not mod-core content. Its `manifest-spec.md` and `architecture/secret-durability-design.md`
  describe the current single-key design and will go stale.
