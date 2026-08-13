// Package fieldcrypto provides versioned, multi-key AES-256-GCM encryption
// and decryption for small database field values such as SSN and EIN.
//
// # Blob layout
//
// Every non-empty blob this package produces is
//
//	version(4) || nonce(12) || ciphertext || tag(16)
//
// where version is the big-endian uint32 key version that produced the blob.
// Those exact four prefix bytes are passed verbatim to the AEAD as additional
// authenticated data; they are not part of the ciphertext handed to
// AEAD.Open. The minimum valid blob is therefore 32 bytes. Version 0 is never
// issued — the field_crypto_keys identity sequence starts at 1 — so a zeroed
// or truncated prefix is a malformed blob rather than a plausible lookup miss.
// Encrypt of the empty string returns a zero-length blob carrying no version
// prefix, which the DB layer stores as NULL.
//
// # Key model
//
// The field_crypto_keys table is the single source of truth for every key
// version. Exactly one row is active (retired_at IS NULL) and it is the only
// key this package ever encrypts under. A rotation retires the incumbent and
// installs a replacement; retired keys stay loaded for decryption so blobs
// written under them remain readable until re-encrypt-on-read has moved them
// onto the active key.
//
// Two per-key lifecycle attributes shape decrypt behavior:
//
//   - decryptable_until bounds a retired key's decrypt grace window. It is
//     enforced here, at decrypt time, as well as by the SQL load filter, so a
//     process that loaded a key hours ago stops honoring it the moment the
//     deadline passes rather than at its next restart. An expired key behaves
//     exactly like an unknown version.
//   - compromised_at marks a retired key as known-leaked. Decrypting a blob
//     written under such a key yields a Rotation whose MustPersist is true,
//     meaning a caller that cannot durably store the replacement blob must
//     fail the read rather than return the plaintext. That policy is derived
//     once, here; no caller reads compromised_at or re-derives it.
//
// # Staleness across processes
//
// A Cipher holds an immutable key-set snapshot that a reload swaps wholesale,
// so a replica converges on a peer's rotation without a restart:
//
//   - on meeting a version it does not hold, the decrypt path reloads once
//     and retries before failing loudly. Reloads are rate-limited so a stream
//     of corrupt or hostile blobs cannot amplify into a query storm.
//   - the encrypt path reloads when its snapshot is older than the key-set
//     TTL, so a replica cannot keep encrypting under a key that was retired
//     underneath it. A reload failure there is logged at error level and the
//     existing snapshot is used: failing every write because the key table is
//     briefly unreachable is worse than a bounded window of stale encryption.
//   - Reload is exported so the process that serves an admin rotation can
//     converge immediately instead of waiting out the TTL.
//
// # CORE_FIELD_KEY_HEX
//
// The environment variable is a first-boot-only bootstrap seed. On an empty
// table its 32 decoded bytes are what NewFromEnvOrGenerate persists as version
// 1. On every later boot, bytes equal to the active key's proceed silently and
// bytes that differ fail construction loudly: the active key is changed
// through POST /v1/field-crypto-keys/rotations, never by editing an
// environment variable.
//
// # Authenticated additional data
//
// The key version is bound as AAD, so a blob cannot be replayed under a
// different key version without failing authentication. No row identity is
// bound: a blob could still theoretically be moved from one row to another
// under the same key version without detection. Binding ciphertext to a
// specific row id remains a planned hardening step.
package fieldcrypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// envKeyName is the first-boot bootstrap seed variable.
	envKeyName = "CORE_FIELD_KEY_HEX"

	// keySize is the AES-256 key length.
	keySize = 32

	// Wire-format geometry. nonceSize and tagSize restate GCM's defaults so
	// the layout is readable at a glance; newKeyEntry verifies the built AEAD
	// still agrees with them.
	versionSize = 4
	nonceSize   = 12
	tagSize     = 16

	// minBlobSize is the shortest well-formed blob: a version prefix, a
	// nonce, and a tag over zero plaintext bytes.
	minBlobSize = versionSize + nonceSize + tagSize

	// defaultKeySetTTL bounds how long Encrypt will keep using a snapshot
	// before refreshing it, and so bounds how long a replica can keep
	// encrypting under a key a peer has retired.
	defaultKeySetTTL = 60 * time.Second

	// defaultMinReloadInterval is the shortest gap between two load attempts.
	// It is what stops corrupt or hostile blobs from turning every read into
	// a database query, and stops an unreachable key table from turning every
	// write into one.
	defaultMinReloadInterval = 5 * time.Second

	// staticKeyVersion is the version a store-less Cipher (NewFromKey) pins.
	staticKeyVersion = 1
)

