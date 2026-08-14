package fieldcrypto

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

// NewFromKey builds a store-less Cipher from a raw 32-byte key, pinned to
// version staticKeyVersion. It never reloads and holds exactly that one key,
// so DecryptWithRotation always reports the zero Rotation: the only version it
// can decrypt is also the only version it encrypts under. Blobs it produces
// are wire-compatible with the multi-key format, which is what lets tests and
// in-memory callers work without a key store.
//
// Unlike the store-backed constructor, this one does not take ownership of
// key: the caller's slice is neither retained nor zeroed.
func NewFromKey(key []byte) (*Cipher, error) {
	set, err := buildKeySet([]KeyRecord{{Version: staticKeyVersion, KeyBytes: key}}, time.Now())
	if err != nil {
		return nil, err
	}
	c := &Cipher{keySetTTL: defaultKeySetTTL, minReloadInterval: defaultMinReloadInterval}
	c.set.Store(set)
	return c, nil
}

// NewFromEnvOrGenerate builds the process's Cipher from the key store.
//
// It loads every usable key. On an entirely empty key table it bootstraps
// version 1 — from CORE_FIELD_KEY_HEX when that is set, otherwise from 32
// freshly generated random bytes — through the store's guarded insert. An
// insert that affects no row means another caller established the key material
// first, in which case this one re-loads and adopts the winner rather than
// ever generating a second key.
//
// CORE_FIELD_KEY_HEX is a first-boot seed only. When the table already holds
// keys, bytes matching the active key's proceed silently and bytes that differ
// fail construction loudly, naming the rotation endpoint: an operator who
// edits the variable expecting to rotate must find out that they have not.
func NewFromEnvOrGenerate(ctx context.Context, store KeyStore) (*Cipher, error) {
	if store == nil {
		return nil, errors.New("fieldcrypto: NewFromEnvOrGenerate needs a non-nil KeyStore")
	}

	envKey, err := envSeedKey()
	if err != nil {
		return nil, err
	}
	defer zeroBytes(envKey)

	// Taken before the load so the snapshot is never treated as fresher than
	// the data it could have seen.
	startedAt := time.Now()
	records, err := store.LoadUsableKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("fieldcrypto: load usable keys: %w", err)
	}

	if len(records) == 0 {
		seed := envKey
		if seed == nil {
			generated, gerr := generateKey()
			if gerr != nil {
				return nil, gerr
			}
			// Deferred to this function's return, which is after the AEADs
			// have been built from it.
			defer zeroBytes(generated)
			seed = generated
		}
		// Re-captured here because the adopted records come from bootstrap's
		// own InsertInitialKey (or a second LoadUsableKeys on a lost race),
		// which started after the startedAt taken above — so the snapshot
		// must not be treated as fresher than what it could have seen.
		startedAt = time.Now()
		if records, err = bootstrap(ctx, store, seed); err != nil {
			return nil, err
		}
	}
	defer zeroKeyMaterial(records)

	set, err := buildKeySet(records, startedAt)
	if err != nil {
		return nil, err
	}
	// Ordered after buildKeySet so that "no active key" and "two active keys"
	// are reported as themselves rather than as an env-var mismatch.
	if err := checkEnvSeed(envKey, records); err != nil {
		return nil, err
	}

	c := &Cipher{store: store, keySetTTL: defaultKeySetTTL, minReloadInterval: defaultMinReloadInterval}
	c.set.Store(set)
	c.lastLoadAttempt.Store(startedAt.UnixNano())
	return c, nil
}

// bootstrap establishes the first key on an empty table and returns the key
// set to adopt.
//
// The store's insert is guarded twice — it inserts only when the table is
// still empty, and does nothing on a unique conflict — so no rows back means
// another caller established the key material, whether it committed before our
// load or concurrently with our insert. Both cases resolve the same way: load
// again and adopt what is there. A re-load that still finds no usable key is a
// hard error, never a second attempt to generate.
func bootstrap(ctx context.Context, store KeyStore, seed []byte) ([]KeyRecord, error) {
	rec, err := store.InsertInitialKey(ctx, seed)
	switch {
	case err == nil:
		return []KeyRecord{rec}, nil

	case errors.Is(err, pgx.ErrNoRows):
		records, rerr := store.LoadUsableKeys(ctx)
		if rerr != nil {
			return nil, fmt.Errorf("fieldcrypto: re-load usable keys after losing the bootstrap race: %w", rerr)
		}
		if len(records) == 0 {
			return nil, errors.New("fieldcrypto: the initial-key insert affected no row and the key table still offers no usable key; it holds rows that are all retired or expired, which must be repaired rather than papered over with a new key")
		}
		return records, nil

	default:
		return nil, fmt.Errorf("fieldcrypto: persist initial key: %w", err)
	}
}

// generateKey returns 32 cryptographically random bytes.
func generateKey() ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("fieldcrypto: generate key: %w", err)
	}
	return key, nil
}

// envSeedKey decodes CORE_FIELD_KEY_HEX, returning (nil, nil) when the
// variable is absent. A variable that is set but unusable is an error: an
// operator who supplied a key must never have it silently ignored.
func envSeedKey() ([]byte, error) {
	hexKey, ok := os.LookupEnv(envKeyName)
	if !ok {
		return nil, nil
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		// A failed decode still returns the prefix it managed to decode.
		zeroBytes(key)
		return nil, fmt.Errorf("fieldcrypto: %s is not valid hex: %w", envKeyName, err)
	}
	if len(key) != keySize {
		zeroBytes(key)
		return nil, fmt.Errorf("fieldcrypto: %s must decode to %d bytes, got %d", envKeyName, keySize, len(key))
	}
	// Rejected here, before the value can reach the bootstrap insert: an
	// all-zero key would otherwise be persisted as version 1 and only then be
	// refused by newKeyEntry, leaving a permanently unusable row behind.
	if subtle.ConstantTimeCompare(key, zeroKey[:]) == 1 {
		zeroBytes(key)
		return nil, fmt.Errorf("fieldcrypto: %s is all zero bytes, which is not usable key material", envKeyName)
	}
	return key, nil
}

// checkEnvSeed enforces the first-boot-only contract: once the table holds
// keys, CORE_FIELD_KEY_HEX must either match the active key exactly or fail
// construction. Silently ignoring it would leave an operator believing they
// had rotated the active key by editing an environment variable.
func checkEnvSeed(envKey []byte, records []KeyRecord) error {
	if envKey == nil {
		return nil
	}
	for _, rec := range records {
		if rec.RetiredAt != nil {
			continue
		}
		if subtle.ConstantTimeCompare(envKey, rec.KeyBytes) == 1 {
			return nil
		}
		return fmt.Errorf(
			"fieldcrypto: %s does not match active key version %d; it is a first-boot bootstrap seed only, so change the active key with POST /v1/field-crypto-keys/rotations rather than by editing the environment",
			envKeyName, rec.Version)
	}
	// Unreachable: buildKeySet runs first and rejects a set with no active key.
	return nil
}
