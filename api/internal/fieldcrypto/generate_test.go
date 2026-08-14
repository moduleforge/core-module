package fieldcrypto_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
)

// validHexKey is a 64-hex-char (32-byte) key.
const validHexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

// invalidHexKey is 64 characters but not valid hex.
const invalidHexKey = "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"

// errTestStoreUnreachable stands in for a key table that cannot be reached.
var errTestStoreUnreachable = errors.New("connection refused")

// ---------------------------------------------------------------------------
// Key-record helpers
// ---------------------------------------------------------------------------

// testKey returns a deterministic 32-byte key. Distinct seeds yield distinct
// keys — byte 0 is the seed itself — so a blob written under one never opens
// under another.
func testKey(seed byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = seed ^ byte(i)
	}
	return key
}

// activeRecord builds the single un-retired key record of a set.
func activeRecord(version uint32, key []byte) fieldcrypto.KeyRecord {
	return fieldcrypto.KeyRecord{Version: version, KeyBytes: append([]byte(nil), key...)}
}

// retiredRecord builds a retired, decrypt-only key record with no grace
// deadline and no compromise flag. Callers set DecryptableUntil or
// CompromisedAt on the result when they need one.
func retiredRecord(version uint32, key []byte) fieldcrypto.KeyRecord {
	retiredAt := time.Now().Add(-time.Hour)
	return fieldcrypto.KeyRecord{
		Version:   version,
		KeyBytes:  append([]byte(nil), key...),
		RetiredAt: &retiredAt,
	}
}

// copyRecords deep-copies records, key material and timestamps alike. The
// Cipher zeroes the key bytes it is handed once it has built its AEADs — the
// ownership rule KeyStore documents — so a fake that handed out its own
// template would hand out zeros on the next call.
func copyRecords(records []fieldcrypto.KeyRecord) []fieldcrypto.KeyRecord {
	if records == nil {
		return nil
	}
	out := make([]fieldcrypto.KeyRecord, 0, len(records))
	for _, rec := range records {
		out = append(out, fieldcrypto.KeyRecord{
			Version:          rec.Version,
			KeyBytes:         append([]byte(nil), rec.KeyBytes...),
			RetiredAt:        copyTimePtr(rec.RetiredAt),
			DecryptableUntil: copyTimePtr(rec.DecryptableUntil),
			CompromisedAt:    copyTimePtr(rec.CompromisedAt),
		})
	}
	return out
}

func copyTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

// ---------------------------------------------------------------------------
// Fake key store
// ---------------------------------------------------------------------------

// fakeLoadResult is one queued response to LoadUsableKeys.
type fakeLoadResult struct {
	records []fieldcrypto.KeyRecord
	err     error
}

// fakeKeyStore is a hand-written KeyStore fake giving each test control over
// the sequence of LoadUsableKeys responses, over InsertInitialKey's outcome,
// and over whether either method may be called at all. It is safe for
// concurrent use, since the -race exercise drives reloads from several
// goroutines at once.
type fakeKeyStore struct {
	t *testing.T

	mu sync.Mutex

	// loadSeq is consumed in order; the final entry repeats for every
	// subsequent call, so a test that only cares about a steady state
	// supplies one entry. Tests assert exact call counts via counts().
	loadSeq   []fakeLoadResult
	loadCalls int

	// insertFn, when set, is invoked by InsertInitialKey and its result
	// returned directly.
	insertFn    func(keyBytes []byte) (fieldcrypto.KeyRecord, error)
	insertCalls int

	// failOnLoad / failOnInsert assert that a code path never touches the
	// respective method.
	failOnLoad   bool
	failOnInsert bool
}

func (f *fakeKeyStore) LoadUsableKeys(_ context.Context) ([]fieldcrypto.KeyRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.loadCalls++
	if f.failOnLoad {
		f.t.Errorf("LoadUsableKeys should not have been called")
		return nil, errors.New("unexpected LoadUsableKeys call")
	}
	if len(f.loadSeq) == 0 {
		f.t.Errorf("LoadUsableKeys called but the test queued no responses")
		return nil, errors.New("unconfigured LoadUsableKeys call")
	}
	i := f.loadCalls - 1
	if i >= len(f.loadSeq) {
		i = len(f.loadSeq) - 1
	}
	return copyRecords(f.loadSeq[i].records), f.loadSeq[i].err
}

