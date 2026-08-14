package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
	"github.com/moduleforge/core-api/txhelper"
	coredb "github.com/moduleforge/core-model/db"
)

// rotationSSN is the plaintext every rotation test round-trips.
const rotationSSN = "123-45-6789"

// rotationEIN is the corporations.ein equivalent.
const rotationEIN = "12-3456789"

// --- key store and cipher helpers ---

// rotationTestKey returns a deterministic 32-byte key. fill must not be zero:
// fieldcrypto rejects an all-zero key as corrupt material rather than a key.
func rotationTestKey(fill byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	return key
}

// rotationKeyStore is a static fieldcrypto.KeyStore over a fixed record set.
type rotationKeyStore struct {
	records []fieldcrypto.KeyRecord
}

// LoadUsableKeys returns a fresh copy of every record on each call, as the
// KeyStore contract requires: the cipher zeroes the key material it is handed
// once it has built the AEADs from it.
func (s *rotationKeyStore) LoadUsableKeys(_ context.Context) ([]fieldcrypto.KeyRecord, error) {
	out := make([]fieldcrypto.KeyRecord, len(s.records))
	for i, rec := range s.records {
		out[i] = rec
		out[i].KeyBytes = append([]byte(nil), rec.KeyBytes...)
	}
	return out, nil
}

func (s *rotationKeyStore) InsertInitialKey(_ context.Context, _ []byte) (fieldcrypto.KeyRecord, error) {
	return fieldcrypto.KeyRecord{}, errors.New("rotationKeyStore: the test key set is never empty, so bootstrap must not run")
}

var _ fieldcrypto.KeyStore = (*rotationKeyStore)(nil)

// withoutFieldKeyEnv clears CORE_FIELD_KEY_HEX for the duration of the test so
// an inherited value cannot fail cipher construction against the test key set.
func withoutFieldKeyEnv(t *testing.T) {
	t.Helper()
	const name = "CORE_FIELD_KEY_HEX"
	prior, had := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("os.Unsetenv(%q): %v", name, err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(name, prior) //nolint:errcheck // best-effort restore in test cleanup
			return
		}
		os.Unsetenv(name) //nolint:errcheck
	})
}

// newRotationTestCipher builds a store-backed Cipher over records.
func newRotationTestCipher(t *testing.T, records ...fieldcrypto.KeyRecord) *fieldcrypto.Cipher {
	t.Helper()
	withoutFieldKeyEnv(t)
	c, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), &rotationKeyStore{records: records})
	if err != nil {
		t.Fatalf("newRotationTestCipher: %v", err)
	}
	return c
}

// rotationEncrypt encrypts plaintext under c's active key.
func rotationEncrypt(t *testing.T, c *fieldcrypto.Cipher, plaintext string) []byte {
	t.Helper()
	blob, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("rotationEncrypt: %v", err)
	}
	return blob
}

// --- querier fake ---

// rotationQuerier stubs only the four queries a rotation write-back can issue.
// Every other Querier method is left to the embedded nil interface, which
// panics if the write-back ever reaches for one — the point being that this
// path must touch nothing else.
type rotationQuerier struct {
	coredb.Querier

	ssnRows  int64
	ssnErr   error
	ssnSwaps []coredb.UpdateNaturalPersonSSNBlobParams

	einRows  int64
	einErr   error
	einSwaps []coredb.UpdateCorporationEINBlobParams

	storedSSN []byte
	storedEIN []byte
	reads     int
	readErr   error
}

func (q *rotationQuerier) UpdateNaturalPersonSSNBlob(_ context.Context, arg coredb.UpdateNaturalPersonSSNBlobParams) (int64, error) {
	q.ssnSwaps = append(q.ssnSwaps, arg)
	if q.ssnErr != nil {
		return 0, q.ssnErr
	}
	return q.ssnRows, nil
}

func (q *rotationQuerier) UpdateCorporationEINBlob(_ context.Context, arg coredb.UpdateCorporationEINBlobParams) (int64, error) {
	q.einSwaps = append(q.einSwaps, arg)
	if q.einErr != nil {
		return 0, q.einErr
	}
	return q.einRows, nil
}

