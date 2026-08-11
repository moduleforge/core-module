// Package fieldcrypto provides AES-256-GCM encryption and decryption for
// small database field values such as SSN and EIN.
//
// # Key management
//
// A Cipher holds exactly one 32-byte key. Key rotation is out of scope for
// this package; if rotation is required it must be handled at the call site
// (e.g. re-encrypt on read when the active key differs from the key that
// produced the stored blob).
//
// CORE_FIELD_KEY_HEX, when set, always wins: NewFromEnv and
// NewFromEnvOrGenerate both read it via the same fromHexKey helper, so
// behavior is identical whichever constructor a caller uses. When the env
// var is unset, NewFromEnvOrGenerate falls back to a durably persisted key,
// fetching one if it already exists or generating and persisting one on
// first boot. Concurrent first-boot callers converge on the same key via a
// DB-level uniqueness constraint (ON CONFLICT DO NOTHING), not a
// read-then-write race. A persisted key that fails validation (wrong
// length) is a fail-loudly error, never silently regenerated — regenerating
// over an existing key would make data already encrypted under the old key
// unrecoverable.
//
// # Authenticated additional data
//
// No AAD is bound to ciphertext at this time. This means a ciphertext blob
// could theoretically be moved from one row to another without detection.
// Binding ciphertext to a specific row id is a planned hardening step.
package fieldcrypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

const (
	envKeyName = "CORE_FIELD_KEY_HEX"
	keySize    = 32 // AES-256
	hexKeySize = keySize * 2
)

// Cipher encrypts and decrypts small field values with AES-256-GCM.
// A single Cipher holds one 32-byte key; rotation is out of scope here.
// Cipher values are safe for concurrent use.
type Cipher struct {
	aead cipher.AEAD
}

// NewFromEnv reads CORE_FIELD_KEY_HEX (64 hex chars = 32 bytes) and
// returns a Cipher. Returns an error if the env var is missing, not valid
// hex, or the decoded key is not exactly 32 bytes.
func NewFromEnv() (*Cipher, error) {
	hexKey, ok := os.LookupEnv(envKeyName)
	if !ok {
		return nil, fmt.Errorf("fieldcrypto: env var %s is not set", envKeyName)
	}
	return fromHexKey(hexKey)
}

// fromHexKey decodes hexKey and builds a Cipher from it. Shared by NewFromEnv
// and NewFromEnvOrGenerate's env-var branch so both run the identical
// decode-and-validate code path.
func fromHexKey(hexKey string) (*Cipher, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("fieldcrypto: %s is not valid hex: %w", envKeyName, err)
	}
	return NewFromKey(key)
}

// FieldKeyQuerier is the minimal persistence contract
// NewFromEnvOrGenerate needs. Satisfied structurally by *coredb.Queries
// (github.com/moduleforge/core-model/db) — the same object every other
// mod-core service constructs via coredb.New(pool) — so this package
// does not need to import core-model/db at all; callers (and the
// moduleforge.module.yaml queries:coredb arg-source) pass it directly.
type FieldKeyQuerier interface {
	GetFieldCryptoKey(ctx context.Context) ([]byte, error)
	InsertFieldCryptoKeyIfAbsent(ctx context.Context, keyBytes []byte) ([]byte, error)
}

// NewFromEnvOrGenerate reads CORE_FIELD_KEY_HEX if set — identical
// behavior to NewFromEnv in every respect, including on invalid values.
// Only when the env var is entirely unset does it fall back to q: fetch
// the persisted key if one exists, or generate and durably persist a new
// one. Concurrent first-boot callers are safe: the persistence layer
// guarantees (via a DB-level uniqueness constraint, not a read-then-write
// race) that every caller converges on the same key. A persisted key
// that fails validation (wrong length) is a fail-loudly error, never
// silently regenerated — regenerating over an existing key would make
// data already encrypted under the old key unrecoverable.
func NewFromEnvOrGenerate(ctx context.Context, q FieldKeyQuerier) (*Cipher, error) {
	if hexKey, ok := os.LookupEnv(envKeyName); ok {
		return fromHexKey(hexKey)
	}
	return fromPersistedOrGenerated(ctx, q)
}

// fromPersistedOrGenerated fetches the persisted field-crypto key, or
// generates and persists a new one if none exists yet.
func fromPersistedOrGenerated(ctx context.Context, q FieldKeyQuerier) (*Cipher, error) {
	key, err := q.GetFieldCryptoKey(ctx)
	switch {
	case err == nil:
		// NewFromKey fails loudly (wrong length) rather than regenerating.
		return NewFromKey(key)
	case errors.Is(err, pgx.ErrNoRows):
		// fall through to generate-and-persist
	default:
		return nil, fmt.Errorf("fieldcrypto: read persisted key: %w", err)
	}

	candidate := make([]byte, keySize)
	if _, rerr := rand.Read(candidate); rerr != nil {
		return nil, fmt.Errorf("fieldcrypto: generate key: %w", rerr)
	}

	inserted, err := q.InsertFieldCryptoKeyIfAbsent(ctx, candidate)
	switch {
	case err == nil:
		// We won the race; the DB echoes back exactly what we inserted.
		return NewFromKey(inserted)
	case errors.Is(err, pgx.ErrNoRows):
		// ON CONFLICT DO NOTHING skipped our row: another caller won the
		// race between our SELECT and our INSERT. Adopt their key.
		winner, rerr := q.GetFieldCryptoKey(ctx)
		if rerr != nil {
			return nil, fmt.Errorf("fieldcrypto: re-fetch key after lost race: %w", rerr)
		}
		return NewFromKey(winner)
	default:
		return nil, fmt.Errorf("fieldcrypto: persist generated key: %w", err)
	}
}

// NewFromKey builds a Cipher from a raw 32-byte key. Useful in tests.
func NewFromKey(key []byte) (*Cipher, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("fieldcrypto: key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		// aes.NewCipher only errors on invalid key length, already guarded above.
		return nil, fmt.Errorf("fieldcrypto: failed to create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("fieldcrypto: failed to create GCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt returns nonce(12) || ciphertext || tag(16) as a single blob.
// Returns ([]byte{}, nil) when plaintext is the empty string so that
// callers do not have to special-case empty input; the DB layer stores NULL
// for a zero-length blob.
func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return []byte{}, nil
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("fieldcrypto: failed to generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce in place.
	blob := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return blob, nil
}

// Decrypt reverses Encrypt. Returns ("", nil) when blob is empty or nil.
// Returns an error when the blob is non-empty but malformed or authentication
// fails.
func (c *Cipher) Decrypt(blob []byte) (string, error) {
	if len(blob) == 0 {
		return "", nil
	}

	nonceSize := c.aead.NonceSize()
	// Minimum valid blob: nonce + tag (no plaintext bytes).
	minSize := nonceSize + 16
	if len(blob) < minSize {
		return "", errors.New("fieldcrypto: blob too short to be a valid ciphertext")
	}

	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("fieldcrypto: decryption failed: %w", err)
	}
	return string(plaintext), nil
}
