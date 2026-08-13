// Package fieldcrypto re-exports the internal versioned AES-256-GCM field
// cipher for use by callers outside core-api (e.g. the users-module main
// package). The implementation lives in internal/fieldcrypto to keep the
// cipher internals package-private; this façade exports only what callers
// need.
package fieldcrypto

import (
	"context"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
)

// Cipher is the exported type alias for fieldcrypto.Cipher.
type Cipher = fieldcrypto.Cipher

// KeyRecord is the exported type alias for fieldcrypto.KeyRecord.
type KeyRecord = fieldcrypto.KeyRecord

// KeyStore is the exported type alias for fieldcrypto.KeyStore.
type KeyStore = fieldcrypto.KeyStore

// NewFromKey constructs a store-less Cipher from a raw 32-byte key. Useful in
// tests.
func NewFromKey(key []byte) (*Cipher, error) {
	return fieldcrypto.NewFromKey(key)
}

// NewFromEnvOrGenerate constructs the process's Cipher from the key store,
// bootstrapping the first key when the key table is empty.
func NewFromEnvOrGenerate(ctx context.Context, store KeyStore) (*Cipher, error) {
	return fieldcrypto.NewFromEnvOrGenerate(ctx, store)
}