func (f *fakeKeyStore) InsertInitialKey(_ context.Context, keyBytes []byte) (fieldcrypto.KeyRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.insertCalls++
	if f.failOnInsert {
		f.t.Errorf("InsertInitialKey should not have been called")
		return fieldcrypto.KeyRecord{}, errors.New("unexpected InsertInitialKey call")
	}
	if f.insertFn == nil {
		f.t.Errorf("InsertInitialKey called but the test configured no insertFn")
		return fieldcrypto.KeyRecord{}, errors.New("unconfigured InsertInitialKey call")
	}
	return f.insertFn(keyBytes)
}

// counts reports how often each method has been called.
func (f *fakeKeyStore) counts() (loads, inserts int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loadCalls, f.insertCalls
}

// echoInsert is the insertFn a winning bootstrap uses: it returns the
// persisted row, echoing the key material back the way the guarded INSERT ...
// RETURNING does — as a fresh slice, per KeyStore's ownership rule.
func echoInsert(version uint32) func([]byte) (fieldcrypto.KeyRecord, error) {
	return func(keyBytes []byte) (fieldcrypto.KeyRecord, error) {
		return activeRecord(version, keyBytes), nil
	}
}

// ---------------------------------------------------------------------------
// Bootstrap
// ---------------------------------------------------------------------------

func TestNewFromEnvOrGenerate_NilStore(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	if _, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), nil); err == nil {
		t.Error("expected an error for a nil KeyStore")
	}
}

// TestNewFromEnvOrGenerate_EmptyTable_Generates covers first boot with no env
// seed: 32 random bytes are generated, persisted as version 1, and adopted.
func TestNewFromEnvOrGenerate_EmptyTable_Generates(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	var persisted []byte
	store := &fakeKeyStore{
		t:       t,
		loadSeq: []fakeLoadResult{{records: nil}},
		insertFn: func(keyBytes []byte) (fieldcrypto.KeyRecord, error) {
			persisted = append([]byte(nil), keyBytes...)
			return echoInsert(1)(keyBytes)
		},
	}

	c, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}
	if len(persisted) != 32 {
		t.Fatalf("persisted key is %d bytes, want 32", len(persisted))
	}
	if bytes.Equal(persisted, make([]byte, 32)) {
		t.Error("persisted key is all zero bytes; no material was generated")
	}

	const plaintext = "123-45-6789"
	blob, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if got := blobVersionOf(t, blob); got != 1 {
		t.Errorf("bootstrapped Cipher encrypts under version %d, want 1", got)
	}
	if got, err := openUnder(t, persisted, blob); err != nil || got != plaintext {
		t.Errorf("blob does not round-trip under the persisted key: (%q, %v)", got, err)
	}

	if loads, inserts := store.counts(); loads != 1 || inserts != 1 {
		t.Errorf("store calls = (loads %d, inserts %d), want (1, 1)", loads, inserts)
	}
}

// TestNewFromEnvOrGenerate_EmptyTable_SeedsFromEnv covers the one job
// CORE_FIELD_KEY_HEX still has: its decoded bytes are what first boot
// persists as version 1.
func TestNewFromEnvOrGenerate_EmptyTable_SeedsFromEnv(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	t.Setenv("CORE_FIELD_KEY_HEX", validHexKey)

	wantKey, err := hex.DecodeString(validHexKey)
	if err != nil {
		t.Fatalf("decode validHexKey: %v", err)
	}

	seeded := false
	store := &fakeKeyStore{
		t:       t,
		loadSeq: []fakeLoadResult{{records: nil}},
		insertFn: func(keyBytes []byte) (fieldcrypto.KeyRecord, error) {
			seeded = bytes.Equal(keyBytes, wantKey)
			return echoInsert(1)(keyBytes)
		},
	}

	c, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}
	if !seeded {
		t.Error("first boot did not persist the CORE_FIELD_KEY_HEX bytes as version 1")
	}

	const plaintext = "123-45-6789"
	blob, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if got, err := openUnder(t, wantKey, blob); err != nil || got != plaintext {
		t.Errorf("blob does not round-trip under the env key: (%q, %v)", got, err)
	}
}