func (q *rotationQuerier) GetNaturalPersonByEntityID(_ context.Context, entityID int64) (coredb.GetNaturalPersonByEntityIDRow, error) {
	q.reads++
	if q.readErr != nil {
		return coredb.GetNaturalPersonByEntityIDRow{}, q.readErr
	}
	return coredb.GetNaturalPersonByEntityIDRow{EntityID: entityID, Ssn: q.storedSSN}, nil
}

func (q *rotationQuerier) GetCorporationByEntityID(_ context.Context, entityID int64) (coredb.GetCorporationByEntityIDRow, error) {
	q.reads++
	if q.readErr != nil {
		return coredb.GetCorporationByEntityIDRow{}, q.readErr
	}
	return coredb.GetCorporationByEntityIDRow{EntityID: entityID, Ein: q.storedEIN}, nil
}

var _ coredb.Querier = (*rotationQuerier)(nil)

// --- log capture ---

type rotationLogRecord struct {
	level slog.Level
	attrs map[string]any
}

type rotationLogSink struct {
	records []rotationLogRecord
}

type rotationLogHandler struct {
	sink *rotationLogSink
}

func (h rotationLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h rotationLogHandler) Handle(_ context.Context, r slog.Record) error {
	rec := rotationLogRecord{level: r.Level, attrs: make(map[string]any, r.NumAttrs())}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.Any()
		return true
	})
	h.sink.records = append(h.sink.records, rec)
	return nil
}

func (h rotationLogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h rotationLogHandler) WithGroup(_ string) slog.Handler      { return h }

func newRotationLogger() (*slog.Logger, *rotationLogSink) {
	sink := &rotationLogSink{}
	return slog.New(rotationLogHandler{sink: sink}), sink
}

// attrUint reads a numeric log attribute as a uint64. slog normalizes the
// width of an integer attribute, so the assertion must not depend on the
// declared type of the value that was logged.
func attrUint(t *testing.T, rec rotationLogRecord, key string) uint64 {
	t.Helper()
	switch v := rec.attrs[key].(type) {
	case uint64:
		return v
	case uint32:
		return uint64(v)
	case int64:
		return uint64(v)
	default:
		t.Fatalf("log attribute %q is %T (%v), not a number", key, rec.attrs[key], rec.attrs[key])
		return 0
	}
}

// requireRotationLog asserts the fixture emitted exactly one rotate-on-read
// line, at the expected level and with the expected outcome and column, and
// returns it.
func requireRotationLog(t *testing.T, sink *rotationLogSink, level slog.Level, outcome, column string) rotationLogRecord {
	t.Helper()
	if len(sink.records) != 1 {
		t.Fatalf("expected exactly 1 rotate-on-read log line, got %d: %+v", len(sink.records), sink.records)
	}
	rec := sink.records[0]
	if rec.level != level {
		t.Errorf("log level = %v, want %v", rec.level, level)
	}
	if got := rec.attrs["event"]; got != rotateOnReadEvent {
		t.Errorf("event = %v, want %q", got, rotateOnReadEvent)
	}
	if got := rec.attrs["outcome"]; got != outcome {
		t.Errorf("outcome = %v, want %q", got, outcome)
	}
	if got := rec.attrs["column"]; got != column {
		t.Errorf("column = %v, want %q", got, column)
	}
	return rec
}

// --- fixture ---

const rotationEntityID int64 = 42

// rotationFixture wires a RotatingCipher whose cipher holds version 1 retired
// (optionally compromised) and version 2 active, over a stubbed querier and a
// capturing logger.
type rotationFixture struct {
	rc     *RotatingCipher
	cipher *fieldcrypto.Cipher
	q      *rotationQuerier
	sink   *rotationLogSink
	// ssnBlob and einBlob are stored values written under version 1.
	ssnBlob []byte
	einBlob []byte
}

