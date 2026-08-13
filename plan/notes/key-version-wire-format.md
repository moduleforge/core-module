# User question: key-version tag wire format

Asked because "each stored ciphertext blob must carry a key-version tag alongside it" admitted two
readings (in-blob prefix vs. sibling column), and the choice cascades into the key-table schema,
every encrypted column's DDL, and the `Encrypt`/`Decrypt` signatures.

The question was presented with a recommended option and sub-decisions bundled into its
description:

> In-blob prefix (Recommended) — Encode as `version || nonce || ciphertext || tag` inside the
> single stored bytes. Self-describing, needs no per-column schema change, works unchanged for any
> future encrypted column in any module. Version bound into AEAD additional authenticated data so
> it can't be swapped undetected.

versus:

> Sibling column per encrypted field — Add a separate DB column next to each encrypted field to
> hold its key version. More explicit in the schema, but every current and future encrypted column
> needs its own version column.

## Answer

In-blob prefix (Recommended)

## Interpretation for planning

The user selected the recommended option as presented, which bundles these sub-decisions (not
separately re-confirmed, but part of the accepted recommendation):

- Wire format: `version || nonce || ciphertext || tag`, all in one stored byte sequence per
  encrypted field — no new sibling DB column per encrypted field.
- The key version is bound into the AEAD as additional authenticated data (AAD), so the version tag
  itself is tamper-evident (an attacker cannot swap the version prefix on a blob without the
  AEAD authentication failing).
- Still open for the schema/migration research pass: whether the version value itself is a
  monotonically increasing integer (tied to the new key table's own version numbering) or a
  fingerprint derived from key material. A monotone integer sourced from the key table is the
  natural fit given the key table is the source of truth (see `key-lifecycle-policy.md`) — the
  research pass should confirm and size it (byte width) rather than treat it as still undecided.