// KeyRecord is fieldcrypto's own view of one field_crypto_keys row. It
// deliberately names no generated sqlc row type: keeping this package
// model-free is what lets the api/fieldcrypto façade absorb that dependency
// and keeps the module manifest's cipher service block unchanged.
//
// RetiredAt == nil identifies the single active key. DecryptableUntil bounds a
// retired key's decrypt grace window (nil means no expiry) and CompromisedAt
// records that a key is known-leaked (nil means it is not).
type KeyRecord struct {
	Version          uint32
	KeyBytes         []byte
	RetiredAt        *time.Time
	DecryptableUntil *time.Time
	CompromisedAt    *time.Time
}

// KeyStore is the persistence contract the multi-key cipher needs.
//
// LoadUsableKeys returns the active key plus every retired key still inside
// its grace window. InsertInitialKey performs the guarded first-boot insert;
// it must report pgx.ErrNoRows when the insert affected no row, which is how
// a caller that lost the bootstrap race learns to adopt the winner's key.
//
// Both methods must return freshly allocated KeyBytes on every call. A Cipher
// zeroes the key material it is handed once it has built the AEADs from it, so
// a store that returns a slice it also retains would hand back zeros next time.
type KeyStore interface {
	LoadUsableKeys(ctx context.Context) ([]KeyRecord, error)
	InsertInitialKey(ctx context.Context, keyBytes []byte) (KeyRecord, error)
}

// Rotation describes what a read owes the store about the blob it just read.
// The zero value means "nothing to do", which is what an empty blob and an
// already-current blob both produce.
type Rotation struct {
	// FromVersion is the key version that produced the blob; 0 for an empty
	// blob.
	FromVersion uint32
	// ToVersion is the active key version at decrypt time.
	ToVersion uint32
	// Blob is the replacement, non-empty iff FromVersion is neither 0 nor
	// ToVersion.
	Blob []byte
	// MustPersist mirrors compromised_at on FromVersion's key row. When true,
	// a caller that cannot durably persist Blob MUST fail the read rather than
	// return the plaintext.
	MustPersist bool
}

// Needed reports whether Blob must be written back.
func (r Rotation) Needed() bool { return len(r.Blob) > 0 }

// keyEntry is one loaded key: the AEAD built from its material plus the two
// lifecycle facts the decrypt path needs. The key bytes themselves are not
// retained — aes.NewCipher copies them into its key schedule, so nothing after
// construction needs them and holding them would only widen what a heap dump
// exposes.
type keyEntry struct {
	aead cipher.AEAD
	// decryptableUntil is the grace deadline for a retired key; nil means the
	// key never stops decrypting.
	decryptableUntil *time.Time
	// compromisedAt is non-nil when the key is known-leaked, which is the sole
	// input to Rotation.MustPersist.
	compromisedAt *time.Time
}

// keySet is an immutable snapshot of the usable key material. A reload swaps a
// whole new one in, so readers never observe a partially-updated set. Nothing
// mutates a keySet after buildKeySet returns it.
type keySet struct {
	// active is the version all new encryption uses.
	active uint32
	// byVersion maps every usable version to its built AEAD.
	byVersion map[uint32]*keyEntry
	// loadedAt is when the load that produced this snapshot began — not when
	// it finished — so both the TTL check and the concurrent-reload collapse
	// treat the snapshot as no fresher than the data it could have seen.
	loadedAt time.Time
}

// usable returns the entry for version, or nil when the version is unknown or
// its grace window has closed. The SQL load filter already drops expired keys,
// but a snapshot taken before the deadline must stop honoring the key without
// waiting for a reload — so expiry is re-checked here on every decrypt.
func (s *keySet) usable(version uint32, now time.Time) *keyEntry {
	entry, ok := s.byVersion[version]
	if !ok {
		return nil
	}
	if entry.decryptableUntil != nil && !now.Before(*entry.decryptableUntil) {
		return nil
	}
	return entry
}

