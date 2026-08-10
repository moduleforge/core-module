package fieldcrypto_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
)

// validHexKey is a 64-hex-char (32-byte) key, matching the "correct key
// succeeds" case in fieldcrypto_test.go.
const validHexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

// invalidHexKey is 64 characters but not valid hex.
const invalidHexKey = "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"

// fakeGetResult is one queued response for a call to
// fakeFieldKeyQuerier.GetFieldCryptoKey.
type fakeGetResult struct {
	key []byte
	err error
}

// fakeFieldKeyQuerier is a hand-written fake implementing
// fieldcrypto.FieldKeyQuerier (2 methods), giving each test full control
// over both the sequence of GetFieldCryptoKey responses and whether either
// method is ever called at all.
type fakeFieldKeyQuerier struct {
	t *testing.T

	// getSeq is consumed in order: the Nth call to GetFieldCryptoKey returns
	// getSeq[N]. Calling more times than entries exist is a test bug and
	// fails the test immediately.
	getSeq   []fakeGetResult
	getCalls int

	// insertFn, when set, is invoked by InsertFieldCryptoKeyIfAbsent and its
	// result returned directly.
	insertFn    func(keyBytes []byte) ([]byte, error)
	insertCalls int

	// failOnGet / failOnInsert make the respective method call t.Fatal if
	// invoked at all, for asserting a code path never touches the querier.
	failOnGet    bool
	failOnInsert bool
}

func (f *fakeFieldKeyQuerier) GetFieldCryptoKey(_ context.Context) ([]byte, error) {
	f.t.Helper()
	if f.failOnGet {
		f.t.Fatal("GetFieldCryptoKey should not have been called")
	}
	if f.getCalls >= len(f.getSeq) {
		f.t.Fatalf("GetFieldCryptoKey called more times (%d) than the test configured (%d)", f.getCalls+1, len(f.getSeq))
	}
	r := f.getSeq[f.getCalls]
	f.getCalls++
	return r.key, r.err
}

func (f *fakeFieldKeyQuerier) InsertFieldCryptoKeyIfAbsent(_ context.Context, keyBytes []byte) ([]byte, error) {
	f.t.Helper()
	f.insertCalls++
	if f.failOnInsert {
		f.t.Fatal("InsertFieldCryptoKeyIfAbsent should not have been called")
	}
	if f.insertFn == nil {
		f.t.Fatal("InsertFieldCryptoKeyIfAbsent called but the test configured no insertFn")
	}
	return f.insertFn(keyBytes)
}

// fixedWinnerKey is a distinct, fixed 32-byte key standing in for the key a
// concurrent winner persisted before we could.
func fixedWinnerKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 100)
	}
	return key
}

func TestNewFromEnvOrGenerate_EnvWins_DBNeverTouched(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	t.Setenv("CORE_FIELD_KEY_HEX", validHexKey)

	fake := &fakeFieldKeyQuerier{t: t, failOnGet: true, failOnInsert: true}

	c, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), fake)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}
	if c == nil {
		t.Fatal("NewFromEnvOrGenerate returned nil Cipher with valid env key")
	}
	if fake.getCalls != 0 || fake.insertCalls != 0 {
		t.Errorf("expected no DB calls, got getCalls=%d insertCalls=%d", fake.getCalls, fake.insertCalls)
	}
}

func TestNewFromEnvOrGenerate_EnvInvalid_DBNeverTouched(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	t.Setenv("CORE_FIELD_KEY_HEX", invalidHexKey)

	fake := &fakeFieldKeyQuerier{t: t, failOnGet: true, failOnInsert: true}

	_, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), fake)
	if err == nil {
		t.Error("expected error for invalid CORE_FIELD_KEY_HEX")
	}
	if fake.getCalls != 0 || fake.insertCalls != 0 {
		t.Errorf("expected no DB calls, got getCalls=%d insertCalls=%d", fake.getCalls, fake.insertCalls)
	}
}

func TestNewFromEnvOrGenerate_Absent_GenerateAndPersist(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	fake := &fakeFieldKeyQuerier{
		t:      t,
		getSeq: []fakeGetResult{{nil, pgx.ErrNoRows}},
		insertFn: func(keyBytes []byte) ([]byte, error) {
			// Echo back exactly what was inserted, like a real
			// ON CONFLICT DO NOTHING ... RETURNING on the winning row.
			return keyBytes, nil
		},
	}

	c, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), fake)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}
	if c == nil {
		t.Fatal("NewFromEnvOrGenerate returned nil Cipher")
	}

	const plaintext = "123-45-6789"
	blob, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := c.Decrypt(blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plaintext {
		t.Errorf("round trip = %q, want %q", got, plaintext)
	}

	if fake.getCalls != 1 || fake.insertCalls != 1 {
		t.Errorf("expected exactly one get and one insert, got getCalls=%d insertCalls=%d", fake.getCalls, fake.insertCalls)
	}
}

