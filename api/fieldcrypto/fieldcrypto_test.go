package fieldcrypto

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	coredb "github.com/moduleforge/core-model/db"
)

// Compile-time assertion: coredb.Querier already satisfies FieldKeyQuerier
// structurally, which is what the module manifest's queries:coredb arg
// source relies on. This line is what will catch a future query-signature
// change breaking that contract.
var _ FieldKeyQuerier = (coredb.Querier)(nil)

// unsetEnv removes key from the environment for the duration of the test and
// restores its prior value (if any) on cleanup.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prior, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("os.Unsetenv(%q): %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, prior) //nolint:errcheck // best-effort restore in test cleanup
		} else {
			os.Unsetenv(key) //nolint:errcheck
		}
	})
}

// testKey returns a 32-byte, non-zero key filled with seed.
func testKey(seed byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = seed
	}
	return key
}

func TestKeyRecordFromRowMapsNullableTimestamps(t *testing.T) {
	retiredAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	decryptableUntil := retiredAt.Add(30 * 24 * time.Hour)
	compromisedAt := retiredAt.Add(time.Hour)

	tests := []struct {
		name string
		row  coredb.FieldCryptoKey
	}{
		{
			name: "active key: every nullable field nil",
			row: coredb.FieldCryptoKey{
				Version:  4,
				KeyBytes: testKey(4),
			},
		},
		{
			name: "retired key: every nullable field set",
			row: coredb.FieldCryptoKey{
				Version:          3,
				KeyBytes:         testKey(3),
				RetiredAt:        &retiredAt,
				DecryptableUntil: &decryptableUntil,
				CompromisedAt:    &compromisedAt,
			},
		},
		{
			name: "retired key: decryptable_until nil (no expiry), compromised set",
			row: coredb.FieldCryptoKey{
				Version:       2,
				KeyBytes:      testKey(2),
				RetiredAt:     &retiredAt,
				CompromisedAt: &compromisedAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := keyRecordFromRow(tt.row)
			if err != nil {
				t.Fatalf("keyRecordFromRow: %v", err)
			}
			if rec.Version != uint32(tt.row.Version) {
				t.Errorf("Version = %d, want %d", rec.Version, tt.row.Version)
			}
			if string(rec.KeyBytes) != string(tt.row.KeyBytes) {
				t.Errorf("KeyBytes = %x, want %x", rec.KeyBytes, tt.row.KeyBytes)
			}
			if !sameTime(rec.RetiredAt, tt.row.RetiredAt) {
				t.Errorf("RetiredAt = %v, want %v", rec.RetiredAt, tt.row.RetiredAt)
			}
			if !sameTime(rec.DecryptableUntil, tt.row.DecryptableUntil) {
				t.Errorf("DecryptableUntil = %v, want %v", rec.DecryptableUntil, tt.row.DecryptableUntil)
			}
			if !sameTime(rec.CompromisedAt, tt.row.CompromisedAt) {
				t.Errorf("CompromisedAt = %v, want %v", rec.CompromisedAt, tt.row.CompromisedAt)
			}
		})
	}
}

// sameTime reports whether a and b are both nil or both non-nil and equal.
func sameTime(a, b *time.Time) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || a.Equal(*b)
}

func TestKeyRecordFromRowRejectsNegativeVersion(t *testing.T) {
	_, err := keyRecordFromRow(coredb.FieldCryptoKey{Version: -1, KeyBytes: testKey(1)})
	if err == nil {
		t.Fatal("keyRecordFromRow: expected an error for a negative version, got nil")
	}
}

// fakeFieldKeyQuerier is a minimal FieldKeyQuerier fake used to exercise
// keyStoreAdapter end-to-end through NewFromEnvOrGenerate.
type fakeFieldKeyQuerier struct {
	usable    []coredb.FieldCryptoKey
	insertRow coredb.FieldCryptoKey
	insertErr error

	insertCalls int
	// lastInsertedRow records the exact row value handed back by the most
	// recent InsertInitialFieldCryptoKey call, so a test can inspect its
	// KeyBytes backing array after the adapter has had a chance to zero it
	// on a mapping-failure path (the adapter mutates in place via aliasing).
	lastInsertedRow coredb.FieldCryptoKey
}

