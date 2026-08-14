package fieldcrypto_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
)

// Wire-format geometry, restated here so the tests assert against the format
// the design fixed rather than against constants the implementation could
// change underneath them.
const (
	versionSize = 4
	nonceSize   = 12
	tagSize     = 16
	minBlobSize = versionSize + nonceSize + tagSize
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// newStoreCipher constructs a store-backed Cipher with CORE_FIELD_KEY_HEX out
// of the picture, which is the shape every key-set test wants.
func newStoreCipher(t *testing.T, store fieldcrypto.KeyStore) *fieldcrypto.Cipher {
	t.Helper()
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	c, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), store)
	if err != nil {
		t.Fatalf("NewFromEnvOrGenerate: %v", err)
	}
	return c
}

// newStaticCipher builds the store-less, version-1 Cipher.
func newStaticCipher(t *testing.T) *fieldcrypto.Cipher {
	t.Helper()
	c, err := fieldcrypto.NewFromKey(testKey(1))
	if err != nil {
		t.Fatalf("NewFromKey: %v", err)
	}
	return c
}

// sealUnder builds a blob the way the wire format specifies, independently of
// the code under test: version(4) || nonce(12) || ciphertext || tag(16), with
// the four version bytes passed verbatim as the AEAD's additional
// authenticated data. A test that round-trips through this helper is asserting
// the format, not merely that the implementation agrees with itself.
func sealUnder(t *testing.T, key []byte, version uint32, plaintext string) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	out := make([]byte, versionSize+nonceSize)
	binary.BigEndian.PutUint32(out[:versionSize], version)
	if _, err := rand.Read(out[versionSize : versionSize+nonceSize]); err != nil {
		t.Fatalf("read nonce: %v", err)
	}
	return aead.Seal(out, out[versionSize:versionSize+nonceSize], []byte(plaintext), out[:versionSize])
}

// openUnder is sealUnder's inverse, for asserting that a blob the Cipher
// produced is readable by an independent implementation of the format.
func openUnder(t *testing.T, key []byte, blob []byte) (string, error) {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	plaintext, err := aead.Open(nil,
		blob[versionSize:versionSize+nonceSize],
		blob[versionSize+nonceSize:],
		blob[:versionSize])
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// blobVersionOf decodes a blob's version prefix without any validation, so a
// test can assert on a prefix the package itself would reject.
func blobVersionOf(t *testing.T, blob []byte) uint32 {
	t.Helper()
	if len(blob) < versionSize {
		t.Fatalf("blob is %d bytes, too short to carry a version prefix", len(blob))
	}
	return binary.BigEndian.Uint32(blob[:versionSize])
}

// withVersionPrefix returns a copy of blob whose version prefix has been
// rewritten, leaving nonce, ciphertext, and tag untouched.
func withVersionPrefix(blob []byte, version uint32) []byte {
	out := append([]byte(nil), blob...)
	binary.BigEndian.PutUint32(out[:versionSize], version)
	return out
}

// isZeroRotation reports whether rot is the "nothing to do" zero value.
// Rotation carries a []byte, so it cannot be compared with ==.
func isZeroRotation(rot fieldcrypto.Rotation) bool {
	return rot.FromVersion == 0 && rot.ToVersion == 0 && len(rot.Blob) == 0 && !rot.MustPersist
}

// captureDefaultLogger redirects slog's default logger into a buffer for the
// duration of the test.
func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prior) })
	return buf
}

// ---------------------------------------------------------------------------
// Wire format
// ---------------------------------------------------------------------------

// TestEncryptBlobLayout asserts the layout contract directly: a blob is at
// least the 32-byte minimum, its first four bytes decode to the active
// version, and an independent AES-GCM implementation can open it using those
// same four bytes as the AAD.
func TestEncryptBlobLayout(t *testing.T) {
	key3 := testKey(3)
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{{records: []fieldcrypto.KeyRecord{
		retiredRecord(1, testKey(1)),
		retiredRecord(2, testKey(2)),
		activeRecord(3, key3),
	}}}}
	c := newStoreCipher(t, store)

	const plaintext = "123-45-6789"
	blob, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if len(blob) < minBlobSize {
		t.Errorf("blob is %d bytes, want at least %d", len(blob), minBlobSize)
	}
	if want := minBlobSize + len(plaintext); len(blob) != want {
		t.Errorf("blob is %d bytes, want exactly %d (version+nonce+plaintext+tag)", len(blob), want)
	}
	if got := blobVersionOf(t, blob); got != 3 {
		t.Errorf("blob version prefix = %d, want the active version 3", got)
	}
	if got, err := fieldcrypto.BlobVersion(blob); err != nil || got != 3 {
		t.Errorf("BlobVersion = (%d, %v), want (3, nil)", got, err)
	}
	got, err := openUnder(t, key3, blob)
	if err != nil {
		t.Fatalf("independent Open of the Cipher's blob: %v", err)
	}
	if got != plaintext {
		t.Errorf("independent Open = %q, want %q", got, plaintext)
	}
}