// Cipher encrypts and decrypts small field values with AES-256-GCM under a
// reloadable set of versioned keys. Cipher values are safe for concurrent use.
type Cipher struct {
	// store is nil for the store-less Cipher NewFromKey builds; such a Cipher
	// pins one key at staticKeyVersion and never reloads.
	store KeyStore

	// set holds the current snapshot. Never nil once a constructor returns.
	set atomic.Pointer[keySet]

	// reloadMu collapses concurrent reloads into a single store query.
	reloadMu sync.Mutex

	// lastLoadAttempt is the unix-nano time of the most recent load attempt,
	// successful or not. It rate-limits the unknown-version decrypt reload and
	// the TTL-driven encrypt reload alike; counting failed attempts is what
	// keeps an unreachable key table from being retried on every call.
	lastLoadAttempt atomic.Int64

	// keySetTTL and minReloadInterval are fixed by the constructor. They are
	// plain fields, read-only once the Cipher is shared across goroutines.
	keySetTTL         time.Duration
	minReloadInterval time.Duration
}

// BlobVersion decodes the key version from a stored blob's 4-byte prefix. It
// rejects a blob shorter than the 32-byte minimum and a decoded version of 0,
// which is never issued. It reads only the prefix: no key material is needed
// and no authentication is performed.
func BlobVersion(blob []byte) (uint32, error) {
	if len(blob) < minBlobSize {
		return 0, fmt.Errorf("fieldcrypto: blob is %d bytes, shorter than the %d-byte minimum", len(blob), minBlobSize)
	}
	version := binary.BigEndian.Uint32(blob[:versionSize])
	if version == 0 {
		return 0, errors.New("fieldcrypto: blob carries key version 0, which is never issued")
	}
	return version, nil
}

// Encrypt returns version(4) || nonce(12) || ciphertext || tag(16) under the
// active key, with the four version bytes bound as the AEAD's additional
// authenticated data. It returns ([]byte{}, nil) for the empty string so
// callers need not special-case empty input; the DB layer stores a zero-length
// blob as NULL.
//
// When the held key set is older than the key-set TTL, Encrypt refreshes it
// first so a replica cannot go on encrypting under a key a peer has retired.
func (c *Cipher) Encrypt(ctx context.Context, plaintext string) ([]byte, error) {
	if plaintext == "" {
		return []byte{}, nil
	}
	return sealBlob(c.encryptSet(ctx), plaintext)
}

// Decrypt returns the plaintext of blob, for callers that cannot rotate. It
// returns ("", nil) when blob is empty or nil, and an error — never an empty
// plaintext — when the blob is malformed, was written under a version this
// process cannot use, or fails authentication.
func (c *Cipher) Decrypt(ctx context.Context, blob []byte) (string, error) {
	plaintext, _, _, _, err := c.openBlob(ctx, blob)
	if err != nil {
		return "", err
	}
	return plaintext, nil
}

// DecryptWithRotation decrypts blob and, when it was written under a
// non-active key version, also returns the replacement blob encrypted under
// the active key. Persisting the replacement is the caller's responsibility;
// Rotation.MustPersist states whether failing to do so must fail the read.
//
// An empty blob and an already-current blob both yield the zero Rotation.
func (c *Cipher) DecryptWithRotation(ctx context.Context, blob []byte) (string, Rotation, error) {
	plaintext, version, entry, set, err := c.openBlob(ctx, blob)
	if err != nil {
		return "", Rotation{}, err
	}
	if version == 0 || version == set.active {
		return plaintext, Rotation{}, nil
	}

	replacement, err := sealBlob(set, plaintext)
	if err != nil {
		// The plaintext is valid, but a caller handed a rotation it cannot act
		// on is worse than a failed read: it would either drop the rotation
		// silently or, under a compromised key, be unable to tell that it must
		// fail. Surface the failure instead.
		return "", Rotation{}, fmt.Errorf("fieldcrypto: re-encrypt version %d blob under active version %d: %w", version, set.active, err)
	}
	return plaintext, Rotation{
		FromVersion: version,
		ToVersion:   set.active,
		Blob:        replacement,
		MustPersist: entry.compromisedAt != nil,
	}, nil
}

// Reload replaces the held key set with a freshly loaded one, unconditionally
// — no TTL and no rate limit — so the process that just served an admin
// rotation converges immediately. Concurrent reloads collapse into one query.
// It is a no-op on a store-less Cipher, which has nothing to reload.
func (c *Cipher) Reload(ctx context.Context) error {
	if c.store == nil {
		return nil
	}
	return c.reload(ctx)
}