func newRotationFixture(t *testing.T, compromised bool, db txhelper.DB) *rotationFixture {
	t.Helper()

	key1 := rotationTestKey(0x11)
	key2 := rotationTestKey(0x22)
	retiredAt := time.Now().Add(-2 * time.Hour)

	var compromisedAt *time.Time
	if compromised {
		at := retiredAt
		compromisedAt = &at
	}

	// A single-key cipher pinned to version 1 stands in for the process that
	// wrote these blobs before the rotation.
	v1 := newRotationTestCipher(t, fieldcrypto.KeyRecord{Version: 1, KeyBytes: key1})

	f := &rotationFixture{
		q:       &rotationQuerier{},
		ssnBlob: rotationEncrypt(t, v1, rotationSSN),
		einBlob: rotationEncrypt(t, v1, rotationEIN),
	}
	f.cipher = newRotationTestCipher(t,
		fieldcrypto.KeyRecord{Version: 1, KeyBytes: key1, RetiredAt: &retiredAt, CompromisedAt: compromisedAt},
		fieldcrypto.KeyRecord{Version: 2, KeyBytes: key2},
	)

	logger, sink := newRotationLogger()
	f.sink = sink
	f.rc = NewRotatingCipher(f.cipher, db, logger)
	f.rc.newQuerier = func(_ pgx.Tx) coredb.Querier { return f.q }
	return f
}

// --- tests ---

// A blob under a retired standard key whose compare-and-swap succeeds: the
// caller gets the right plaintext and the replacement is persisted under the
// active key, guarded by the exact bytes that were read.
func TestRotatingCipher_StandardKeyCASSucceeds(t *testing.T) {
	f := newRotationFixture(t, false, newFakeDB())
	f.q.ssnRows = 1

	got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
	if err != nil {
		t.Fatalf("DecryptSSN: unexpected error: %v", err)
	}
	if got != rotationSSN {
		t.Errorf("plaintext = %q, want %q", got, rotationSSN)
	}

	if len(f.q.ssnSwaps) != 1 {
		t.Fatalf("expected exactly 1 compare-and-swap, got %d", len(f.q.ssnSwaps))
	}
	swap := f.q.ssnSwaps[0]
	if swap.EntityID != rotationEntityID {
		t.Errorf("swap entity id = %d, want %d", swap.EntityID, rotationEntityID)
	}
	if string(swap.OldSsn) != string(f.ssnBlob) {
		t.Error("swap guard is not the exact blob that was read")
	}
	if version, verr := fieldcrypto.BlobVersion(swap.NewSsn); verr != nil || version != 2 {
		t.Errorf("replacement blob version = %d (err %v), want 2", version, verr)
	}
	// The replacement must decrypt to the same plaintext under the active key.
	if round, rerr := f.cipher.Decrypt(context.Background(), swap.NewSsn); rerr != nil || round != rotationSSN {
		t.Errorf("replacement decrypts to %q (err %v), want %q", round, rerr, rotationSSN)
	}
	if f.q.reads != 0 {
		t.Errorf("a succeeding compare-and-swap issued %d verification read(s), want 0", f.q.reads)
	}

	rec := requireRotationLog(t, f.sink, slog.LevelDebug, outcomePersisted, "natural_persons.ssn")
	if from, to := attrUint(t, rec, "from_version"), attrUint(t, rec, "to_version"); from != 1 || to != 2 {
		t.Errorf("log versions = %d -> %d, want 1 -> 2", from, to)
	}
	if rec.attrs["compromised"] != false {
		t.Errorf("compromised = %v, want false", rec.attrs["compromised"])
	}
	if got := attrUint(t, rec, "entity_id"); got != uint64(rotationEntityID) {
		t.Errorf("entity_id = %d, want %d", got, rotationEntityID)
	}
}

