-- name: ListUsableFieldCryptoKeys :many
SELECT version, key_bytes, created_at, updated_at, retired_at, decryptable_until, compromised_at
FROM field_crypto_keys
WHERE retired_at IS NULL OR decryptable_until IS NULL OR decryptable_until > now()
ORDER BY version;

-- name: InsertInitialFieldCryptoKey :one
INSERT INTO field_crypto_keys (key_bytes)
SELECT sqlc.arg(key_bytes)::BYTEA
WHERE NOT EXISTS (SELECT 1 FROM field_crypto_keys)
ON CONFLICT DO NOTHING
RETURNING version, key_bytes, created_at, updated_at, retired_at, decryptable_until, compromised_at;

-- name: RetireActiveFieldCryptoKey :one
UPDATE field_crypto_keys
SET retired_at = now(),
    decryptable_until = CASE
      WHEN sqlc.narg(grace_days)::INT IS NULL THEN NULL
      ELSE now() + sqlc.narg(grace_days)::INT * INTERVAL '1 day'
    END,
    compromised_at = CASE WHEN sqlc.arg(compromised)::BOOLEAN THEN now() ELSE NULL END
WHERE retired_at IS NULL
RETURNING version, retired_at, decryptable_until, compromised_at;

-- name: InsertActiveFieldCryptoKey :one
INSERT INTO field_crypto_keys (key_bytes)
VALUES (sqlc.arg(key_bytes)::BYTEA)
RETURNING version, key_bytes, created_at, updated_at, retired_at, decryptable_until, compromised_at;

-- name: MarkFieldCryptoKeyCompromised :one
UPDATE field_crypto_keys
SET compromised_at = COALESCE(compromised_at, now())
WHERE version = $1 AND retired_at IS NOT NULL
RETURNING version, compromised_at;

-- name: SetFieldCryptoKeyDecryptableUntil :one
UPDATE field_crypto_keys
SET decryptable_until = CASE
  WHEN sqlc.narg(grace_days)::INT IS NULL THEN NULL
  ELSE now() + sqlc.narg(grace_days)::INT * INTERVAL '1 day'
END
WHERE version = $1 AND retired_at IS NOT NULL
RETURNING version, decryptable_until;

-- name: ListFieldCryptoKeyMetadata :many
SELECT version, created_at, updated_at, retired_at, decryptable_until, compromised_at
FROM field_crypto_keys
ORDER BY version;

-- name: GetFieldCryptoKeyByVersion :one
SELECT version, created_at, updated_at, retired_at, decryptable_until, compromised_at
FROM field_crypto_keys
WHERE version = $1;
