# Key-Store Schema Design

## Purpose and scope

Resolves open decision 4 of [`overview.md`](../overview.md): the concrete replacement DDL for
`field_crypto_keys`, how first-boot convergence survives the loss of the `id = 1` singleton, whether
migration `0017` is edited or superseded, and the sqlc query surface Phase 2 and the admin rotation
endpoint call. It also settles the one point
[`key-version-wire-format.md`](./key-version-wire-format.md) left open — integer version vs.
fingerprint — because the key table owns that value.

Written to be lifted into Phase 1 task requirements. Everything under
[Decisions](#decisions-at-a-glance) through [sqlc query surface](#sqlc-query-surface) is
implementable as written; [Open questions](#open-questions) lists what is genuinely unresolved and
must not be guessed at by a task author.

Inputs treated as settled and not re-litigated: in-blob version prefix bound as AAD
([`key-version-wire-format.md`](./key-version-wire-format.md)), DB-as-single-source-of-truth for all
key versions ([`key-lifecycle-policy.md`](./key-lifecycle-policy.md)), per-key failure policy with
standard/compromised modes ([`rotation-api-shape.md`](./rotation-api-shape.md)), and the clean-break
ground rule (no backfill, no dual-format decode, existing databases regenerated from scratch).

## Decisions at a glance

| Question | Decision |
| --- | --- |
| Version identifier | Monotone integer assigned by the key table, not a key fingerprint |
| Version column type | `INTEGER GENERATED ALWAYS AS IDENTITY` (sqlc → `int32`) |
| Wire prefix width | 4 bytes, big-endian `uint32`, identical bytes used as AEAD AAD |
| "Active" representation | `retired_at IS NULL`; no separate status column |
| One-active invariant | Partial unique index on `(retired_at IS NULL) WHERE retired_at IS NULL` |
| First-boot convergence | That same index + `ON CONFLICT DO NOTHING`, unchanged in shape from today |
| Grace period | Absolute `decryptable_until TIMESTAMPTZ`, resolved from operator-supplied days by the DB clock at rotation time; `NULL` = no expiry |
| Compromised flag | `compromised_at TIMESTAMPTZ` (`NULL` = not compromised), settable after the fact |
| Table count | One table; no separate "current version pointer" table |
| Migration | Edit `0017_field_crypto_keys.sql` in place; no `0018` |
| New sqlc queries | Six, replacing the two that exist today |

## Replacement DDL

This is the full replacement body of `model/migrations/0017_field_crypto_keys.sql`. Comment density
matches the file it replaces and the surrounding migrations, which are read as design documentation.

```sql
-- +goose Up

-- field_crypto_keys holds every AES-256-GCM field-encryption key this
-- database has ever used: exactly one active key (retired_at IS NULL),
-- which all new encryption uses, plus an indefinite number of retired,
-- decrypt-only keys kept so blobs written under them stay readable until
-- re-encrypt-on-read has moved them onto the active key.
--
-- version is the value carried in each stored blob's version prefix
-- (version || nonce || ciphertext || tag) and bound into the AEAD as
-- additional authenticated data, so a blob names the exact row that can
-- decrypt it. GENERATED ALWAYS AS IDENTITY because no application code may
-- ever choose a version number: reusing one would silently make ciphertext
-- undecryptable. Gaps are expected and harmless — a lost first-boot race
-- consumes a value without inserting a row.
--
-- Exactly one active key is a database-level invariant, enforced by the
-- field_crypto_keys_one_active partial unique index below. That index also
-- replaces the id = 1 CHECK the previous single-row shape of this table
-- used to converge concurrent first-boot INSERTs on one winner: the
-- bootstrap INSERT is always an active-key INSERT, so concurrent
-- bootstraps race on that index exactly as they used to race on the
-- primary key, and the loser adopts the winner's key rather than
-- generating one of its own.
--
-- Deliberately not modeled as an entity (no FK to entities.id): this is
-- bootstrap/operational data unrelated to the domain entity hierarchy, the
-- same reasoning that keeps goose_db_version_core a bare table. It
-- likewise gets no accessible_*_ids_for_actor access function
-- (0099_access_function_stubs.sql) — there is no actor-scoped read path
-- into key material.
CREATE TABLE field_crypto_keys (
  -- Wire version tag. Starts at 1; 0 is never issued, so application code
  -- may treat a 0 version prefix as a malformed blob.
  version           INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  -- 32 bytes = AES-256. UNIQUE so an operator cannot re-introduce key
  -- material already on file — notably a key previously retired as
  -- compromised — under a fresh version number.
  key_bytes         BYTEA NOT NULL UNIQUE CHECK (octet_length(key_bytes) = 32),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- NULL = this is the active key, the one all new encryption uses. Set
  -- when a rotation demotes it to decrypt-only. Never cleared: a retired
  -- key is never re-activated.
  retired_at        TIMESTAMPTZ,
  -- End of the decrypt grace window for a retired key, resolved to an
  -- absolute instant from the operator-supplied grace period in days,
  -- against the database clock, at the moment of retirement. NULL = no
  -- expiry, and NULL is the default: a retired key stays usable for
  -- decryption indefinitely unless an operator deliberately sets a
  -- deadline. Once this instant passes the key is no longer loaded by the
  -- cipher and any blob still carrying its version fails loudly rather
  -- than decrypting.
  decryptable_until TIMESTAMPTZ,
  -- NULL = not known to be compromised. When set, re-encrypt-on-read must
  -- fail the read rather than return plaintext whenever it cannot persist
  -- the re-encrypted blob. Set either at rotation time or afterwards, when
  -- a key previously believed safe turns out not to be.
  compromised_at    TIMESTAMPTZ,
  -- A grace deadline and a compromise flag both describe a key that is no
  -- longer the active encryption key. Refusing either on the active row
  -- makes "we know this key leaked but we are still encrypting under it"
  -- unrepresentable: marking the active key compromised is a rotation, not
  -- a flag update.
  CONSTRAINT field_crypto_keys_retired_only_flags CHECK (
    (decryptable_until IS NULL AND compromised_at IS NULL) OR retired_at IS NOT NULL
  ),
  CONSTRAINT field_crypto_keys_grace_after_retirement CHECK (
    decryptable_until IS NULL OR decryptable_until >= retired_at
  )
);

-- At most one active key. Unique over a constant-true expression,
-- restricted to the rows where retired_at IS NULL, so a second active row
-- is a unique violation rather than a state the application has to
-- police. Also the first-boot convergence arbiter (see the table comment).
CREATE UNIQUE INDEX field_crypto_keys_one_active
  ON field_crypto_keys ((retired_at IS NULL))
  WHERE retired_at IS NULL;

-- retired_at and compromised_at are self-timestamping, but a grace-window
-- extension writes only decryptable_until; updated_at is what records when
-- that happened. Same set_updated_at() helper every other mutable table
-- uses (0001_helpers.sql).
CREATE TRIGGER field_crypto_keys_set_updated_at
  BEFORE UPDATE ON field_crypto_keys
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- No secondary indexes: this table gains one row per rotation and is read
-- in full at process start.

-- +goose Down

DROP TRIGGER IF EXISTS field_crypto_keys_set_updated_at ON field_crypto_keys;
DROP TABLE IF EXISTS field_crypto_keys;
```

### Generated Go types

With `model/sqlc.yaml`'s existing overrides (nullable `timestamptz` → `*time.Time`, non-null
`timestamptz` → `pgtype.Timestamptz`), the regenerated model struct is:

```go
type FieldCryptoKey struct {
	Version          int32              `json:"version"`
	KeyBytes         []byte             `json:"key_bytes"`
	CreatedAt        pgtype.Timestamptz `json:"created_at"`
	UpdatedAt        pgtype.Timestamptz `json:"updated_at"`
	RetiredAt        *time.Time         `json:"retired_at"`
	DecryptableUntil *time.Time         `json:"decryptable_until"`
	CompromisedAt    *time.Time         `json:"compromised_at"`
}
```

The three nullable columns landing as `*time.Time` is deliberate and convenient: `RetiredAt == nil`
is "active", `CompromisedAt != nil` is "compromised", and `DecryptableUntil` is directly comparable
with `time.Now()` for an in-process expiry re-check.

### Column-by-column rationale

**`version INTEGER GENERATED ALWAYS AS IDENTITY`.** `GENERATED ALWAYS` (rather than the `BIGSERIAL`
used elsewhere in `model/migrations/`) is a deliberate departure: it makes it impossible for
application code, a fixture, or a hand-run `INSERT` to pick a version number, and a reused version
number is the one failure mode in this design that silently produces wrong plaintext instead of a
loud error. `INTEGER` rather than `SMALLINT` because sqlc maps `SMALLINT` to `int16`, an awkward Go
type to carry through an encoder, and because a 16-bit ceiling is a real (if distant) limit while a
32-bit one is not. Version gaps are expected — a lost bootstrap race consumes an identity value —
and no test should assert contiguity. *If the pinned sqlc (1.31.1) or `goose validate` rejects the
`GENERATED ALWAYS AS IDENTITY` syntax, fall back to `version SERIAL PRIMARY KEY` and add
`CHECK (version > 0)`; nothing else in the design depends on the identity form.*

**`key_bytes BYTEA NOT NULL UNIQUE CHECK (octet_length(key_bytes) = 32)`.** Length check carried
over unchanged from the current migration, for the same defense-in-depth reason. `UNIQUE` is new:
key material is generated randomly, so an accidental collision is impossible, which means a
collision can only be an operator re-supplying material that is already on file — exactly the
mistake (re-introducing a key retired as compromised) worth turning into a hard error.

**`retired_at` as the activeness marker, rather than a `status` column.** A `status` enum plus
timestamps admits states where the two disagree (`status = 'retired'` with `retired_at IS NULL`),
and nothing in the lifecycle needs a third state: the rotation endpoint generates and activates in
one transaction, so there is no "created but not yet active" row. One column, no possible skew.

**Grace period as `decryptable_until TIMESTAMPTZ`, not `grace_period_days INT`.** Four reasons, in
order of weight:

1. A days integer is ambiguous about its origin — days from `created_at` or from `retired_at`? — and
   forces every consumer (the Go cipher, the SQL load filter, the admin API) to independently
   re-derive the same interval arithmetic and agree on it. An absolute instant is directly
   comparable in both SQL (`decryptable_until > now()`) and Go (`time.Now().Before(*k.DecryptableUntil)`)
   with no arithmetic at all.
2. It expresses "no expiry" (`NULL`) and "expired now" (any past instant) uniformly. A days integer
   has to overload `NULL` or `0` for one of those, and `0` reads ambiguously.
3. It is stable under a later change to the grace policy: keys retired under the old policy keep the
   deadline they were actually given, instead of retroactively moving when a default changes.
4. The operator-facing input stays in days regardless — the admin API takes `grace_period_days`, and
   the *SQL* resolves it (`now() + $n * INTERVAL '1 day'`), so the database clock is authoritative
   and no app/DB clock skew enters the stored deadline.

The cost is that changing a window after the fact is an `UPDATE` rather than a re-read of a policy
number; that `UPDATE` is provided as `SetFieldCryptoKeyDecryptableUntil` below, which the operator
needs anyway as the escape hatch for a window about to expire over data that has not been read yet.

**`NULL` (no expiry) is the default, and expiry is opt-in.** mod-core cannot know whether any blob
still carries a given version without scanning every encrypted column in every module, so a grace
deadline is an inherently blind cutoff: when it passes, any un-rotated blob under that key becomes
unreadable. Defaulting to "expires in N days" would make silent data loss the default behavior of a
routine rotation. The safe default is indefinite decryptability, with a deadline set only when an
operator explicitly asks for one.

**`compromised_at TIMESTAMPTZ` rather than `compromised BOOLEAN`.** `IS NULL` / `IS NOT NULL` tests
exactly as easily as a boolean, and the timestamp carries the incident date for free — there is no
audit table here, so the column is the only record of when the determination was made. Set by
`MarkFieldCryptoKeyCompromised` independently of any rotation event, which answers open point 3 of
[`rotation-api-shape.md`](./rotation-api-shape.md): **yes, compromise is settable after the fact**,
on any already-retired version, not only at the moment its replacement is generated.

**`updated_at` + `set_updated_at()` trigger.** Follows the convention `0016_apps_updated_at.sql`
states explicitly (a table with mutable columns carries its own `updated_at` and trigger). It earns
its place solely for grace-window changes, which are the one mutation not already self-timestamping.
Droppable if a reviewer objects to the extra column; nothing depends on it.

### Invariants and where each is enforced

| Invariant | Enforced by |
| --- | --- |
| At most one active key | `field_crypto_keys_one_active` partial unique index |
| Concurrent first boots converge on one key | Same index, via `ON CONFLICT DO NOTHING` |
| A version number is never reused | `GENERATED ALWAYS AS IDENTITY` + primary key |
| Key material is 32 bytes | `CHECK (octet_length(key_bytes) = 32)` |
| Key material is never re-introduced | `UNIQUE (key_bytes)` |
| Grace/compromise flags only on retired keys | `field_crypto_keys_retired_only_flags` CHECK |
| A grace deadline is not before retirement | `field_crypto_keys_grace_after_retirement` CHECK |
| At least one active key always exists | *Not expressible as a table constraint* — upheld by running retire+insert in one transaction, and by the cipher failing loudly rather than minting a key when the table is non-empty but has no active row |

## First-boot convergence and bootstrap

### The mechanism

The `id = 1` singleton is replaced by the one-active partial unique index, which serves the identical
purpose without any change to the calling pattern. The bootstrap insert is always an *active-key*
insert, so N concurrent first-boot callers race on that index exactly as they raced on the primary
key before: one wins, every other gets zero rows back from `ON CONFLICT DO NOTHING`, and re-reads to
adopt the winner's key. `pgx` surfaces the zero-row result of a `:one` query as `pgx.ErrNoRows`,
which is precisely the branch `fromPersistedOrGenerated` already has.

`ON CONFLICT DO NOTHING` is written **without a conflict target**, which arbitrates on any unique
index including a partial expression index. Do not attempt to name the partial index as an inference
target; the untargeted form is both simpler and correct here.

```sql
INSERT INTO field_crypto_keys (key_bytes)
SELECT sqlc.arg(key_bytes)::BYTEA
WHERE NOT EXISTS (SELECT 1 FROM field_crypto_keys)
ON CONFLICT DO NOTHING
RETURNING version, key_bytes, created_at, updated_at, retired_at, decryptable_until, compromised_at;
```

The two guards compose deliberately, and both are needed:

- `WHERE NOT EXISTS (SELECT 1 FROM field_crypto_keys)` covers the *committed* case: a table that
  holds rows but no active row (only reachable through operator error or a partial manual edit,
  since rotation is atomic) must **not** silently mint a new active key. Minting one there would
  strand every existing blob behind a key the cipher then treats as retired-with-no-replacement.
- `ON CONFLICT DO NOTHING` covers the *concurrent* case that `NOT EXISTS` cannot see, which is the
  original first-boot race.

Zero rows returned therefore means "someone else already established the key material" for either
reason, and the caller's response is the same in both: re-run the load query and use what is there.
If that re-load finds no active key, that is a hard, loud error — never a second attempt to
generate.

### Where `CORE_FIELD_KEY_HEX` fits

With the DB as single source of truth, an env key that is *not* in the table has no version number
and therefore cannot legally encrypt anything. The env var can only be a bootstrap seed: on first
boot with an empty table, its bytes are what the bootstrap insert persists as version 1, instead of
freshly generated random bytes. Every other path is identical.

That leaves the case where the table is non-empty and `CORE_FIELD_KEY_HEX` is set. Recommended
behavior — **flagged for confirmation**, see [Open questions](#open-questions) — is:

- env bytes equal the active key's bytes → proceed silently (the normal steady state for an operator
  who bootstrapped from env and never removed the variable);
- env bytes differ from the active key's bytes → **fail loudly at construction**, naming the admin
  rotation endpoint in the error. Silently ignoring the variable would leave an operator believing
  they had rotated by editing an env var when they had not.

This is the one place where the overview's "`CORE_FIELD_KEY_HEX` keeps winning when set" cannot
survive intact, because "winning" without a version row is now unrepresentable. The overview already
scopes that clause with "beyond whatever the multi-key model requires of it", so this is a required
consequence rather than a re-opened decision — but it changes observable operator behavior and
belongs in the app-mfmanager deploy-doc rewrite the overview already defers.

## Rotation transaction

Rotation is two statements in one transaction, in this order, run through a tx-scoped querier
(`coredb.New(tx)` / `q.WithTx(tx)` — the pattern `AppsHandler` already uses):

1. `RetireActiveFieldCryptoKey` — stamps `retired_at`, resolves `decryptable_until` from the
   supplied grace days, and sets `compromised_at` when the caller declares this a compromised-key
   rotation. Returns the retired version; zero rows means there was no active key, which is a loud
   error, not a no-op.
2. `InsertActiveFieldCryptoKey` — inserts the new key material as the new active row and returns its
   version.

**The order is mandatory.** A unique index is checked immediately and, being partial, cannot be
declared `DEFERRABLE`, so inserting the replacement before retiring the incumbent always fails.

Two concurrent rotations resolve safely without any extra locking: the second `UPDATE` blocks on the
first transaction's row lock, then re-evaluates `WHERE retired_at IS NULL`, matches nothing, and
returns zero rows — a loud failure the admin endpoint surfaces as a conflict. Even if that check
were bypassed, the second `INSERT` would hit the one-active index and raise a unique violation. The
failure mode is "one rotation wins, the other errors and is retried", never two active keys.
`pg_advisory_xact_lock` is available as optional hardening if the admin endpoint wants a cleaner
error, but it is not required for correctness.

Marking the *active* key compromised is not a flag update — the `field_crypto_keys_retired_only_flags`
CHECK forbids it. It is a rotation with the compromised flag set, which is the action an operator
must take anyway. Inside the single rotation transaction the constraint is satisfied, because
`retired_at` and `compromised_at` are written by the same statement.

## Grace expiry semantics

This answers open point 2 of [`rotation-api-shape.md`](./rotation-api-shape.md).

An expired key (`decryptable_until` in the past) simply stops being loaded — `ListUsableFieldCryptoKeys`
filters it out. A blob still carrying that version then hits the existing "version matches no loaded
key" path and **fails loudly**, which is already a stated success criterion in
[`overview.md`](../overview.md). There is no reaper, no sweep, and no deletion: key rows are tiny,
and deleting one is the only irreversible operation in this design.

Because the cipher loads keys once at construction, expiry must also be re-checked in Go at decrypt
time against the loaded `DecryptableUntil`, or a long-running process will keep honoring a key that
expired hours ago. Enforce in both places: the SQL filter keeps expired keys out of memory at load,
and the Go check makes expiry take effect without a reload.

The operator's recovery path when a window expires over data that turns out to still exist is
`SetFieldCryptoKeyDecryptableUntil`, which extends (or clears) the deadline; the process then picks
the key back up on its next load. Providing that query is what keeps the recovery path out of a
manual `psql` session.

For a compromised key, the recommended `decryptable_until` is `NULL`: you want the maximum
opportunity to re-encrypt data away from a leaked key, and the compromise policy already guarantees
that any read which *cannot* re-encrypt fails. Setting both a compromise flag and a short deadline
converts un-rotated rows into unreadable rows on a timer, which is a data-destruction control rather
than a security control. Flagged below rather than enforced, since it is arguably the backend
architect's call.

## Version identifier: integer, not fingerprint

`key-version-wire-format.md` left this open. Settled: **a monotone integer assigned by
`field_crypto_keys.version`**, encoded as a 4-byte big-endian `uint32` prefix and passed verbatim as
the AEAD AAD.

Why not a fingerprint derived from key material:

- The key table is already the source of truth and already assigns a unique, ordered identifier at
  no cost. A fingerprint would need its own uniqueness constraint and its own lookup index to do the
  same job.
- A fingerprint publishes a deterministic function of the secret key in every stored row and in
  every database dump and backup. Truncated-hash-of-key is not a practical break of AES-256-GCM, but
  it is gratuitous exposure that would need to be salted or HMAC'd under a separate secret to be
  defensible — more machinery for no gain.
- A fingerprint must be wide enough to make collisions negligible (8–16 bytes); the integer is 4, on
  a payload whose fixed overhead is otherwise 28 bytes.
- The integer orders naturally, so `ORDER BY version` *is* the rotation history, and "is this blob
  current?" is an integer comparison. A fingerprint gives neither.

The accepted trade-off: version numbers are meaningless across databases (two deployments both have
a version 1 with different key material). Blobs are never moved between deployments, and making
cross-deployment blobs portable is not a goal.

Consequences for the Phase 2 wire format:

- Layout `version(4) || nonce(12) || ciphertext || tag(16)`; minimum valid blob length becomes 32
  (currently 28). The empty-plaintext case is unchanged — `Encrypt("")` still returns a zero-length
  blob with no version prefix, and the DB layer still stores NULL.
- Encode with `binary.BigEndian.PutUint32`. Big-endian is the wire convention and keeps a hexdump
  readable; a varint would save two bytes at the cost of a variable-length header and a more fragile
  minimum-length check.
- AAD is exactly those 4 prefix bytes; the prefix is *not* part of the ciphertext passed to
  `AEAD.Open`.
- A decoded version of 0 is a malformed blob (the identity sequence starts at 1), so a zeroed or
  truncated prefix fails fast rather than looking like a plausible lookup miss.

## Migration path: edit `0017` in place

**Recommendation: edit `model/migrations/0017_field_crypto_keys.sql` in place. Do not add an `0018`.**

Reasoning:

1. The ground rule is a clean break — nothing is deployed on the current format, and existing
   databases are regenerated from scratch. [`overview.md`](../overview.md) already states the table
   is replaced outright "rather than migrating it in place".
2. A superseding `0018` that drops and recreates would leave `0017`'s comment block — a careful
   explanation of the `id = 1` singleton and why it makes concurrent first-boot inserts converge —
   standing in the tree as permanently misleading documentation of a table shape that no longer
   exists. Migrations in this repo are heavily commented and are read as schema design docs; leaving
   a lie in one is a real, recurring cost paid by every future reader.
3. Every path that matters applies migrations from scratch: `cd model && make lint` runs them against
   an ephemeral shadow Postgres, `make test.integration` and host-application startup run them
   against a fresh database.
4. An `0018` buys exactly one thing — a correct upgrade for a database that already applied `0017`
   and cannot be reset. No such database exists, by the user's explicit statement.

**The one real cost, which must become a task deliverable.** goose stores no per-migration checksum,
so a developer or CI database that already applied `0017` will report "no pending migrations" and
silently keep the old single-row table. The recovery is exact and cheap, and belongs in the task doc
and in the `AGENTS.md` note this plan already schedules:

```bash
cd model
# Down section is 'DROP TABLE IF EXISTS field_crypto_keys' in both the old and
# new file, so rolling back past 17 with the edited file in place is correct.
goose -dir migrations postgres "$DATABASE_URL" down-to 16
goose -dir migrations postgres "$DATABASE_URL" up
```

Recreating the database wholesale works equally well and is the more likely choice given the plan
already requires it for the blob format change.

**Adjacent comment-only edits, same justification.** `0010_natural_persons.sql` and
`0011_corporations.sql` both document their blob columns as holding
"`nonce || ciphertext || tag`". Those comments are the schema-level statement of the wire format and
go stale the moment Phase 2 lands; update both to `version || nonce || ciphertext || tag`. They are
comment-only edits to already-applied migrations, which change nothing at apply time.

## sqlc query surface

Six queries replace the two in `model/queries/field_crypto_keys.sql`. Both existing queries
(`GetFieldCryptoKey`, `InsertFieldCryptoKeyIfAbsent`) are deleted outright.

| Query | Kind | Called by | Purpose |
| --- | --- | --- | --- |
| `ListUsableFieldCryptoKeys` | `:many` | Phase 2 cipher, at construction and on reload | The whole key set the process may use: the active key plus every retired key still inside its grace window. Full column list, ordered by `version`. Filter: `retired_at IS NULL OR decryptable_until IS NULL OR decryptable_until > now()`. |
| `InsertInitialFieldCryptoKey` | `:one` | Phase 2 cipher, first boot only | The bootstrap insert shown above. Zero rows (`pgx.ErrNoRows`) means another caller established the key; re-load and adopt it. Takes key bytes from either the generator or `CORE_FIELD_KEY_HEX`. |
| `RetireActiveFieldCryptoKey` | `:one` | Admin rotation endpoint, step 1 | Stamps `retired_at = now()`, resolves `decryptable_until` from a nullable grace-days argument (`CASE WHEN $grace_days IS NULL THEN NULL ELSE now() + $grace_days * INTERVAL '1 day' END`), sets `compromised_at = now()` when a boolean `compromised` argument is true. `WHERE retired_at IS NULL RETURNING version`. Zero rows = no active key = loud error. |
| `InsertActiveFieldCryptoKey` | `:one` | Admin rotation endpoint, step 2 | Inserts new key material as the new active row, returns the full row. Must run in the same transaction as, and after, `RetireActiveFieldCryptoKey`. No `ON CONFLICT` clause — a conflict here must be loud. |
| `MarkFieldCryptoKeyCompromised` | `:one` | Admin endpoint, after the fact | `SET compromised_at = COALESCE(compromised_at, now()) WHERE version = $1 AND retired_at IS NOT NULL RETURNING version, compromised_at`. Idempotent (never moves an existing timestamp). Zero rows = unknown version or still-active key, which the handler maps to a 404/409. |
| `SetFieldCryptoKeyDecryptableUntil` | `:one` | Admin endpoint, operator recovery | Extends, shortens, or clears (`NULL` grace days) a retired key's grace window using the same `now() + n * INTERVAL '1 day'` resolution. `WHERE version = $1 AND retired_at IS NOT NULL RETURNING version, decryptable_until`. |

Deliberately **not** included, each a one-liner to add later if a caller materializes:

- `GetActiveFieldCryptoKey` — derivable in Go from `ListUsableFieldCryptoKeys` as the single row with
  `RetiredAt == nil`, which is code the cipher needs regardless. Keeping one load path avoids two
  places that can disagree about which key is active.
- `GetFieldCryptoKeyByVersion` — a stale process that meets an unknown version wants to re-run
  `ListUsableFieldCryptoKeys` (which also picks up the new *active* key, which is what it actually
  needs), not to fetch one row.

Deliberately **not** included and deliberately not returning key material: an admin inventory query
(`SELECT version, created_at, updated_at, retired_at, decryptable_until, compromised_at ...`, no
`key_bytes`) is likely wanted by the rotation endpoint's status/GET route. It is left to the
[parallel admin-endpoint design pass](./key-lifecycle-policy.md) to specify, because only it knows
whether that route exists; if added, its narrower column list yields a distinct sqlc row type, which
usefully makes "no key material crosses this boundary" a compile-time property.

**Query-authoring notes for the implementer.** Give the five key-material-bearing queries the full
column list in table order so sqlc reuses the `FieldCryptoKey` model struct rather than emitting a
per-query `...Row` type. `sqlc.narg()` is required for the nullable grace-days argument and
`sqlc.arg(...)::BYTEA` for the bare-`SELECT` bootstrap insert, whose parameter type is otherwise
uninferable. Verify all of it with `cd model && make verify` (`goose validate` + `sqlc compile`)
before regenerating.

### Cost this imposes on `coredb.Querier` implementers

`model/sqlc.yaml` sets `emit_interface: true`, and four api test files declare
`var _ coredb.Querier = (*someStub)(nil)`: `api/types/types_test.go`, `api/entity/resolver_test.go`,
`api/httpapi/apps_test.go`, and `api/httpapi/masked_lookup_test.go`. Every query added to or removed
from this file forces edits in all four. Going from two queries to six means roughly sixteen no-op
stub methods to add and four to delete — mechanical, but easy to omit from a task estimate and
guaranteed to break the api build if missed. This is also the concrete reason the query list above
is kept as short as it is.

A separate sqlc package for key queries (a second `sql:` entry in `sqlc.yaml`) would isolate that
churn permanently, and was considered and rejected: it needs a second `Queries` value constructed
from the same pool and a manifest arg source other than `queries:coredb`, which mfgen does not offer.

### Adjacent: the write-back queries Phase 1 also owns

Out of this table's scope but in Phase 1's, and worth specifying here because the shape is
non-obvious. Re-encrypt-on-read cannot reuse `UpdateNaturalPerson` / `UpdateCorporation`: those
rewrite the domain columns too, and their `COALESCE($4, ssn)` convention means "NULL leaves it
unchanged", which cannot express "replace this blob". Two narrow queries are needed:

```sql
-- name: UpdateNaturalPersonSSNBlob :execrows
UPDATE natural_persons
SET ssn = sqlc.arg(new_ssn)
WHERE entity_id = sqlc.arg(entity_id) AND ssn = sqlc.arg(old_ssn);
```

plus the `corporations.ein` equivalent. Two design points:

- **Compare-and-swap on the old blob** (`AND ssn = $old_ssn`), not a blind write. An opportunistic
  write-back must never clobber a value another writer changed between the read and the write-back.
- **`:execrows`, not `:exec`.** Zero rows affected means the stored blob is no longer the one that
  was read. Under a standard-rotation key that is a benign skip (log and continue); under a
  compromised key the write-back did not happen, which by the stated policy fails the read. The
  subtlety that zero rows may *also* mean another writer already replaced the blob with a
  current-version one is noted in [Open questions](#open-questions).

## Consequences beyond the table

These are Phase 2/3 concerns that this schema forces, recorded so task authors budget for them.

**`FieldKeyQuerier` cannot stay import-free by accident.** Today's interface trades in `[]byte`, so
`api/internal/fieldcrypto` satisfies it structurally from `*coredb.Queries` without importing
`core-model/db`. A multi-key load returns `[]coredb.FieldCryptoKey`, and naming that type requires
the import. The recommended shape preserves the existing decoupling *and* the manifest:

- `api/internal/fieldcrypto` declares its own `KeyRecord` struct (version, key bytes, retired-at,
  decryptable-until, compromised-at) and a `KeyStore` interface over it (`LoadUsableKeys`,
  `InsertInitialKey`). All bootstrap and race-resolution logic stays here.
- `api/fieldcrypto`, the façade the manifest already names, gains the `coredb` import and a ~30-line
  adapter mapping sqlc rows to `KeyRecord`.
- `moduleforge.module.yaml`'s `cipher` block is unchanged: still
  `constructor: fieldcrypto.NewFromEnvOrGenerate` with `args: [context, queries:coredb]`. That
  matters — an unchanged manifest removes the mfgen/composing-app regeneration risk the overview
  flags for Phase 3.

The alternative (import `core-model/db` directly into `api/internal/fieldcrypto`) costs no new module
dependency — `api/go.mod` already requires `core-model` — but discards a deliberate design property
and drags `coredb` into the cipher's unit test fakes.

**The cipher's key set goes stale across processes.** After replica A rotates, replica B keeps
encrypting under the now-retired version and cannot decrypt anything B has not seen before. The
schema supports the fix and the fix is cheap: hold the `KeyStore` on the `Cipher` and re-run
`LoadUsableKeys` once on encountering an unknown version before failing. Continuing to *encrypt*
under a stale active key is the residual gap; it self-heals on restart and the blobs stay readable
because the old key is retired-with-grace rather than deleted. Mechanism and policy belong to Phase 2
and the parallel admin-endpoint pass; flagged below.

**A rotation write-back bumps the row's `updated_at`.** `natural_persons` and `corporations` both
carry unconditional `BEFORE UPDATE ... set_updated_at()` triggers, so an opportunistic re-encrypt
makes a pure read look like a domain modification to anything reading `updated_at`. Flagged below
rather than decided.

**A database dump now leaks every key, not one.** With the DB as single source of truth, the table
holds all active and retired key material, so a dump compromises data encrypted under *every* key
version, retroactively — where previously an env-only operator kept key material out of the database
entirely. This is an accepted consequence of a settled decision, not a re-opened one, but it should
be recorded as a cross-repo followup (envelope-encrypt `key_bytes` under a KEK supplied by the
environment) and must be stated plainly in the app-mfmanager deploy-doc rewrite the overview already
defers.

## Phase 1 task checklist

Everything the model layer owes, derived from the above:

1. Rewrite `model/migrations/0017_field_crypto_keys.sql` in place with the DDL above (table, partial
   unique index, `updated_at` trigger, `Down` section).
2. Update the blob-format comments in `0010_natural_persons.sql` and `0011_corporations.sql` to the
   versioned layout.
3. Rewrite `model/queries/field_crypto_keys.sql` with the six queries; delete `GetFieldCryptoKey` and
   `InsertFieldCryptoKeyIfAbsent`.
4. Add the two narrow blob-only CAS write-back queries to `model/queries/natural_persons.sql` and
   `model/queries/corporations.sql`.
5. Regenerate `model/db/` (`cd model && make gen`) and commit it.
6. Update the four `coredb.Querier` stub implementations in `api/` so the api module still compiles.
7. Validate: `cd model && make verify` and `cd model && make lint`.
8. Document the `goose down-to 16 && goose up` (or full database recreate) step required of anyone
   with an existing database.

## Open questions

Flagged rather than guessed. Each names who should resolve it.

1. **Is "active key known compromised, rotation pending" a state operations needs?** The
   `field_crypto_keys_retired_only_flags` CHECK makes it unrepresentable by design, forcing
   "mark compromised" on the active key to be a rotation. If the parallel admin-endpoint design
   wants an alarm state that precedes rotation, drop `compromised_at` from that CHECK (leaving the
   `decryptable_until` half) and define what the cipher does when its *active* key is flagged. →
   architect-backend pass.
2. **Does the rotation write-back bumping `updated_at` matter?** Recommended: accept and document —
   `updated_at` on these tables already means "row last written", and nothing currently uses it as an
   ETag or update precondition. The alternative, if a reviewer disagrees, is a transaction-scoped GUC
   (`SET LOCAL app.suppress_updated_at = on`) checked inside `set_updated_at()`, which means editing
   the shared helper in `0001_helpers.sql` and thereby touching every table's trigger behavior. →
   architect-backend pass / Phase 3.
3. **Stale key sets across processes.** Reload-on-unknown-version is the recommended minimum and the
   schema supports it, but whether a periodic refresh is also wanted — and what a replica should do
   when it discovers its active key was retired underneath it — is Phase 2 + admin-endpoint policy. →
   architect-backend pass.
4. **`CORE_FIELD_KEY_HEX` precedence once the table is populated.** Recommended fail-loudly-on-mismatch
   is a visible behavior change for operators, and the overview lists env-always-wins as existing
   deliberate behavior. Worth one confirming question to the user rather than a silent reinterpretation.
   → manager/user.
5. **Should a compromised rotation force `decryptable_until = NULL`?** Recommended yes (maximize the
   window to re-encrypt away from a leaked key), but not enforced by a constraint, so the admin
   endpoint's default matters. → architect-backend pass.
6. **CAS write-back returning zero rows under a compromised key.** Zero rows means the stored blob
   changed underneath the reader — which usually means another writer already wrote a
   current-version blob, i.e. the compromised ciphertext is already gone. Treating it uniformly as
   "not persisted → fail the read" is the simple, safe rule and is what this note recommends; a
   re-read-and-verify refinement is possible but adds a round trip to a path that should stay cheap.
   → architect-backend pass.