// A blob under a compromised key whose compare-and-swap loses, with the stored
// blob still carrying the compromised version: the read must fail, and the
// error must name the column and the source key version.
func TestRotatingCipher_CompromisedKeyLostCASStillCompromisedFailsRead(t *testing.T) {
	f := newRotationFixture(t, true, newFakeDB())
	f.q.ssnRows = 0
	// Whatever is on disk is still under version 1.
	f.q.storedSSN = f.ssnBlob

	got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
	if err == nil {
		t.Fatal("expected the read to fail when a compromised-key rotation cannot be persisted")
	}
	if got != "" {
		t.Errorf("plaintext = %q, want %q on a failed read", got, "")
	}
	if !errors.Is(err, errStillCompromised) {
		t.Errorf("error does not wrap errStillCompromised: %v", err)
	}
	if !strings.Contains(err.Error(), "natural_persons.ssn") {
		t.Errorf("error does not name the column: %v", err)
	}
	if !strings.Contains(err.Error(), "compromised key version 1") {
		t.Errorf("error does not name the source key version: %v", err)
	}
	if f.q.reads != 1 {
		t.Errorf("verification issued %d read(s), want 1", f.q.reads)
	}

	rec := requireRotationLog(t, f.sink, slog.LevelError, outcomeError, "natural_persons.ssn")
	if rec.attrs["compromised"] != true {
		t.Errorf("compromised = %v, want true", rec.attrs["compromised"])
	}
}

// A blob under a compromised key whose compare-and-swap loses to a blob that
// has already been re-encrypted under the active key: nothing under the
// compromised version remains, so the read must succeed.
func TestRotatingCipher_CompromisedKeyLostCASAlreadyRotatedSucceeds(t *testing.T) {
	f := newRotationFixture(t, true, newFakeDB())
	f.q.ssnRows = 0
	// The winner of the race already stored a version-2 blob.
	f.q.storedSSN = rotationEncrypt(t, f.cipher, rotationSSN)

	got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
	if err != nil {
		t.Fatalf("DecryptSSN: unexpected error: %v", err)
	}
	if got != rotationSSN {
		t.Errorf("plaintext = %q, want %q", got, rotationSSN)
	}
	if f.q.reads != 1 {
		t.Errorf("verification issued %d read(s), want 1", f.q.reads)
	}
	requireRotationLog(t, f.sink, slog.LevelDebug, outcomePersisted, "natural_persons.ssn")
}

// A blob under a compromised key whose compare-and-swap loses to a blob
// stored under a third, retired, also-compromised version — neither
// FromVersion nor the active ToVersion. verifyStale must not treat "version
// differs from FromVersion" as sufficient evidence of a benign race: only a
// stored version equal to ToVersion is verifiably safe, so this read must
// fail rather than be reported as already rotated.
func TestRotatingCipher_CompromisedKeyLostCASDifferentCompromisedVersionFailsRead(t *testing.T) {
	f := newRotationFixture(t, true, newFakeDB())
	f.q.ssnRows = 0
	// A second retired, compromised key version (3), distinct from both
	// FromVersion (1) and the active ToVersion (2). Its own cipher only needs
	// to exist long enough to produce a blob carrying that version prefix;
	// verifyStale decodes the version without decrypting.
	v3 := newRotationTestCipher(t, fieldcrypto.KeyRecord{Version: 3, KeyBytes: rotationTestKey(0x33)})
	f.q.storedSSN = rotationEncrypt(t, v3, rotationSSN)

	got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
	if err == nil {
		t.Fatal("expected the read to fail: the stored blob is under a second compromised version, not the active one")
	}
	if got != "" {
		t.Errorf("plaintext = %q, want %q on a failed read", got, "")
	}
	if !errors.Is(err, errStillCompromised) {
		t.Errorf("error does not wrap errStillCompromised: %v", err)
	}
	if f.q.reads != 1 {
		t.Errorf("verification issued %d read(s), want 1", f.q.reads)
	}

	rec := requireRotationLog(t, f.sink, slog.LevelError, outcomeError, "natural_persons.ssn")
	if rec.attrs["compromised"] != true {
		t.Errorf("compromised = %v, want true", rec.attrs["compromised"])
	}
}

// The same lost compare-and-swap under a compromised key, but the stored value
// was cleared outright: nothing remains under the old key, so the read
// succeeds.
func TestRotatingCipher_CompromisedKeyLostCASClearedValueSucceeds(t *testing.T) {
	f := newRotationFixture(t, true, newFakeDB())
	f.q.ssnRows = 0
	f.q.storedSSN = nil

	got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
	if err != nil {
		t.Fatalf("DecryptSSN: unexpected error: %v", err)
	}
	if got != rotationSSN {
		t.Errorf("plaintext = %q, want %q", got, rotationSSN)
	}
	requireRotationLog(t, f.sink, slog.LevelDebug, outcomePersisted, "natural_persons.ssn")
}