// TestBlobVersion covers the standalone decoder's contract, including the two
// malformed shapes it must reject.
func TestBlobVersion(t *testing.T) {
	valid := sealUnder(t, testKey(1), 7, "x")

	tests := []struct {
		name    string
		blob    []byte
		want    uint32
		wantErr bool
	}{
		{name: "valid", blob: valid, want: 7},
		{name: "nil", blob: nil, wantErr: true},
		{name: "empty", blob: []byte{}, wantErr: true},
		{name: "one byte short of the minimum", blob: make([]byte, minBlobSize-1), wantErr: true},
		{name: "exactly the minimum but version 0", blob: make([]byte, minBlobSize), wantErr: true},
		{name: "version 0 on an otherwise valid blob", blob: withVersionPrefix(valid, 0), wantErr: true},
		{name: "max version", blob: withVersionPrefix(valid, ^uint32(0)), want: ^uint32(0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fieldcrypto.BlobVersion(tc.blob)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("BlobVersion(%d bytes) = %d, want an error", len(tc.blob), got)
				}
				return
			}
			if err != nil {
				t.Fatalf("BlobVersion: %v", err)
			}
			if got != tc.want {
				t.Errorf("BlobVersion = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestMalformedBlobRejectedWithoutReload proves that a malformed blob fails
// immediately and, critically, costs no database query — the property that
// stops corrupt or hostile blobs from amplifying into a query storm.
func TestMalformedBlobRejectedWithoutReload(t *testing.T) {
	valid := sealUnder(t, testKey(1), 1, "123-45-6789")

	tests := []struct {
		name string
		blob []byte
	}{
		{name: "one byte short of the minimum", blob: valid[:minBlobSize-1]},
		{name: "version 0", blob: withVersionPrefix(valid, 0)},
		{name: "all zero bytes at the minimum length", blob: make([]byte, minBlobSize)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
				{records: []fieldcrypto.KeyRecord{activeRecord(1, testKey(1))}},
			}}
			c := newStoreCipher(t, store)
			// Reloads are permitted; the point is that none is attempted.
			fieldcrypto.SetReloadTuningForTest(c, time.Hour, 0)

			got, err := c.Decrypt(context.Background(), tc.blob)
			if err == nil {
				t.Fatalf("Decrypt(%s) = %q, want an error", tc.name, got)
			}
			if got != "" {
				t.Errorf("Decrypt returned plaintext %q alongside its error; failures must never yield plaintext", got)
			}
			if loads, _ := store.counts(); loads != 1 {
				t.Errorf("LoadUsableKeys called %d times, want 1 (construction only, no reload on a malformed blob)", loads)
			}
		})
	}
}

// TestVersionPrefixIsAuthenticated proves the AAD binding actually works.
//
// Two versions are deliberately given identical key material — impossible in
// the real table, which has a UNIQUE constraint on key_bytes — so that the
// only thing distinguishing a decrypt under version 1 from a decrypt under
// version 2 is the four AAD bytes. Rewriting the prefix must therefore fail
// authentication rather than quietly decrypt.
func TestVersionPrefixIsAuthenticated(t *testing.T) {
	shared := testKey(9)
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{{records: []fieldcrypto.KeyRecord{
		retiredRecord(1, shared),
		activeRecord(2, shared),
	}}}}
	c := newStoreCipher(t, store)

	const plaintext = "123-45-6789"
	blob := sealUnder(t, shared, 1, plaintext)

	got, err := c.Decrypt(context.Background(), blob)
	if err != nil {
		t.Fatalf("Decrypt of an untampered version-1 blob: %v", err)
	}
	if got != plaintext {
		t.Fatalf("Decrypt = %q, want %q", got, plaintext)
	}

	if _, err := c.Decrypt(context.Background(), withVersionPrefix(blob, 2)); err == nil {
		t.Error("Decrypt accepted a blob whose version prefix was rewritten to another version holding the same key material; the prefix is not bound as AAD")
	}
}

