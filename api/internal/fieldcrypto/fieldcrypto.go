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
// # Authenticated additional data
//
// No AAD is bound to ciphertext at this time. This means a ciphertext blob
// could theoretically be moved from one row to another without detection.
// Binding ciphertext to a specific row id is a planned hardening step.
package fieldcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("fieldcrypto: %s is not valid hex: %w", envKeyName, err)
	}
	return NewFromKey(key)
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
