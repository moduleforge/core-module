# Versioned Cipher

## Purpose and scope

Phase summary for the cipher core: the version-tagged blob format and the multi-key `Cipher` that
reads and writes it. Task breakdown deferred until the
[open decisions](../overview.md#open-decisions) are resolved.

## Goals

Turn `Cipher` from a single-key value into a keyring: one active key used for every new encryption,
plus an indefinite number of retired keys usable for decryption and never selectable for
encryption. Define and implement the new on-disk format so each blob identifies the key that
produced it, and expose the version comparison that Phase 3's read paths act on.

This phase owns the security-critical decisions in the plan — the wire format, the key-selection
logic, and the failure behavior when a blob names a key that is not loaded (which must fail loudly,
never fall back to trying every key silently or returning empty plaintext). It comes after Phase 1
because the constructors read from the schema that phase defines, and before Phase 3 because the
call sites need the rotation-aware API this phase exports.

`fromPersistedOrGenerated` — the exact function open followup `MnvB` (optional zeroing of key byte
slices after use) concerns — is rewritten wholesale here. The task that rewrites it should note
`MnvB` as a natural incidental place to address, without treating it as a requirement or an
acceptance criterion.

## Inputs

- Phase 1's regenerated `model/db/` method set and its replacement key queries.
- Open decision 1 (wire format) — the tag's location, identifier type, width, and AAD binding.
- Open decision 2 (key lifecycle) — how retired keys reach the process in `CORE_FIELD_KEY_HEX`
  mode, and what makes a new key active.
- The current `api/internal/fieldcrypto/fieldcrypto.go` implementation and its three test files,
  all of which encode the single-key contract and the untagged blob layout.
- The existing constructor precedence rule (env var always wins when set), which stays.

## Outputs

- A version-tagged blob format, documented in the package doc as the authoritative statement of the
  layout.
- A multi-key `Cipher` encrypting only under the active key and decrypting under any loaded key,
  with a loud failure for an unknown version.
- Reworked constructors and a reworked `FieldKeyQuerier` contract, structurally satisfied by
  `*coredb.Queries` without importing `core-model/db`, as today.
- The rotation-detection surface Phase 3 consumes, in whichever shape open decision 3 selects.
- A rewritten package doc replacing the "Key rotation is out of scope for this package" paragraph,
  which is the sentence app-mfmanager's deploy docs quote verbatim as justification for calling the
  key unrotatable.
- Updated unit and integration tests covering multi-key round-trips, retired-key decryption,
  active-key-only encryption, and unknown-version rejection.
- `cd api && make test` and `cd api && make lint` passing.