// TestTamperedCiphertextFails covers the ordinary authentication guarantee.
func TestTamperedCiphertextFails(t *testing.T) {
	c := newStaticCipher(t)

	blob, err := c.Encrypt(context.Background(), "123-45-6789")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	for _, at := range []struct {
		name  string
		index int
	}{
		{name: "tag", index: len(blob) - 1},
		{name: "ciphertext", index: minBlobSize - tagSize},
		{name: "nonce", index: versionSize},
	} {
		t.Run(at.name, func(t *testing.T) {
			tampered := append([]byte(nil), blob...)
			tampered[at.index] ^= 0xFF
			if got, err := c.Decrypt(context.Background(), tampered); err == nil {
				t.Errorf("Decrypt accepted a blob tampered in the %s: got %q", at.name, got)
			}
		})
	}
}

// TestEmptyInput verifies the empty-value contract on all three methods.
func TestEmptyInput(t *testing.T) {
	c := newStaticCipher(t)
	ctx := context.Background()

	blob, err := c.Encrypt(ctx, "")
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	if len(blob) != 0 {
		t.Errorf("Encrypt(\"\") returned a %d-byte blob, want 0 (no version prefix)", len(blob))
	}

	for _, empty := range [][]byte{nil, {}} {
		got, err := c.Decrypt(ctx, empty)
		if err != nil {
			t.Fatalf("Decrypt(%v): %v", empty, err)
		}
		if got != "" {
			t.Errorf("Decrypt(%v) = %q, want the empty string", empty, got)
		}

		got, rot, err := c.DecryptWithRotation(ctx, empty)
		if err != nil {
			t.Fatalf("DecryptWithRotation(%v): %v", empty, err)
		}
		if got != "" || !isZeroRotation(rot) || rot.Needed() {
			t.Errorf("DecryptWithRotation(%v) = (%q, %+v), want (\"\", zero Rotation)", empty, got, rot)
		}
	}
}

// TestNonceUniqueness confirms two encryptions of the same plaintext produce
// distinct blobs.
func TestNonceUniqueness(t *testing.T) {
	c := newStaticCipher(t)
	const plaintext = "123-45-6789"

	first, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	second, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Error("two encryptions of the same plaintext produced identical blobs (nonce reuse)")
	}
}

// ---------------------------------------------------------------------------
// Multi-key behavior
// ---------------------------------------------------------------------------

// TestDecryptsEveryUsableVersion round-trips a blob written under each loaded
// key, active and retired alike.
func TestDecryptsEveryUsableVersion(t *testing.T) {
	keys := map[uint32][]byte{1: testKey(1), 2: testKey(2), 3: testKey(3)}
	compromised := time.Now().Add(-30 * time.Minute)
	retiredCompromised := retiredRecord(2, keys[2])
	retiredCompromised.CompromisedAt = &compromised

	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{{records: []fieldcrypto.KeyRecord{
		retiredRecord(1, keys[1]),
		retiredCompromised,
		activeRecord(3, keys[3]),
	}}}}
	c := newStoreCipher(t, store)

	for version, key := range keys {
		plaintext := "value-under-version-" + string(rune('0'+version))
		got, err := c.Decrypt(context.Background(), sealUnder(t, key, version, plaintext))
		if err != nil {
			t.Errorf("Decrypt of a version-%d blob: %v", version, err)
			continue
		}
		if got != plaintext {
			t.Errorf("Decrypt of a version-%d blob = %q, want %q", version, got, plaintext)
		}
	}

	if loads, _ := store.counts(); loads != 1 {
		t.Errorf("LoadUsableKeys called %d times, want 1: every version was already held", loads)
	}
}

// TestEncryptAlwaysSelectsActiveKey proves encryption never reaches for a
// retired key, however many are loaded.
func TestEncryptAlwaysSelectsActiveKey(t *testing.T) {
	key1, key3 := testKey(1), testKey(3)
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{{records: []fieldcrypto.KeyRecord{
		retiredRecord(1, key1),
		retiredRecord(2, testKey(2)),
		activeRecord(3, key3),
	}}}}
	c := newStoreCipher(t, store)

	for i := 0; i < 16; i++ {
		blob, err := c.Encrypt(context.Background(), "123-45-6789")
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		if got := blobVersionOf(t, blob); got != 3 {
			t.Fatalf("Encrypt produced version %d, want the active version 3", got)
		}
		if _, err := openUnder(t, key1, withVersionPrefix(blob, 1)); err == nil {
			t.Fatal("a blob Encrypt produced opened under a retired key")
		}
	}
}

