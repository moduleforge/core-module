# Rotate On Read

## Purpose and scope

Phase summary for wiring transparent re-encryption into mod-core's read paths and exposing the new
cipher surface to callers. Task breakdown deferred until the
[open decisions](../overview.md#open-decisions) are resolved.

## Goals

Deliver the user-visible half of the feature: reading a value stored under a retired key leaves it
re-encrypted under the active key. This is the behavior fieldcrypto's package doc has promised at
the call site since it was written, with no call site ever implementing it.

There is no single choke point to wire it through — `Decrypt` is a pure function with no database
handle and no knowledge of the row it is reading, and mod-core has six decrypt sites across four
files in `api/service/` spanning two encrypted columns. The inventory and the structural obstacles
are in [`../notes/rotation-on-read-call-sites.md`](../notes/rotation-on-read-call-sites.md); this
phase's real work is applying the chosen wiring uniformly so no read path is silently left
un-rotating.

This phase also carries the module's outward-facing surface: the `api/fieldcrypto/` façade
re-exports and the `moduleforge.module.yaml` `cipher` service block, which is the point at which
every composing app inherits the change.

## Inputs

- Phase 2's rotation-aware cipher API and Phase 1's narrow blob-only update queries.
- Open decision 3 (API shape and failure policy) — which wiring option, and what a read does when
  the write-back fails or the caller has no writable transaction.
- The six decrypt sites: `profile.go`'s `ResolveProfileByEntityID` (both branches),
  `legal_entity.go`'s `GetTaxID` (both branches), `natural_person.go`'s `GetDecryptedSSN`, and
  `corporation.go`'s `GetDecryptedEIN`.
- The current façade `api/fieldcrypto/fieldcrypto.go` and the manifest's `cipher` service block.

## Outputs

- Every mod-core read path that decrypts a field value re-encrypts it under the active key when its
  version is stale, with a single consistent failure policy across all six sites.
- `api/fieldcrypto/` re-exporting exactly the surface outside callers need and nothing more,
  preserving the existing internal/façade split.
- `moduleforge.module.yaml`'s `cipher` service block matching the constructor's final signature.
- Service-layer tests proving a stale-version read both returns correct plaintext and persists the
  re-encrypted blob, and that a second read finds it already current.
- `cd api && make test`, `cd api && make lint`, and a whole-module `make build` passing.