// TestNewFromEnvOrGenerate_LostBootstrapRace covers the concurrent first-boot
// path: no rows back from the guarded insert means another caller established
// the key material, so this one adopts the winner rather than generating a
// second key.
func TestNewFromEnvOrGenerate_LostBootstrapRace(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	winnerKey := testKey(7)
	store := &fakeKeyStore{
		t: t,
		loadSeq: []fakeLoadResult{
			{records: nil}, // nothing persisted yet
			{records: []fieldcrypto.KeyRecord{activeRecord(4, winnerKey)}}, // the winner's row
		},
		insertFn: func(_ []byte) (fieldcrypto.KeyRecord, error) {
			return fieldcrypto.KeyRecord{}, pgx.ErrNoRows
		},
	}

	c, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}

	const plaintext = "probe-value"
	blob, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if got := blobVersionOf(t, blob); got != 4 {
		t.Errorf("adopted version %d, want the winner's version 4", got)
	}
	if got, err := openUnder(t, winnerKey, blob); err != nil || got != plaintext {
		t.Errorf("Cipher was not built from the winner's key: (%q, %v)", got, err)
	}

	if loads, inserts := store.counts(); loads != 2 || inserts != 1 {
		t.Errorf("store calls = (loads %d, inserts %d), want (2, 1)", loads, inserts)
	}
}

// TestNewFromEnvOrGenerate_LostRaceReloadStillEmpty pins the fail-hard rule:
// a re-load that still finds no usable key must never trigger a second
// generate, because minting a key over an existing (all-retired) table would
// strand every blob already written.
func TestNewFromEnvOrGenerate_LostRaceReloadStillEmpty(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	store := &fakeKeyStore{
		t:       t,
		loadSeq: []fakeLoadResult{{records: nil}, {records: nil}},
		insertFn: func(_ []byte) (fieldcrypto.KeyRecord, error) {
			return fieldcrypto.KeyRecord{}, pgx.ErrNoRows
		},
	}

	if _, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store); err == nil {
		t.Fatal("expected a hard error when the re-load finds no usable key")
	}
	if loads, inserts := store.counts(); loads != 2 || inserts != 1 {
		t.Errorf("store calls = (loads %d, inserts %d), want (2, 1): never a second generate", loads, inserts)
	}
}

func TestNewFromEnvOrGenerate_InsertFailsForAnotherReason(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	insertErr := errors.New("insert failed: disk full")
	store := &fakeKeyStore{
		t:       t,
		loadSeq: []fakeLoadResult{{records: nil}},
		insertFn: func(_ []byte) (fieldcrypto.KeyRecord, error) {
			return fieldcrypto.KeyRecord{}, insertErr
		},
	}

	_, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store)
	if err == nil {
		t.Fatal("expected an error when the insert fails for a non-conflict reason")
	}
	if !errors.Is(err, insertErr) {
		t.Errorf("returned error does not wrap the insert failure: %v", err)
	}
	if loads, _ := store.counts(); loads != 1 {
		t.Errorf("LoadUsableKeys called %d times, want 1: no re-fetch on a genuine insert failure", loads)
	}
}

func TestNewFromEnvOrGenerate_InitialLoadFails(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	store := &fakeKeyStore{
		t:            t,
		loadSeq:      []fakeLoadResult{{err: errTestStoreUnreachable}},
		failOnInsert: true,
	}

	_, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store)
	if err == nil {
		t.Fatal("expected an error when the initial load fails")
	}
	if !errors.Is(err, errTestStoreUnreachable) {
		t.Errorf("returned error does not wrap the load failure: %v", err)
	}
	if _, inserts := store.counts(); inserts != 0 {
		t.Errorf("InsertInitialKey called %d times, want 0", inserts)
	}
}