// TestUnknownVersionReloadsOnceThenFails covers the mandatory
// reload-on-unknown-version path in both outcomes: the reload that finds the
// version, and the reload that does not and must then fail loudly.
func TestUnknownVersionReloadsOnceThenFails(t *testing.T) {
	key1, key2 := testKey(1), testKey(2)

	t.Run("reload finds the version", func(t *testing.T) {
		store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
			{records: []fieldcrypto.KeyRecord{activeRecord(1, key1)}},
			{records: []fieldcrypto.KeyRecord{retiredRecord(1, key1), activeRecord(2, key2)}},
		}}
		c := newStoreCipher(t, store)
		fieldcrypto.SetReloadTuningForTest(c, time.Hour, 0)

		const plaintext = "written-by-a-peer-after-rotating"
		got, err := c.Decrypt(context.Background(), sealUnder(t, key2, 2, plaintext))
		if err != nil {
			t.Fatalf("Decrypt of a peer's post-rotation blob: %v", err)
		}
		if got != plaintext {
			t.Errorf("Decrypt = %q, want %q", got, plaintext)
		}
		if loads, _ := store.counts(); loads != 2 {
			t.Errorf("LoadUsableKeys called %d times, want 2 (construction + one reload)", loads)
		}
	})

	t.Run("reload does not find the version", func(t *testing.T) {
		store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
			{records: []fieldcrypto.KeyRecord{activeRecord(1, key1)}},
		}}
		c := newStoreCipher(t, store)
		fieldcrypto.SetReloadTuningForTest(c, time.Hour, 0)

		got, err := c.Decrypt(context.Background(), sealUnder(t, testKey(42), 42, "unreadable"))
		if err == nil {
			t.Fatalf("Decrypt of an unknown version = %q, want an error", got)
		}
		if got != "" {
			t.Errorf("Decrypt returned plaintext %q alongside its error", got)
		}
		if !strings.Contains(err.Error(), "unknown key version 42") {
			t.Errorf("error does not name the offending version: %v", err)
		}
		if loads, _ := store.counts(); loads != 2 {
			t.Errorf("LoadUsableKeys called %d times, want exactly 2 (construction + one reload, then failure)", loads)
		}
	})
}

// TestUnknownVersionReloadIsRateLimited proves a stream of blobs naming an
// unknown version cannot turn every read into a database query.
func TestUnknownVersionReloadIsRateLimited(t *testing.T) {
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
		{records: []fieldcrypto.KeyRecord{activeRecord(1, testKey(1))}},
	}}
	c := newStoreCipher(t, store)
	fieldcrypto.SetReloadTuningForTest(c, time.Hour, time.Hour)

	hostile := sealUnder(t, testKey(42), 42, "unreadable")
	for i := 0; i < 25; i++ {
		if _, err := c.Decrypt(context.Background(), hostile); err == nil {
			t.Fatal("Decrypt of an unknown version succeeded")
		}
	}
	if loads, _ := store.counts(); loads != 1 {
		t.Errorf("LoadUsableKeys called %d times across 25 unknown-version decrypts, want 1: reloads are rate-limited", loads)
	}
}

// TestGraceExpiryTakesEffectWithoutReload proves the Go-side deadline re-check
// exists: a key that was usable when the snapshot was taken stops decrypting
// the moment its window closes, with no reload and no restart.
func TestGraceExpiryTakesEffectWithoutReload(t *testing.T) {
	key1 := testKey(1)
	const window = 75 * time.Millisecond
	deadline := time.Now().Add(window)

	retired := retiredRecord(1, key1)
	retired.DecryptableUntil = &deadline

	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{{records: []fieldcrypto.KeyRecord{
		retired,
		activeRecord(2, testKey(2)),
	}}}}
	c := newStoreCipher(t, store)
	// A reload would re-supply the same (now expired) record anyway; pinning
	// the rate limit high makes "no reload happened" assertable.
	fieldcrypto.SetReloadTuningForTest(c, time.Hour, time.Hour)

	blob := sealUnder(t, key1, 1, "123-45-6789")
	if _, err := c.Decrypt(context.Background(), blob); err != nil {
		t.Fatalf("Decrypt inside the grace window: %v", err)
	}

	time.Sleep(window + 50*time.Millisecond)

	got, err := c.Decrypt(context.Background(), blob)
	if err == nil {
		t.Fatalf("Decrypt past the grace deadline = %q, want an error", got)
	}
	if got != "" {
		t.Errorf("Decrypt returned plaintext %q alongside its error", got)
	}
	if !strings.Contains(err.Error(), "grace deadline") {
		t.Errorf("error does not explain the expiry: %v", err)
	}
	if loads, _ := store.counts(); loads != 1 {
		t.Errorf("LoadUsableKeys called %d times, want 1: expiry must take effect without a reload", loads)
	}
}

