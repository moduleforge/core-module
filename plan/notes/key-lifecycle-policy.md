# User question: key lifecycle and supply policy

This was flagged as the biggest open gap: without a defined source of truth for keys and a defined
rotation trigger, the plan could ship the rotation mechanism with no way to actually use it. Two
sub-questions were asked separately.

## Sub-question 1: source of truth for keys (active + retired)

Presented options:

> DB-backed, always (Recommended) — The replacement `field_crypto_keys` table becomes the single
> source of truth for every key version, including the active one. `CORE_FIELD_KEY_HEX` (if used at
> all) only bootstraps the first key on first boot; all subsequent rotation and multi-key state
> lives in the DB. Simplest to reason about, but means key material touches the DB even for
> operators who prefer env-only keys.

versus:

> Env-list + DB hybrid — Keep `CORE_FIELD_KEY_HEX` as the active key with a new env var for a
> comma-separated list of retired keys, alongside the existing DB-fallback mode. Keeps key material
> out of the DB for env-mode operators, but is more moving parts and two supply paths to keep
> consistent.

### Answer

DB-backed, always (Recommended)

## Sub-question 2: what triggers a rotation (creates a new active key version)

mod-core has no admin API or CLI for this today. Presented options:

> Boot-time env diff (Recommended) — On process startup, if `CORE_FIELD_KEY_HEX` is set and differs
> from the currently-active DB key, treat that as "operator supplied a new key" — activate it as a
> new version, retire the old one automatically. No new API surface needed.

> New admin API/CLI endpoint — Build an explicit operator-facing action (e.g. an admin HTTP
> endpoint or a make/CLI command) that generates or accepts a new key and activates it. More
> deliberate and auditable, but adds a new surface this plan would need to design and secure.

> Defer — build the mechanism only — Ship multi-key storage, decrypt-any-version, and
> rotate-on-read, but rotation itself is triggered by a future, unplanned mechanism. Fastest to
> ship, but the feature is inert until that follow-on exists.

### Answer

New admin API/CLI endpoint

## Interpretation for planning

Keys, including the currently-active one, live entirely in the (redesigned) `field_crypto_keys`
table — no comma-separated env-var list for retired keys. `CORE_FIELD_KEY_HEX` may still bootstrap
the very first key on first boot (existing `NewFromEnvOrGenerate` precedent), but is not the
ongoing multi-key supply mechanism.

Rotation is triggered by a new, explicit admin-facing surface (HTTP endpoint and/or CLI/make
command — the research/design pass should pick the concrete shape appropriate to mod-core's
existing `httpapi`/admin conventions, e.g. how `AppsHandler`'s admin-only routes are structured) that
generates or accepts a new key and activates it as the new current version, retiring the prior
active version. This surface also needs to support marking a key as **compromised** at rotation
time — see `rotation-api-shape.md` for how that interacts with re-encrypt-on-read failure policy.
This is new scope this plan must design and secure (authz-gated, admin-only, following mod-core's
existing authorization conventions for admin surfaces like `AppsHandler`).

## Follow-up user question: `CORE_FIELD_KEY_HEX` precedence once the table is populated

Raised by `key-store-schema-design.md`'s open question 4, once the DB became the single source of
truth for all key versions: an env key that is not a row in the table has no version number and
cannot legally en/decrypt anything, so "env always wins" (today's behavior — `NewFromEnv` reads
`CORE_FIELD_KEY_HEX` unconditionally every boot) cannot survive intact. Presented options:

> Fail loudly on mismatch (Recommended) — `CORE_FIELD_KEY_HEX` only ever bootstraps the very first
> key (version 1) on an empty table. After that, if it's set and doesn't match the active DB key,
> refuse to start rather than silently doing something with it — operators rotate only through the
> new admin endpoint. Prevents an operator believing they rotated by editing an env var when they
> didn't.

versus:

> Silently ignore after bootstrap — `CORE_FIELD_KEY_HEX` is read only at first boot on an empty
> table. Any value present afterward is simply ignored — no error, no effect.

### Answer

Fail loudly on mismatch (Recommended)

### Interpretation for planning

`CORE_FIELD_KEY_HEX` is a first-boot-only bootstrap seed (used solely for `InsertInitialFieldCryptoKey`
when the table is empty). On every subsequent boot, if the env var is set: bytes equal to the active
DB key's bytes → proceed silently (the normal steady state for an operator who bootstrapped from env
and never removed it); bytes differ from the active DB key's bytes → **fail construction loudly**,
with an error message naming the new admin rotation endpoint as the correct way to change the active
key. This is a deliberate, visible behavior change from today's "env always wins" and must be called
out prominently in both `AGENTS.md` and the app-mfmanager deploy-doc rewrite this plan already defers
as a cross-repo follow-on.
