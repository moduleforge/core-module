package fieldcrypto_test

import (
	"bytes"
	"os"
	"sync"
	"testing"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
)

// newTestCipher creates a Cipher with a fixed, deterministic 32-byte key.
func newTestCipher(t *testing.T) *fieldcrypto.Cipher {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1) // 0x01..0x20
	}
	c, err := fieldcrypto.NewFromKey(key)
	if err != nil {
		t.Fatalf("NewFromKey: %v", err)
	}
	return c
}

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

// TestRoundTrip verifies that Encrypt then Decrypt returns the original value.
func TestRoundTrip(t *testing.T) {
	c := newTestCipher(t)
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
		t.Errorf("Decrypt = %q, want %q", got, plaintext)
	}
}

// TestNonceUniqueness confirms that two encryptions of the same plaintext
// produce distinct blobs (probabilistic nonce uniqueness).
func TestNonceUniqueness(t *testing.T) {
	c := newTestCipher(t)
	const plaintext = "123-45-6789"

	blob1, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	blob2, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}
	if bytes.Equal(blob1, blob2) {
		t.Error("two encryptions of the same plaintext produced identical blobs (nonce reuse)")
	}
}

// TestEmptyInput verifies the empty-string contract on both sides.
func TestEmptyInput(t *testing.T) {
	c := newTestCipher(t)

	blob, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	if len(blob) != 0 {
		t.Errorf("Encrypt(\"\") returned blob of length %d, want 0", len(blob))
	}

	got, err := c.Decrypt(nil)
	if err != nil {
		t.Fatalf("Decrypt nil: %v", err)
	}
	if got != "" {
		t.Errorf("Decrypt(nil) = %q, want empty string", got)
	}

	got, err = c.Decrypt([]byte{})
	if err != nil {
		t.Fatalf("Decrypt empty slice: %v", err)
	}
	if got != "" {
		t.Errorf("Decrypt([]byte{}) = %q, want empty string", got)
	}
}

// TestTampering verifies that flipping a byte in a valid blob causes
// decryption to return a non-nil error.
func TestTampering(t *testing.T) {
	c := newTestCipher(t)

	blob, err := c.Encrypt("123-45-6789")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Flip the last byte (in the GCM authentication tag region).
	tampered := make([]byte, len(blob))
	copy(tampered, blob)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = c.Decrypt(tampered)
	if err == nil {
		t.Error("Decrypt accepted tampered blob; expected an error")
	}
}

// TestNewFromEnv_Errors exercises the three failure cases for NewFromEnv.
func TestNewFromEnv_Errors(t *testing.T) {
	t.Run("missing env var", func(t *testing.T) {
		unsetEnv(t, "CORE_FIELD_KEY_HEX")
		_, err := fieldcrypto.NewFromEnv()
		if err == nil {
			t.Error("expected error when CORE_FIELD_KEY_HEX is absent")
		}
	})

	t.Run("non-hex value", func(t *testing.T) {
		t.Setenv("CORE_FIELD_KEY_HEX", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
		_, err := fieldcrypto.NewFromEnv()
		if err == nil {
			t.Error("expected error for non-hex value")
		}
	})

	t.Run("wrong length hex", func(t *testing.T) {
		// 62 hex chars = 31 bytes (one byte short of 32).
		t.Setenv("CORE_FIELD_KEY_HEX", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
		_, err := fieldcrypto.NewFromEnv()
		if err == nil {
			t.Error("expected error for 31-byte key")
		}
	})

	t.Run("correct key succeeds", func(t *testing.T) {
		// 64 hex chars = 32 bytes.
		t.Setenv("CORE_FIELD_KEY_HEX", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
		c, err := fieldcrypto.NewFromEnv()
		if err != nil {
			t.Fatalf("expected success: %v", err)
		}
		if c == nil {
			t.Error("returned nil Cipher with valid key")
		}
	})
}

// TestConcurrent exercises Cipher under concurrent load for the -race detector.
func TestConcurrent(t *testing.T) {
	c := newTestCipher(t)
	const goroutines = 32
	const plaintext = "987-65-4321"

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			blob, err := c.Encrypt(plaintext)
			if err != nil {
				t.Errorf("Encrypt: %v", err)
				return
			}
			got, err := c.Decrypt(blob)
			if err != nil {
				t.Errorf("Decrypt: %v", err)
				return
			}
			if got != plaintext {
				t.Errorf("Decrypt = %q, want %q", got, plaintext)
			}
		}()
	}
	wg.Wait()
}