// ---------------------------------------------------------------------------
// Rotation
// ---------------------------------------------------------------------------

// TestDecryptWithRotation covers every shape of the Rotation value the read
// path can produce.
func TestDecryptWithRotation(t *testing.T) {
	key1, key2, key3 := testKey(1), testKey(2), testKey(3)
	compromisedAt := time.Now().Add(-time.Hour)

	compromised := retiredRecord(2, key2)
	compromised.CompromisedAt = &compromisedAt

	newCipher := func(t *testing.T) (*fieldcrypto.Cipher, *fakeKeyStore) {
		t.Helper()
		store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{{records: []fieldcrypto.KeyRecord{
			retiredRecord(1, key1),
			compromised,
			activeRecord(3, key3),
		}}}}
		return newStoreCipher(t, store), store
	}

	t.Run("already current", func(t *testing.T) {
		c, _ := newCipher(t)
		const plaintext = "123-45-6789"
		got, rot, err := c.DecryptWithRotation(context.Background(), sealUnder(t, key3, 3, plaintext))
		if err != nil {
			t.Fatalf("DecryptWithRotation: %v", err)
		}
		if got != plaintext {
			t.Errorf("plaintext = %q, want %q", got, plaintext)
		}
		if !isZeroRotation(rot) {
			t.Errorf("Rotation = %+v, want the zero value for a current blob", rot)
		}
		if rot.Needed() {
			t.Error("Needed() reported true for a current blob")
		}
	})

	t.Run("retired standard-rotation key", func(t *testing.T) {
		c, _ := newCipher(t)
		const plaintext = "111-22-3333"
		got, rot, err := c.DecryptWithRotation(context.Background(), sealUnder(t, key1, 1, plaintext))
		if err != nil {
			t.Fatalf("DecryptWithRotation: %v", err)
		}
		if got != plaintext {
			t.Errorf("plaintext = %q, want %q", got, plaintext)
		}
		if !rot.Needed() {
			t.Fatal("Needed() reported false for a blob written under a retired key")
		}
		if rot.FromVersion != 1 || rot.ToVersion != 3 {
			t.Errorf("Rotation versions = %d→%d, want 1→3", rot.FromVersion, rot.ToVersion)
		}
		if rot.MustPersist {
			t.Error("MustPersist is true for a key that was never marked compromised")
		}
		if v := blobVersionOf(t, rot.Blob); v != 3 {
			t.Errorf("replacement blob carries version %d, want the active version 3", v)
		}
		replayed, err := openUnder(t, key3, rot.Blob)
		if err != nil {
			t.Fatalf("open the replacement blob under the active key: %v", err)
		}
		if replayed != plaintext {
			t.Errorf("replacement blob decrypts to %q, want %q", replayed, plaintext)
		}
	})

	t.Run("compromised source key", func(t *testing.T) {
		c, _ := newCipher(t)
		const plaintext = "444-55-6666"
		got, rot, err := c.DecryptWithRotation(context.Background(), sealUnder(t, key2, 2, plaintext))
		if err != nil {
			t.Fatalf("DecryptWithRotation: %v", err)
		}
		if got != plaintext {
			t.Errorf("plaintext = %q, want %q", got, plaintext)
		}
		if !rot.Needed() || rot.FromVersion != 2 || rot.ToVersion != 3 {
			t.Fatalf("Rotation = %+v, want a needed 2→3 rotation", rot)
		}
		if !rot.MustPersist {
			t.Error("MustPersist is false for a blob written under a compromised key; the caller cannot tell it must fail the read")
		}
	})

	t.Run("malformed blob", func(t *testing.T) {
		c, _ := newCipher(t)
		got, rot, err := c.DecryptWithRotation(context.Background(), make([]byte, minBlobSize))
		if err == nil {
			t.Fatalf("DecryptWithRotation of a version-0 blob = (%q, %+v), want an error", got, rot)
		}
		if got != "" || rot.Needed() {
			t.Errorf("failure yielded (%q, %+v), want (\"\", zero Rotation)", got, rot)
		}
	})
}