func TestNewFromEnvOrGenerate_Absent_InsertLosesRace(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	winnerKey := fixedWinnerKey()
	fake := &fakeFieldKeyQuerier{
		t: t,
		getSeq: []fakeGetResult{
			{nil, pgx.ErrNoRows}, // first read: nothing persisted yet
			{winnerKey, nil},     // re-fetch after lost race: the winner's key
		},
		insertFn: func(_ []byte) ([]byte, error) {
			// ON CONFLICT DO NOTHING skipped our row: another caller won.
			return nil, pgx.ErrNoRows
		},
	}

	c, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), fake)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}
	if c == nil {
		t.Fatal("NewFromEnvOrGenerate returned nil Cipher")
	}

	// Prove c was built from the winner's key, not our own random candidate:
	// a Cipher built directly from winnerKey must be able to decrypt what c
	// encrypted, and vice versa.
	winnerCipher, err := fieldcrypto.NewFromKey(winnerKey)
	if err != nil {
		t.Fatalf("NewFromKey(winnerKey): %v", err)
	}

	const plaintext = "probe-value"
	blob, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := winnerCipher.Decrypt(blob)
	if err != nil {
		t.Fatalf("winnerCipher.Decrypt(c's blob): %v (c was not built from the winner's key)", err)
	}
	if got != plaintext {
		t.Errorf("winnerCipher.Decrypt(c's blob) = %q, want %q", got, plaintext)
	}

	if fake.getCalls != 2 {
		t.Errorf("expected exactly two get calls (initial + re-fetch), got %d", fake.getCalls)
	}
	if fake.insertCalls != 1 {
		t.Errorf("expected exactly one insert call, got %d", fake.insertCalls)
	}
}

func TestNewFromEnvOrGenerate_Absent_PersistedKeyCorruptLength(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	for _, badLen := range []int{16, 40} {
		t.Run(strconv.Itoa(badLen)+"_bytes", func(t *testing.T) {
			fake := &fakeFieldKeyQuerier{
				t:            t,
				getSeq:       []fakeGetResult{{make([]byte, badLen), nil}},
				failOnInsert: true,
			}

			_, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), fake)
			if err == nil {
				t.Errorf("expected error for corrupt %d-byte persisted key", badLen)
			}
			if fake.insertCalls != 0 {
				t.Errorf("expected InsertFieldCryptoKeyIfAbsent never called, got %d calls", fake.insertCalls)
			}
		})
	}
}

func TestNewFromEnvOrGenerate_AbsentCheckErrors(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	connErr := errors.New("connection refused")
	fake := &fakeFieldKeyQuerier{
		t:            t,
		getSeq:       []fakeGetResult{{nil, connErr}},
		failOnInsert: true,
	}

	_, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), fake)
	if err == nil {
		t.Fatal("expected error when the absent-check read itself fails")
	}
	if !errors.Is(err, connErr) {
		t.Errorf("expected returned error to wrap the underlying read error, got: %v", err)
	}
	if fake.insertCalls != 0 {
		t.Errorf("expected InsertFieldCryptoKeyIfAbsent never called, got %d calls", fake.insertCalls)
	}
}

func TestNewFromEnvOrGenerate_InsertErrorsForOtherReason(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	insertErr := errors.New("insert failed: disk full")
	fake := &fakeFieldKeyQuerier{
		t:      t,
		getSeq: []fakeGetResult{{nil, pgx.ErrNoRows}}, // only one Get call is expected
		insertFn: func(_ []byte) ([]byte, error) {
			return nil, insertErr
		},
	}

	_, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), fake)
	if err == nil {
		t.Fatal("expected error when insert fails for a non-conflict reason")
	}
	if !errors.Is(err, insertErr) {
		t.Errorf("expected returned error to wrap the underlying insert error, got: %v", err)
	}
	if fake.getCalls != 1 {
		t.Errorf("expected exactly one get call (no re-fetch on a genuine insert failure), got %d", fake.getCalls)
	}
	if fake.insertCalls != 1 {
		t.Errorf("expected exactly one insert call, got %d", fake.insertCalls)
	}
}