// encryptSet returns the snapshot Encrypt must use, refreshing it first when
// it is older than the TTL. A refresh failure is logged and the existing
// snapshot is used: failing every write because the key table is briefly
// unreachable is worse than a bounded window of stale encryption.
func (c *Cipher) encryptSet(ctx context.Context) *keySet {
	set := c.set.Load()
	if c.store == nil || time.Since(set.loadedAt) < c.keySetTTL || !c.reloadAllowed() {
		return set
	}
	if err := c.reload(ctx); err != nil {
		slog.ErrorContext(ctx, "fieldcrypto: key-set refresh failed; encrypting under the existing snapshot",
			"error", err,
			"snapshot_age_seconds", time.Since(set.loadedAt).Seconds(),
			"active_version", set.active)
	}
	return c.set.Load()
}

// openBlob decrypts blob under the key version its prefix names, reloading the
// key set at most once when that version is not usable. It returns the
// snapshot that decrypted the blob and the entry that did so, so
// DecryptWithRotation can derive the rotation from the same consistent view.
// The returned version is 0 exactly when blob is empty.
func (c *Cipher) openBlob(ctx context.Context, blob []byte) (string, uint32, *keyEntry, *keySet, error) {
	if len(blob) == 0 {
		return "", 0, nil, c.set.Load(), nil
	}

	version, err := BlobVersion(blob)
	if err != nil {
		// A malformed blob is never a stale-key-set problem, so it must not
		// cost a query: fail immediately with no reload attempt.
		return "", 0, nil, nil, err
	}

	set := c.set.Load()
	entry := set.usable(version, time.Now())
	if entry == nil && c.store != nil && c.reloadAllowed() {
		if rerr := c.reload(ctx); rerr != nil {
			return "", 0, nil, nil, fmt.Errorf("fieldcrypto: key version %d is not usable and the key-set reload failed: %w", version, rerr)
		}
		set = c.set.Load()
		entry = set.usable(version, time.Now())
	}
	if entry == nil {
		return "", 0, nil, nil, unusableVersionError(set, version)
	}

	nonce := blob[versionSize : versionSize+nonceSize]
	ciphertext := blob[versionSize+nonceSize:]
	plaintext, err := entry.aead.Open(nil, nonce, ciphertext, blob[:versionSize])
	if err != nil {
		return "", 0, nil, nil, fmt.Errorf("fieldcrypto: authenticate blob under key version %d: %w", version, err)
	}
	return string(plaintext), version, entry, set, nil
}

// unusableVersionError reports why version cannot be used. An expired key and
// an unknown one behave identically — one rate-limited reload, then a loud
// failure — but they call for different operator action, so they say so.
func unusableVersionError(set *keySet, version uint32) error {
	if entry, held := set.byVersion[version]; held && entry.decryptableUntil != nil {
		return fmt.Errorf(
			"fieldcrypto: key version %d passed its decrypt grace deadline at %s; extend it via PUT /v1/field-crypto-keys/%d/grace to read blobs still written under it",
			version, entry.decryptableUntil.UTC().Format(time.RFC3339), version)
	}
	return fmt.Errorf(
		"fieldcrypto: unknown key version %d (active version is %d); this database offers no usable key for that blob",
		version, set.active)
}

// reloadAllowed reports whether enough time has passed since the last load
// attempt — successful or not — to make another one.
func (c *Cipher) reloadAllowed() bool {
	return time.Since(time.Unix(0, c.lastLoadAttempt.Load())) >= c.minReloadInterval
}

// reload loads a fresh key set and swaps it in. Callers hold no lock; the
// reload mutex serializes loads, and a caller that finds a newer snapshot
// already installed when its turn comes returns without issuing a query.
func (c *Cipher) reload(ctx context.Context) error {
	entered := time.Now()

	c.reloadMu.Lock()
	defer c.reloadMu.Unlock()

	if cur := c.set.Load(); cur != nil && cur.loadedAt.After(entered) {
		// Another goroutine's load began after we started waiting, so its
		// snapshot already reflects everything ours would have seen.
		return nil
	}

	startedAt := time.Now()
	c.lastLoadAttempt.Store(startedAt.UnixNano())

	records, err := c.store.LoadUsableKeys(ctx)
	if err != nil {
		return fmt.Errorf("fieldcrypto: load usable keys: %w", err)
	}
	defer zeroKeyMaterial(records)

	set, err := buildKeySet(records, startedAt)
	if err != nil {
		return err
	}
	c.set.Store(set)
	return nil
}