// ---------------------------------------------------------------------------
// CORE_FIELD_KEY_HEX precedence
// ---------------------------------------------------------------------------

// TestNewFromEnvOrGenerate_EnvMatchesActiveKey is the steady state for an
// operator who bootstrapped from the environment and never removed the
// variable: it proceeds silently and persists nothing.
func TestNewFromEnvOrGenerate_EnvMatchesActiveKey(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	t.Setenv("CORE_FIELD_KEY_HEX", validHexKey)

	activeKey, err := hex.DecodeString(validHexKey)
	if err != nil {
		t.Fatalf("decode validHexKey: %v", err)
	}

	store := &fakeKeyStore{
		t: t,
		loadSeq: []fakeLoadResult{{records: []fieldcrypto.KeyRecord{
			retiredRecord(1, testKey(1)),
			activeRecord(2, activeKey),
		}}},
		failOnInsert: true,
	}

	if _, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store); err != nil {
		t.Fatalf("NewFromEnvOrGenerate with a matching env key: %v", err)
	}
}

// TestNewFromEnvOrGenerate_EnvDiffersFromActiveKey is the deliberate,
// visible break from "env always wins": an operator who edits the variable
// expecting to rotate must find out that they have not.
func TestNewFromEnvOrGenerate_EnvDiffersFromActiveKey(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	t.Setenv("CORE_FIELD_KEY_HEX", validHexKey)

	store := &fakeKeyStore{
		t: t,
		loadSeq: []fakeLoadResult{{records: []fieldcrypto.KeyRecord{
			activeRecord(2, testKey(9)),
		}}},
		failOnInsert: true,
	}

	_, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store)
	if err == nil {
		t.Fatal("expected construction to fail when CORE_FIELD_KEY_HEX differs from the active key")
	}
	if !strings.Contains(err.Error(), "POST /v1/field-crypto-keys/rotations") {
		t.Errorf("error does not name the rotation endpoint as the correct action: %v", err)
	}
	if !strings.Contains(err.Error(), "CORE_FIELD_KEY_HEX") {
		t.Errorf("error does not name the offending variable: %v", err)
	}
	if strings.Contains(err.Error(), validHexKey) {
		t.Error("error text leaks the supplied key material")
	}
}

