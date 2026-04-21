// Package fieldcrypto re-exports the internal AES-256-GCM cipher for use by
// callers outside core-api (e.g. the users-module main package). The
// implementation lives in internal/fieldcrypto to keep the cipher internals
// package-private; this façade exports only what callers need.
package fieldcrypto

import (
	"github.com/moduleforge/core-api/internal/fieldcrypto"
)

// Cipher is the exported type alias for fieldcrypto.Cipher.
type Cipher = fieldcrypto.Cipher

// NewFromEnv reads CORE_FIELD_KEY_HEX and constructs a Cipher.
// Returns an error if the env var is missing, not valid hex, or not 32 bytes.
func NewFromEnv() (*Cipher, error) {
	return fieldcrypto.NewFromEnv()
}

// NewFromKey constructs a Cipher from a raw 32-byte key. Useful in tests.
func NewFromKey(key []byte) (*Cipher, error) {
	return fieldcrypto.NewFromKey(key)
}