// sealBlob encrypts plaintext under set's active key.
func sealBlob(set *keySet, plaintext string) ([]byte, error) {
	entry, ok := set.byVersion[set.active]
	if !ok {
		// buildKeySet guarantees the active version is present, so reaching
		// this is a violated invariant rather than an operational failure.
		return nil, fmt.Errorf("fieldcrypto: key set holds no entry for active version %d", set.active)
	}

	out := make([]byte, versionSize+nonceSize, minBlobSize+len(plaintext))
	binary.BigEndian.PutUint32(out[:versionSize], set.active)
	nonce := out[versionSize : versionSize+nonceSize]
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("fieldcrypto: generate nonce: %w", err)
	}
	// Seal appends ciphertext+tag after the prefix and nonce already in out;
	// the four prefix bytes go in verbatim as the AAD and stay outside the
	// ciphertext.
	return entry.aead.Seal(out, nonce, []byte(plaintext), out[:versionSize]), nil
}

// buildKeySet validates records and builds the immutable snapshot. Everything
// here is boundary validation of data arriving from the key store: a set with
// no active key, two active keys, a duplicate or zero version, or key material
// of the wrong length is rejected loudly rather than half-adopted.
func buildKeySet(records []KeyRecord, loadedAt time.Time) (*keySet, error) {
	set := &keySet{byVersion: make(map[uint32]*keyEntry, len(records)), loadedAt: loadedAt}
	for _, rec := range records {
		if rec.Version == 0 {
			return nil, errors.New("fieldcrypto: key store returned version 0, which is never issued")
		}
		if _, dup := set.byVersion[rec.Version]; dup {
			return nil, fmt.Errorf("fieldcrypto: key store returned key version %d twice", rec.Version)
		}
		entry, err := newKeyEntry(rec)
		if err != nil {
			return nil, err
		}
		set.byVersion[rec.Version] = entry

		if rec.RetiredAt == nil {
			if set.active != 0 {
				return nil, fmt.Errorf("fieldcrypto: key store returned two active keys (versions %d and %d)", set.active, rec.Version)
			}
			set.active = rec.Version
		}
	}
	if set.active == 0 {
		return nil, errors.New("fieldcrypto: key store returned no active key; every usable key is retired, so nothing can be encrypted")
	}
	return set, nil
}

// newKeyEntry builds the AEAD for one key record. It does not retain
// rec.KeyBytes: aes.NewCipher copies the material into its key schedule, which
// is what makes zeroing the record afterwards safe.
func newKeyEntry(rec KeyRecord) (*keyEntry, error) {
	if len(rec.KeyBytes) != keySize {
		return nil, fmt.Errorf("fieldcrypto: key version %d must be %d bytes, got %d", rec.Version, keySize, len(rec.KeyBytes))
	}
	block, err := aes.NewCipher(rec.KeyBytes)
	if err != nil {
		return nil, fmt.Errorf("fieldcrypto: build AES cipher for key version %d: %w", rec.Version, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("fieldcrypto: build GCM for key version %d: %w", rec.Version, err)
	}
	if aead.NonceSize() != nonceSize || aead.Overhead() != tagSize {
		return nil, fmt.Errorf(
			"fieldcrypto: GCM geometry is nonce %d / overhead %d, but the wire format assumes %d / %d",
			aead.NonceSize(), aead.Overhead(), nonceSize, tagSize)
	}
	return &keyEntry{
		aead: aead,
		// Copy the timestamps rather than aliasing the store's pointers: a
		// snapshot other goroutines read concurrently must not be mutable by
		// whoever handed us the record.
		decryptableUntil: copyTime(rec.DecryptableUntil),
		compromisedAt:    copyTime(rec.CompromisedAt),
	}, nil
}

// copyTime returns a private copy of t, or nil.
func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

// zeroBytes overwrites b. Best-effort key hygiene: it shortens how long key
// material sits in a heap the process may later dump or swap, and nothing more
// — Go makes no guarantee that no copy was made along the way.
func zeroBytes(b []byte) { clear(b) }

// zeroKeyMaterial overwrites the key bytes of every record. Called once the
// AEADs have been built, at which point the material is no longer needed. See
// KeyStore for the ownership rule this relies on.
func zeroKeyMaterial(records []KeyRecord) {
	for _, rec := range records {
		zeroBytes(rec.KeyBytes)
	}
}
