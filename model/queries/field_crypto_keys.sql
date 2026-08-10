-- name: GetFieldCryptoKey :one
SELECT key_bytes FROM field_crypto_keys WHERE id = 1;

-- name: InsertFieldCryptoKeyIfAbsent :one
INSERT INTO field_crypto_keys (id, key_bytes)
VALUES (1, $1)
ON CONFLICT (id) DO NOTHING
RETURNING key_bytes;
