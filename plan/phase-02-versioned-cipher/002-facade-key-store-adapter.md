# Facade Key Store Adapter

## Purpose and scope

Give `api/fieldcrypto` — the public façade the module manifest names — its real shape: a
`coredb`-typed `FieldKeyQuerier` interface and a small adapter mapping sqlc rows onto the internal
package's `KeyRecord`. This is what absorbs the `core-model/db` dependency at the façade boundary so
`api/internal/fieldcrypto` stays model-free, and — critically — what keeps
`moduleforge.module.yaml`'s `cipher` service block byte-for-byte unchanged.

Files this task owns:

- `api/fieldcrypto/fieldcrypto.go` — replacing the placeholder task 001 left.
- A new `api/fieldcrypto/fieldcrypto_test.go` (or equivalent) for the adapter.

Depends on [`001-multi-key-cipher-core.md`](./001-multi-key-cipher-core.md). No standard skill covers
this work.

## Requirements

1. **Declare the façade's `FieldKeyQuerier`** over `coredb` types, exactly as
   [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-2-surface--the-apifieldcrypto-façade)
   gives it:

   ```go
   type FieldKeyQuerier interface {
       ListUsableFieldCryptoKeys(ctx context.Context) ([]coredb.FieldCryptoKey, error)
       InsertInitialFieldCryptoKey(ctx context.Context, keyBytes []byte) (coredb.FieldCryptoKey, error)
   }
   ```

   It must be satisfied structurally by both `*coredb.Queries` and `coredb.Querier`, so the
   manifest's `queries:coredb` arg source still type-checks unchanged. Verify the exact generated
   method signatures against `model/db/` rather than assuming them — in particular whether the insert
   takes a bare `[]byte` or a generated params struct — and match reality.
2. **Write the unexported `keyStoreAdapter`** implementing the internal package's `KeyStore` over a
   `FieldKeyQuerier`, mapping `coredb.FieldCryptoKey` → `fieldcrypto.KeyRecord`. Roughly 30 lines.
   The `int32` → `uint32` version conversion **must guard against a negative value** and return an
   explicit "corrupt key row" error rather than wrapping around silently.