func (f *fakeFieldKeyQuerier) ListUsableFieldCryptoKeys(_ context.Context) ([]coredb.FieldCryptoKey, error) {
	return f.usable, nil
}

func (f *fakeFieldKeyQuerier) InsertInitialFieldCryptoKey(_ context.Context, keyBytes []byte) (coredb.FieldCryptoKey, error) {
	f.insertCalls++
	if f.insertErr != nil {
		return coredb.FieldCryptoKey{}, f.insertErr
	}
	row := f.insertRow
	// A defensive copy: the adapter must not be handed the same backing array
	// the Cipher goes on to zero, or a second call in the same test would
	// observe zeroed bytes rather than the seed it inserted.
	row.KeyBytes = append([]byte(nil), keyBytes...)
	f.lastInsertedRow = row
	return row, nil
}

func TestNewFromEnvOrGenerateAdaptsUsableKeys(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	q := &fakeFieldKeyQuerier{usable: []coredb.FieldCryptoKey{{
		Version:  1,
		KeyBytes: testKey(1),
	}}}

	c, err := NewFromEnvOrGenerate(context.Background(), q)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}
	if c == nil {
		t.Fatal("NewFromEnvOrGenerate returned a nil Cipher")
	}
}

func TestNewFromEnvOrGenerateRejectsCorruptRow(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	q := &fakeFieldKeyQuerier{usable: []coredb.FieldCryptoKey{{
		Version:  -1,
		KeyBytes: testKey(1),
	}}}

	if _, err := NewFromEnvOrGenerate(context.Background(), q); err == nil {
		t.Fatal("NewFromEnvOrGenerate: expected an error for a negative version, got nil")
	}
}

func TestNewFromEnvOrGenerateBootstrapsOnEmptyTable(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	q := &fakeFieldKeyQuerier{insertRow: coredb.FieldCryptoKey{Version: 1}}

	c, err := NewFromEnvOrGenerate(context.Background(), q)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}
	if c == nil {
		t.Fatal("NewFromEnvOrGenerate returned a nil Cipher")
	}
	if q.insertCalls != 1 {
		t.Errorf("InsertInitialFieldCryptoKey called %d times, want 1", q.insertCalls)
	}
}

func TestNewFromEnvOrGenerateWrapsInsertError(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	q := &fakeFieldKeyQuerier{insertErr: errors.New("boom")}

	if _, err := NewFromEnvOrGenerate(context.Background(), q); err == nil {
		t.Fatal("NewFromEnvOrGenerate: expected an error from a failed insert, got nil")
	}
}

// allZero reports whether every byte of b is zero.
func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func TestLoadUsableKeysZeroesKeyBytesOnMappingFailure(t *testing.T) {
	badKey := testKey(9)
	q := &fakeFieldKeyQuerier{usable: []coredb.FieldCryptoKey{{
		Version:  -1,
		KeyBytes: badKey,
	}}}
	adapter := &keyStoreAdapter{q: q}

	if _, err := adapter.LoadUsableKeys(context.Background()); err == nil {
		t.Fatal("LoadUsableKeys: expected an error for a negative version, got nil")
	}
	if !allZero(badKey) {
		t.Errorf("LoadUsableKeys: row KeyBytes = %x, want all zero after a mapping failure", badKey)
	}
}

func TestInsertInitialKeyZeroesKeyBytesOnMappingFailure(t *testing.T) {
	q := &fakeFieldKeyQuerier{insertRow: coredb.FieldCryptoKey{Version: -1}}
	adapter := &keyStoreAdapter{q: q}

	if _, err := adapter.InsertInitialKey(context.Background(), testKey(9)); err == nil {
		t.Fatal("InsertInitialKey: expected an error for a negative version, got nil")
	}
	if q.insertCalls != 1 {
		t.Fatalf("InsertInitialFieldCryptoKey called %d times, want 1", q.insertCalls)
	}
	if !allZero(q.lastInsertedRow.KeyBytes) {
		t.Errorf("InsertInitialKey: row KeyBytes = %x, want all zero after a mapping failure", q.lastInsertedRow.KeyBytes)
	}
}