// TestStorelessCipherNeverRotates pins the contract api/service/mock_test.go
// and the round-trip tests depend on.
func TestStorelessCipherNeverRotates(t *testing.T) {
	key := testKey(1)
	c := newStaticCipher(t)
	ctx := context.Background()

	const plaintext = "123-45-6789"
	blob, err := c.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if got := blobVersionOf(t, blob); got != 1 {
		t.Errorf("store-less Cipher encrypted under version %d, want 1", got)
	}

	got, rot, err := c.DecryptWithRotation(ctx, blob)
	if err != nil {
		t.Fatalf("DecryptWithRotation: %v", err)
	}
	if got != plaintext {
		t.Errorf("plaintext = %q, want %q", got, plaintext)
	}
	if !isZeroRotation(rot) {
		t.Errorf("Rotation = %+v, want the zero value", rot)
	}

	if err := c.Reload(ctx); err != nil {
		t.Errorf("Reload on a store-less Cipher = %v, want nil (nothing to reload)", err)
	}

	if _, err := c.Decrypt(ctx, sealUnder(t, key, 2, plaintext)); err == nil {
		t.Error("store-less Cipher decrypted a version-2 blob; it pins version 1")
	}

	if _, err := fieldcrypto.NewFromKey(make([]byte, 31)); err == nil {
		t.Error("NewFromKey accepted a 31-byte key")
	}
	if _, err := fieldcrypto.NewFromKey(make([]byte, 32)); err == nil {
		t.Error("NewFromKey accepted an all-zero key, which is corrupt material rather than a key")
	}
}

// ---------------------------------------------------------------------------
// Staleness
// ---------------------------------------------------------------------------

// TestEncryptRefreshesStaleKeySet proves the TTL bound on the encrypt path:
// without it a replica keeps writing fresh ciphertext under a key a peer
// retired, which is exactly what a compromised-key rotation must stop.
func TestEncryptRefreshesStaleKeySet(t *testing.T) {
	key1, key2 := testKey(1), testKey(2)
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
		{records: []fieldcrypto.KeyRecord{activeRecord(1, key1)}},
		{records: []fieldcrypto.KeyRecord{retiredRecord(1, key1), activeRecord(2, key2)}},
	}}
	c := newStoreCipher(t, store)
	fieldcrypto.SetReloadTuningForTest(c, 0, 0)

	blob, err := c.Encrypt(context.Background(), "123-45-6789")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if got := blobVersionOf(t, blob); got != 2 {
		t.Errorf("Encrypt used version %d, want 2: a stale snapshot must be refreshed before the active key is selected", got)
	}
	if loads, _ := store.counts(); loads != 2 {
		t.Errorf("LoadUsableKeys called %d times, want 2 (construction + the TTL refresh)", loads)
	}
}

// TestEncryptSurvivesReloadFailure proves the deliberate asymmetry on the
// encrypt path: a briefly unreachable key table logs loudly and writes go on
// under the snapshot already held, rather than every write failing.
func TestEncryptSurvivesReloadFailure(t *testing.T) {
	logs := captureDefaultLogger(t)

	key1 := testKey(1)
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
		{records: []fieldcrypto.KeyRecord{activeRecord(1, key1)}},
		{err: errTestStoreUnreachable},
	}}
	c := newStoreCipher(t, store)
	fieldcrypto.SetReloadTuningForTest(c, 0, 0)

	const plaintext = "123-45-6789"
	blob, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt after a failed refresh = %v, want success under the existing snapshot", err)
	}
	if got := blobVersionOf(t, blob); got != 1 {
		t.Errorf("Encrypt used version %d, want the held version 1", got)
	}
	if got, err := openUnder(t, key1, blob); err != nil || got != plaintext {
		t.Errorf("blob does not round-trip under the held key: (%q, %v)", got, err)
	}
	if loads, _ := store.counts(); loads != 2 {
		t.Errorf("LoadUsableKeys called %d times, want 2 (construction + the failed refresh)", loads)
	}
	if out := logs.String(); !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "key-set refresh failed") {
		t.Errorf("a failed encrypt-path refresh must be observable at error level; captured log was:\n%s", out)
	}
}

