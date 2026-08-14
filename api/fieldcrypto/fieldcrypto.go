// Package fieldcrypto re-exports the internal versioned AES-256-GCM field
// cipher for use by callers outside core-api (e.g. the users-module main
// package). The implementation lives in internal/fieldcrypto to keep the
// cipher internals free of any core-model dependency; this façade absorbs
// that dependency at the boundary, adapting sqlc-generated coredb rows onto
// the internal package's model-free KeyRecord.
package fieldcrypto

import (
	"context"
	"fmt"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
	coredb "github.com/moduleforge/core-model/db"
)

// Cipher is the exported type alias for fieldcrypto.Cipher.
type Cipher = fieldcrypto.Cipher

// KeyRecord is the exported type alias for fieldcrypto.KeyRecord.
type KeyRecord = fieldcrypto.KeyRecord

// KeyStore is the exported type alias for fieldcrypto.KeyStore.
type KeyStore = fieldcrypto.KeyStore

// Rotation is the exported type alias for fieldcrypto.Rotation.
type Rotation = fieldcrypto.Rotation

// BlobVersion decodes the key version from a stored blob's 4-byte prefix. See
// fieldcrypto.BlobVersion for the full contract.
func BlobVersion(blob []byte) (uint32, error) {
	return fieldcrypto.BlobVersion(blob)
}

// NewFromKey constructs a store-less Cipher from a raw 32-byte key. Useful in
// tests.
func NewFromKey(key []byte) (*Cipher, error) {
	return fieldcrypto.NewFromKey(key)
}

// FieldKeyQuerier is the façade's persistence contract: the two
// field_crypto_keys queries NewFromEnvOrGenerate needs. It is satisfied
// structurally by both *coredb.Queries and coredb.Querier, so the module
// manifest's queries:coredb arg source still type-checks unchanged. Returned
// KeyBytes must be freshly allocated on every call and retained by nothing
// else — the cipher zeroes it after building the AEAD.
type FieldKeyQuerier interface {
	ListUsableFieldCryptoKeys(ctx context.Context) ([]coredb.FieldCryptoKey, error)
	InsertInitialFieldCryptoKey(ctx context.Context, keyBytes []byte) (coredb.FieldCryptoKey, error)
}

// NewFromEnvOrGenerate constructs the process's Cipher from q, bootstrapping
// the first key when the key table is empty. Its name, parameter list, and
// error return are fixed by moduleforge.module.yaml's cipher service block
// (constructor: fieldcrypto.NewFromEnvOrGenerate, args: [context,
// queries:coredb]) and must not change.
func NewFromEnvOrGenerate(ctx context.Context, q FieldKeyQuerier) (*Cipher, error) {
	return fieldcrypto.NewFromEnvOrGenerate(ctx, &keyStoreAdapter{q: q})
}

// keyStoreAdapter implements the internal package's KeyStore over a
// FieldKeyQuerier, mapping coredb.FieldCryptoKey rows onto KeyRecord values.
type keyStoreAdapter struct {
	q FieldKeyQuerier
}

func (a *keyStoreAdapter) LoadUsableKeys(ctx context.Context) ([]KeyRecord, error) {
	rows, err := a.q.ListUsableFieldCryptoKeys(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]KeyRecord, len(rows))
	for i, row := range rows {
		rec, err := keyRecordFromRow(row)
		if err != nil {
			// Best-effort key hygiene, mirroring the internal package's own
			// zeroKeyMaterial: a mapping failure partway through the batch
			// must not leave any row's key bytes — processed or not —
			// sitting unzeroed in the returned rows slice.
			for j := range rows {
				zeroBytes(rows[j].KeyBytes)
			}
			return nil, err
		}
		records[i] = rec
	}
	return records, nil
}

func (a *keyStoreAdapter) InsertInitialKey(ctx context.Context, keyBytes []byte) (KeyRecord, error) {
	row, err := a.q.InsertInitialFieldCryptoKey(ctx, keyBytes)
	if err != nil {
		return KeyRecord{}, err
	}
	rec, err := keyRecordFromRow(row)
	if err != nil {
		zeroBytes(row.KeyBytes)
		return KeyRecord{}, err
	}
	return rec, nil
}

// zeroBytes overwrites b. Best-effort key hygiene, matching the internal
// package's zeroBytes: it shortens how long key material sits in a heap the
// process may later dump or swap, and nothing more.
func zeroBytes(b []byte) { clear(b) }

// keyRecordFromRow maps one coredb.FieldCryptoKey row onto a KeyRecord,
// returning a fresh KeyRecord built directly from row on every call — never a
// cached or previously-returned value. The Cipher zeroes the KeyBytes slice
// it is handed once the AEADs are built from it, so an adapter that reused or
// aliased a prior call's slice would hand back zeros instead of key material
// on the next load.
func keyRecordFromRow(row coredb.FieldCryptoKey) (KeyRecord, error) {
	if row.Version < 0 {
		return KeyRecord{}, fmt.Errorf("fieldcrypto: corrupt key row: version %d is negative", row.Version)
	}
	return KeyRecord{
		Version:          uint32(row.Version),
		KeyBytes:         row.KeyBytes,
		RetiredAt:        row.RetiredAt,
		DecryptableUntil: row.DecryptableUntil,
		CompromisedAt:    row.CompromisedAt,
	}, nil
}