// A read under a standard key whose write-back errors outright — a read-only
// replica, a permission error, a lock timeout — must still succeed, logging
// the tolerated miss at warn level.
func TestRotatingCipher_StandardKeyWriteBackErrorSucceeds(t *testing.T) {
	f := newRotationFixture(t, false, newFakeDB())
	writeErr := errors.New("permission denied for table natural_persons")
	f.q.ssnErr = writeErr

	got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
	if err != nil {
		t.Fatalf("DecryptSSN: a standard-rotation write-back failure must not fail the read: %v", err)
	}
	if got != rotationSSN {
		t.Errorf("plaintext = %q, want %q", got, rotationSSN)
	}

	rec := requireRotationLog(t, f.sink, slog.LevelWarn, outcomeStale, "natural_persons.ssn")
	logged, _ := rec.attrs["error"].(error)
	if !errors.Is(logged, writeErr) {
		t.Errorf("logged error = %v, want it to wrap %v", rec.attrs["error"], writeErr)
	}
}

// A lost compare-and-swap under a standard key is a benign skip: the read
// succeeds and costs no verification query.
func TestRotatingCipher_StandardKeyLostCASSkipsWithoutExtraQuery(t *testing.T) {
	f := newRotationFixture(t, false, newFakeDB())
	f.q.ssnRows = 0
	// Set so that a verification read, if one were wrongly issued, would look
	// like a genuine failure rather than passing by accident.
	f.q.storedSSN = f.ssnBlob

	got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
	if err != nil {
		t.Fatalf("DecryptSSN: unexpected error: %v", err)
	}
	if got != rotationSSN {
		t.Errorf("plaintext = %q, want %q", got, rotationSSN)
	}
	if f.q.reads != 0 {
		t.Errorf("a standard-key lost compare-and-swap issued %d verification read(s), want 0", f.q.reads)
	}

	rec := requireRotationLog(t, f.sink, slog.LevelWarn, outcomeStale, "natural_persons.ssn")
	logged, _ := rec.attrs["error"].(error)
	if !errors.Is(logged, errStaleCAS) {
		t.Errorf("logged error = %v, want it to wrap errStaleCAS", rec.attrs["error"])
	}
}

// With no write handle at all the write-back cannot even be attempted: a
// standard rotation tolerates that, a compromised one must not.
func TestRotatingCipher_NilWriteHandle(t *testing.T) {
	t.Run("standard key succeeds", func(t *testing.T) {
		f := newRotationFixture(t, false, nil)

		got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
		if err != nil {
			t.Fatalf("DecryptSSN: unexpected error: %v", err)
		}
		if got != rotationSSN {
			t.Errorf("plaintext = %q, want %q", got, rotationSSN)
		}
		if len(f.q.ssnSwaps) != 0 || f.q.reads != 0 {
			t.Error("a nil write handle must issue no query at all")
		}
		rec := requireRotationLog(t, f.sink, slog.LevelWarn, outcomeStale, "natural_persons.ssn")
		logged, _ := rec.attrs["error"].(error)
		if !errors.Is(logged, errNoWriteHandle) {
			t.Errorf("logged error = %v, want it to wrap errNoWriteHandle", rec.attrs["error"])
		}
	})

	t.Run("compromised key fails", func(t *testing.T) {
		f := newRotationFixture(t, true, nil)

		got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, f.ssnBlob)
		if err == nil {
			t.Fatal("expected the read to fail: a compromised-key rotation cannot be persisted with no write handle")
		}
		if got != "" {
			t.Errorf("plaintext = %q, want %q on a failed read", got, "")
		}
		if !errors.Is(err, errNoWriteHandle) {
			t.Errorf("error does not wrap errNoWriteHandle: %v", err)
		}
		if !strings.Contains(err.Error(), "natural_persons.ssn") || !strings.Contains(err.Error(), "compromised key version 1") {
			t.Errorf("error does not name the column and source key version: %v", err)
		}
		requireRotationLog(t, f.sink, slog.LevelError, outcomeError, "natural_persons.ssn")
	})
}