3. **`NewFromEnvOrGenerate(ctx context.Context, q FieldKeyQuerier) (*Cipher, error)`** wraps the
   querier in the adapter and delegates to the internal constructor. Its name, parameter list, and
   error return must match the manifest's `cipher` block exactly (`constructor:
   fieldcrypto.NewFromEnvOrGenerate`, `returnsError: true`, `args: [context, queries:coredb]`).
4. **Re-export exactly what outside callers need, and nothing more.** At minimum: the `Cipher` type
   alias, `Rotation`, `KeyRecord`, `KeyStore`, `BlobVersion`, `NewFromKey`, and
   `NewFromEnvOrGenerate`. `NewFromEnv` must not exist. Keep the façade a thin re-export layer: no
   crypto logic, no policy, no key handling beyond the adapter mapping.
5. **Do not change `moduleforge.module.yaml`.** Its `cipher` service block is unchanged by this plan;
   confirm that as a validation step. (Phase 4 adds a *separate* handler service block; it does not
   touch this one.)
6. **Tests.** Cover the adapter mapping directly: every nullable timestamp maps through correctly, a
   negative version is rejected, and a `coredb.Querier`-shaped fake satisfies `FieldKeyQuerier`.
   Include a compile-time assertion in the test file that `coredb.Querier` satisfies
   `fieldcrypto.FieldKeyQuerier` — that single line is what will catch a future query-signature
   change breaking the manifest contract.

## Validation

- `cd api && make build`, `cd api && make test`, and `cd api && make lint` all pass.
- The test file contains a `var _ fieldcrypto.FieldKeyQuerier = (coredb.Querier)(nil)`-style
  compile-time assertion (adapted to the actual package aliases) and it compiles.
- `grep -rn "core-model/db\|coredb" api/internal/fieldcrypto/` still returns nothing.
- `grep -rn "func NewFromEnv(" api/` returns nothing.
- `git diff moduleforge.module.yaml` is empty.
- The façade file stays small — a re-export layer plus the adapter, not a second implementation.
- A negative `Version` in a `coredb.FieldCryptoKey` produces an error from the adapter (assert in a
  test).

## Metadata

architectural_impact: true

## Assumptions

- Task 001 has landed and left `api/fieldcrypto` compiling against a placeholder signature that this
  task replaces. A small amount of rework in that file is expected and intended.
- `api/go.mod` already requires `core-model`, so importing `coredb` in the façade adds no new module
  dependency.
- Nothing outside mod-core calls `Cipher.Encrypt` or `Cipher.Decrypt` directly — composing apps
  construct a cipher and hand it to `coreservice.New` — so the added `context.Context` parameter is a
  mod-core-internal change and needs no façade shim.

## References

- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-2-surface--the-apifieldcrypto-façade)
  — the façade interface, the adapter, and why the manifest stays unchanged.
- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#consequences-beyond-the-table)
  — the `KeyRecord`/`KeyStore` split and the rejected alternative of importing `coredb` into the
  internal package.
- [`../notes/fieldcrypto-current-state.md`](../notes/fieldcrypto-current-state.md) — the current
  35-line façade and the manifest block it must keep satisfying.

## Status

**succeeded** — 2026-08-13.

### Validation

| Check | Result |
| --- | --- |
| `cd api && make build` | passed |
| `cd api && make test` | passed (all 13 packages, including new `fieldcrypto` façade tests) |
| `cd api && make lint` | passed (`go vet ./...`, `gofmt -l .` empty) |
| `cd api && go test -race ./fieldcrypto/...` | passed |
| `var _ FieldKeyQuerier = (coredb.Querier)(nil)` compiles | confirmed in `api/fieldcrypto/fieldcrypto_test.go` |
| `grep -rn "core-model/db\|coredb" api/internal/fieldcrypto/` | no matches |
| `grep -rn "func NewFromEnv(" api/` | no matches |
| `git diff moduleforge.module.yaml` | empty |
| Negative `Version` rejected by the adapter | asserted by `TestKeyRecordFromRowRejectsNegativeVersion` and `TestNewFromEnvOrGenerateRejectsCorruptRow` |
| Inline security review (`review_focus: security`) | no findings — the diff is a thin mapping/re-export layer with no crypto logic of its own; key-material freshness (no adapter-level caching/aliasing of `KeyBytes` across calls) verified against the `KeyStore` zeroing contract from task 001 |

### Affected source files

- `api/fieldcrypto/fieldcrypto.go` — rewritten: `FieldKeyQuerier` interface over `coredb` types,
  unexported `keyStoreAdapter` mapping `coredb.FieldCryptoKey` → `fieldcrypto.KeyRecord` (with the
  negative-version guard), `NewFromEnvOrGenerate(ctx, FieldKeyQuerier)`, and re-exports of `Cipher`,
  `KeyRecord`, `KeyStore`, `Rotation`, `BlobVersion`, `NewFromKey`.
- `api/fieldcrypto/fieldcrypto_test.go` — new: adapter mapping tests (nullable timestamps, negative
  version rejection), the `coredb.Querier` ⇒ `FieldKeyQuerier` compile-time assertion, and
  `NewFromEnvOrGenerate` wiring tests against a `FieldKeyQuerier` fake.

### Notes

- `moduleforge.module.yaml`'s `cipher` service block is untouched, confirmed by an empty `git diff`.
- No callers outside this façade invoke `fieldcrypto.NewFromEnvOrGenerate` today (grepped across
  `api/` and `model/`), so the constructor's signature change (from a `KeyStore` parameter to
  `FieldKeyQuerier`) has no ripple effect inside this repo.
