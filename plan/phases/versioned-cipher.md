# Versioned Cipher

## Purpose and scope

Phase summary for the cipher core: the version-tagged blob format and the reloadable multi-key
`Cipher` that reads and writes it. The surface is settled in
[`rotation-api-shape.md`](../notes/rotation-api-shape.md#phase-2-surface--apiinternalfieldcrypto);
this phase implements it.

## Goals

Turn `Cipher` from a single-key value into a reloadable keyring: one active key used for every new
encryption, plus an indefinite number of retired keys usable for decryption and never selectable for
encryption. Define and implement the `version(4) || nonce(12) || ciphertext || tag(16)` on-disk
format with the version prefix bound as AEAD additional authenticated data, and export the
rotation-detection surface (`DecryptWithRotation`, `Rotation`, `BlobVersion`) that Phase 3's read
paths act on.

This phase owns the security-critical decisions in the plan — the wire format, key selection, the
policy bit (`Rotation.MustPersist`, derived once inside the cipher from the source key's
`compromised_at`), and the failure behavior when a blob names a key that is not loaded, which must
fail loudly rather than trying every key or returning empty plaintext. It comes after Phase 1
because the constructors read the schema that phase defines, and before Phase 3 because the call
sites need the rotation-aware API this phase exports.

It also carries two deliberate breaking changes: `Encrypt`, `Decrypt`, and `DecryptWithRotation` all
take a `context.Context` (any of them may trigger a key-set reload), and `NewFromEnv` is deleted
outright from both the internal package and the façade, because a cipher built from an env var alone
has no version number and cannot legally encrypt anything under the DB-as-source-of-truth model.

`fromPersistedOrGenerated` — the exact function open followup `MnvB` (optional zeroing of key byte
slices after use) concerns — is rewritten wholesale here, which makes this the natural incidental
place to address `MnvB`, without it being a requirement or an acceptance criterion.

## Inputs

- Phase 1's regenerated `model/db/` method set and its seven replacement key queries.
- The settled wire format and version-identifier decision in
  [`key-store-schema-design.md`](../notes/key-store-schema-design.md#version-identifier-integer-not-fingerprint)
  and [`key-version-wire-format.md`](../notes/key-version-wire-format.md).
- The settled `KeyRecord` / `KeyStore` / `Cipher` / `Rotation` surface and the staleness policy
  (reload-on-unknown-version, `keySetTTL` on the encrypt path, exported `Reload`) in
  [`rotation-api-shape.md`](../notes/rotation-api-shape.md).
- The settled bootstrap policy in
  [`key-lifecycle-policy.md`](../notes/key-lifecycle-policy.md): `CORE_FIELD_KEY_HEX` is a
  first-boot-only seed and fails construction loudly when it differs from the active DB key
  thereafter.
- The current `api/internal/fieldcrypto/fieldcrypto.go` implementation and its three test files, all
  of which encode the single-key contract and the untagged blob layout.

## Outputs

- A version-tagged blob format, documented in the rewritten package doc as the authoritative
  statement of the layout, replacing both the "key rotation is out of scope" paragraph and the "no
  AAD is bound to ciphertext" claim (which narrows to "no *row identity* is bound").
- A multi-key `Cipher` holding an atomically-swappable key set, encrypting only under the active key,
  decrypting under any loaded and unexpired key, and failing loudly on an unknown or expired version
  after one rate-limited reload attempt.
- `DecryptWithRotation`, the `Rotation` value type carrying `MustPersist`, `BlobVersion`, and an
  exported `Reload` — the whole surface Phases 3 and 4 consume.
- Reworked constructors: bootstrap through `KeyStore.InsertInitialKey` with the two-guard insert and
  adopt-the-winner race resolution; `NewFromKey` retained as a store-less, version-1, never-rotating
  cipher for tests; `NewFromEnv` deleted from both packages.
- An `api/fieldcrypto` façade that absorbs the `coredb` import via a `keyStoreAdapter`, keeping
  `api/internal/fieldcrypto` free of any model dependency and leaving
  `moduleforge.module.yaml`'s `cipher` service block byte-for-byte unchanged.
- Updated unit and integration tests covering multi-key round-trips, retired-key decryption,
  active-key-only encryption, unknown-version and expired-version rejection, AAD tamper detection on
  the version prefix, and `-race` safety across a reload.
- `cd api && make test` and `cd api && make lint` passing.