// The EIN column runs the identical core over its own descriptor.
func TestRotatingCipher_DecryptEINRotatesCorporationsColumn(t *testing.T) {
	f := newRotationFixture(t, false, newFakeDB())
	f.q.einRows = 1

	got, err := f.rc.DecryptEIN(context.Background(), rotationEntityID, f.einBlob)
	if err != nil {
		t.Fatalf("DecryptEIN: unexpected error: %v", err)
	}
	if got != rotationEIN {
		t.Errorf("plaintext = %q, want %q", got, rotationEIN)
	}
	if len(f.q.einSwaps) != 1 {
		t.Fatalf("expected exactly 1 compare-and-swap, got %d", len(f.q.einSwaps))
	}
	if string(f.q.einSwaps[0].OldEin) != string(f.einBlob) {
		t.Error("swap guard is not the exact blob that was read")
	}
	if version, verr := fieldcrypto.BlobVersion(f.q.einSwaps[0].NewEin); verr != nil || version != 2 {
		t.Errorf("replacement blob version = %d (err %v), want 2", version, verr)
	}
	requireRotationLog(t, f.sink, slog.LevelDebug, outcomePersisted, "corporations.ein")
}

// A compromised-key read of the EIN column whose compare-and-swap loses to a
// blob still under the compromised version fails, naming its own column.
func TestRotatingCipher_CompromisedKeyEINLostCASFailsRead(t *testing.T) {
	f := newRotationFixture(t, true, newFakeDB())
	f.q.einRows = 0
	f.q.storedEIN = f.einBlob

	if _, err := f.rc.DecryptEIN(context.Background(), rotationEntityID, f.einBlob); err == nil {
		t.Fatal("expected the read to fail")
	} else if !strings.Contains(err.Error(), "corporations.ein") {
		t.Errorf("error does not name the column: %v", err)
	}
	requireRotationLog(t, f.sink, slog.LevelError, outcomeError, "corporations.ein")
}

// A blob already under the active key needs no write-back and emits no log
// line; an absent value decrypts to "" without touching the database.
func TestRotatingCipher_NoRotationNeededTouchesNothing(t *testing.T) {
	f := newRotationFixture(t, false, newFakeDB())
	current := rotationEncrypt(t, f.cipher, rotationSSN)

	got, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, current)
	if err != nil {
		t.Fatalf("DecryptSSN: unexpected error: %v", err)
	}
	if got != rotationSSN {
		t.Errorf("plaintext = %q, want %q", got, rotationSSN)
	}

	absent, err := f.rc.DecryptSSN(context.Background(), rotationEntityID, nil)
	if err != nil {
		t.Fatalf("DecryptSSN(nil): unexpected error: %v", err)
	}
	if absent != "" {
		t.Errorf("absent value = %q, want %q", absent, "")
	}

	if len(f.q.ssnSwaps) != 0 || f.q.reads != 0 {
		t.Error("a blob already under the active key must issue no query")
	}
	if len(f.sink.records) != 0 {
		t.Errorf("expected no log lines, got %+v", f.sink.records)
	}
}

// Encrypt forwards to the underlying cipher, which always uses the active key.
func TestRotatingCipher_EncryptUsesActiveKey(t *testing.T) {
	f := newRotationFixture(t, false, newFakeDB())

	blob, err := f.rc.Encrypt(context.Background(), rotationSSN)
	if err != nil {
		t.Fatalf("Encrypt: unexpected error: %v", err)
	}
	version, err := fieldcrypto.BlobVersion(blob)
	if err != nil {
		t.Fatalf("BlobVersion: %v", err)
	}
	if version != 2 {
		t.Errorf("encrypted under version %d, want the active version 2", version)
	}
	if len(f.q.ssnSwaps) != 0 {
		t.Error("Encrypt must not write back")
	}
}