// TestReloadConvergesImmediately covers the exported Reload the admin rotation
// handler calls post-commit: it must take effect regardless of TTL or rate
// limit, both of which are pinned high here.
func TestReloadConvergesImmediately(t *testing.T) {
	key1, key2 := testKey(1), testKey(2)
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
		{records: []fieldcrypto.KeyRecord{activeRecord(1, key1)}},
		{records: []fieldcrypto.KeyRecord{retiredRecord(1, key1), activeRecord(2, key2)}},
	}}
	c := newStoreCipher(t, store)
	fieldcrypto.SetReloadTuningForTest(c, time.Hour, time.Hour)

	before, err := c.Encrypt(context.Background(), "x")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if got := blobVersionOf(t, before); got != 1 {
		t.Fatalf("pre-Reload Encrypt used version %d, want 1", got)
	}

	if err := c.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	after, err := c.Encrypt(context.Background(), "x")
	if err != nil {
		t.Fatalf("Encrypt after Reload: %v", err)
	}
	if got := blobVersionOf(t, after); got != 2 {
		t.Errorf("post-Reload Encrypt used version %d, want 2", got)
	}
}

// TestReloadFailureKeepsPreviousKeySet proves a failed reload never leaves the
// Cipher with a degraded or empty key set.
func TestReloadFailureKeepsPreviousKeySet(t *testing.T) {
	key1 := testKey(1)
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
		{records: []fieldcrypto.KeyRecord{activeRecord(1, key1)}},
		{err: errTestStoreUnreachable},
	}}
	c := newStoreCipher(t, store)
	fieldcrypto.SetReloadTuningForTest(c, time.Hour, 0)

	if err := c.Reload(context.Background()); err == nil {
		t.Fatal("Reload against an unreachable store returned nil")
	}
	got, err := c.Decrypt(context.Background(), sealUnder(t, key1, 1, "123-45-6789"))
	if err != nil {
		t.Fatalf("Decrypt after a failed Reload: %v", err)
	}
	if got != "123-45-6789" {
		t.Errorf("Decrypt = %q after a failed Reload", got)
	}
}

// TestConcurrentAcrossReload is the -race exercise. It drives Encrypt and
// Decrypt from many goroutines while reloads swap the key set underneath them,
// which is the hazard the atomic snapshot swap exists to close. Every round
// trip must succeed regardless of which snapshot it lands on, and blobs
// written under the pre-rotation key must stay readable throughout.
func TestConcurrentAcrossReload(t *testing.T) {
	key1, key2 := testKey(1), testKey(2)
	store := &fakeKeyStore{t: t, loadSeq: []fakeLoadResult{
		{records: []fieldcrypto.KeyRecord{activeRecord(1, key1)}},
		{records: []fieldcrypto.KeyRecord{retiredRecord(1, key1), activeRecord(2, key2)}},
	}}
	c := newStoreCipher(t, store)
	// Long TTL and no rate limit: reloads happen because this test asks for
	// them, not because a timer fired.
	fieldcrypto.SetReloadTuningForTest(c, time.Hour, 0)

	const (
		workers   = 24
		reloaders = 4
		rounds    = 40
	)
	const plaintext = "987-65-4321"
	legacy := sealUnder(t, key1, 1, plaintext)

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(workers + reloaders)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				blob, err := c.Encrypt(ctx, plaintext)
				if err != nil {
					t.Errorf("Encrypt: %v", err)
					return
				}
				got, rot, err := c.DecryptWithRotation(ctx, blob)
				if err != nil {
					t.Errorf("DecryptWithRotation of a just-written blob: %v", err)
					return
				}
				if got != plaintext {
					t.Errorf("DecryptWithRotation = %q, want %q", got, plaintext)
					return
				}
				if rot.Needed() && rot.ToVersion == rot.FromVersion {
					t.Errorf("rotation reported for an unchanged version: %+v", rot)
					return
				}
				if got, err := c.Decrypt(ctx, legacy); err != nil || got != plaintext {
					t.Errorf("Decrypt of a pre-rotation blob = (%q, %v); retired keys must stay readable across a reload", got, err)
					return
				}
			}
		}()
	}
	for i := 0; i < reloaders; i++ {
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				if err := c.Reload(ctx); err != nil {
					t.Errorf("Reload: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
