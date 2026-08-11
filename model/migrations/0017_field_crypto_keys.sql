-- +goose Up

-- field_crypto_keys is a private, single-row table holding the
-- AES-256-GCM field-encryption key when CORE_FIELD_KEY_HEX is not
-- supplied via the environment (see
-- api/internal/fieldcrypto.NewFromEnvOrGenerate). The id = 1 CHECK plus
-- PRIMARY KEY makes this table hold at most one row ever: concurrent
-- first-boot INSERTs race on that constraint, so exactly one writer wins
-- and every other writer must adopt the winner's key rather than each
-- independently generating one and picking arbitrarily (see
-- InsertFieldCryptoKeyIfAbsent's ON CONFLICT (id) DO NOTHING below). The
-- octet_length CHECK is a defense-in-depth guard against a corrupt or
-- truncated key ever being persisted through this code path;
-- NewFromEnvOrGenerate independently validates length on every read and
-- fails loudly rather than regenerating if a persisted key does not
-- decode to exactly 32 bytes.
--
-- Deliberately not modeled as an entity (no FK to entities.id): this is
-- bootstrap/operational data unrelated to the domain entity hierarchy,
-- the same reasoning that keeps goose_db_version_core a bare table.
CREATE TABLE field_crypto_keys (
  id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  key_bytes  BYTEA NOT NULL CHECK (octet_length(key_bytes) = 32),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE IF EXISTS field_crypto_keys;
