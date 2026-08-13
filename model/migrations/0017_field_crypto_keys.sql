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