// TestNewFromEnvOrGenerate_EnvUnusable covers a variable that is set but
// cannot be a key. It must fail before the store is touched at all — an
// operator who supplied a key never has it silently ignored.
func TestNewFromEnvOrGenerate_EnvUnusable(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "non-hex", value: invalidHexKey},
		{name: "31 bytes", value: "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"},
		{name: "empty", value: ""},
		{name: "odd length", value: validHexKey + "0"},
		{name: "all zero bytes", value: strings.Repeat("0", 64)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetEnv(t, "CORE_FIELD_KEY_HEX")
			t.Setenv("CORE_FIELD_KEY_HEX", tc.value)

			store := &fakeKeyStore{t: t, failOnLoad: true, failOnInsert: true}
			if _, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store); err == nil {
				t.Fatal("expected an error for an unusable CORE_FIELD_KEY_HEX")
			}
			if loads, inserts := store.counts(); loads != 0 || inserts != 0 {
				t.Errorf("store calls = (loads %d, inserts %d), want (0, 0)", loads, inserts)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Key-set validation
// ---------------------------------------------------------------------------

// TestNewFromEnvOrGenerate_RejectsMalformedKeySet covers the boundary
// validation applied to whatever the store hands back. Each of these is a
// state the schema's constraints make unreachable, which is exactly why the
// cipher must refuse rather than half-adopt it.
func TestNewFromEnvOrGenerate_RejectsMalformedKeySet(t *testing.T) {
	corrupt := activeRecord(1, testKey(1))
	corrupt.KeyBytes = corrupt.KeyBytes[:16]

	compromisedAt := time.Now()
	activeCompromised := activeRecord(1, testKey(1))
	activeCompromised.CompromisedAt = &compromisedAt

	deadline := time.Now().Add(time.Hour)
	activeGraced := activeRecord(1, testKey(1))
	activeGraced.DecryptableUntil = &deadline

	tests := []struct {
		name    string
		records []fieldcrypto.KeyRecord
		wantIn  string
	}{
		{
			name:    "no active key",
			records: []fieldcrypto.KeyRecord{retiredRecord(1, testKey(1)), retiredRecord(2, testKey(2))},
			wantIn:  "no active key",
		},
		{
			name:    "two active keys",
			records: []fieldcrypto.KeyRecord{activeRecord(1, testKey(1)), activeRecord(2, testKey(2))},
			wantIn:  "two active keys",
		},
		{
			name:    "duplicate version",
			records: []fieldcrypto.KeyRecord{retiredRecord(1, testKey(1)), activeRecord(1, testKey(2))},
			wantIn:  "twice",
		},
		{
			name:    "version 0",
			records: []fieldcrypto.KeyRecord{activeRecord(0, testKey(1))},
			wantIn:  "version 0",
		},
		{
			name:    "key material of the wrong length",
			records: []fieldcrypto.KeyRecord{corrupt},
			wantIn:  "32 bytes",
		},
		{
			// What a wiped or half-scanned buffer looks like — including a
			// store that handed back a slice the Cipher had already zeroed.
			// Silently adopting it would mean encrypting under a key the key
			// table has never held.
			name:    "all-zero key material",
			records: []fieldcrypto.KeyRecord{{Version: 1, KeyBytes: make([]byte, 32)}},
			wantIn:  "all zero bytes",
		},
		{
			// The field_crypto_keys_retired_only_flags CHECK constraint
			// reserves compromised_at for a retired row; a KeyStore need not
			// be DB-backed, so buildKeySet re-checks it.
			name:    "active key with compromised_at set",
			records: []fieldcrypto.KeyRecord{activeCompromised},
			wantIn:  "compromised_at",
		},
		{
			// Same constraint, decryptable_until side.
			name:    "active key with decryptable_until set",
			records: []fieldcrypto.KeyRecord{activeGraced},
			wantIn:  "decryptable_until",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetEnv(t, "CORE_FIELD_KEY_HEX")
			store := &fakeKeyStore{
				t:            t,
				loadSeq:      []fakeLoadResult{{records: tc.records}},
				failOnInsert: true,
			}
			_, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store)
			if err == nil {
				t.Fatalf("expected an error for a key set with %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err, tc.wantIn)
			}
		})
	}
}

// TestKeyMaterialZeroedAfterLoad pins the ownership rule KeyStore documents:
// once the AEADs are built, the key bytes the store handed over are wiped.
func TestKeyMaterialZeroedAfterLoad(t *testing.T) {
	unsetEnv(t, "CORE_FIELD_KEY_HEX")

	handedOut := make(chan []byte, 1)
	store := &handoutKeyStore{records: []fieldcrypto.KeyRecord{activeRecord(1, testKey(1))}, handedOut: handedOut}

	if _, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store); err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}

	select {
	case key := <-handedOut:
		if !bytes.Equal(key, make([]byte, 32)) {
			t.Errorf("key material handed to the Cipher was not zeroed after use: %x", key)
		}
	default:
		t.Fatal("the store was never asked for keys")
	}
}

// handoutKeyStore reports the exact slice it handed to the Cipher, so a test
// can inspect it after construction.
type handoutKeyStore struct {
	records   []fieldcrypto.KeyRecord
	handedOut chan []byte
}

func (s *handoutKeyStore) LoadUsableKeys(_ context.Context) ([]fieldcrypto.KeyRecord, error) {
	out := copyRecords(s.records)
	select {
	case s.handedOut <- out[0].KeyBytes:
	default:
	}
	return out, nil
}

func (s *handoutKeyStore) InsertInitialKey(_ context.Context, _ []byte) (fieldcrypto.KeyRecord, error) {
	return fieldcrypto.KeyRecord{}, errors.New("handoutKeyStore does not bootstrap")
}
