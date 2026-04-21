# Phase 8, Task 1 — fieldcrypto package

## Context
We need a small, well-tested AES-256-GCM helper used by the service layer
to encrypt tax ids (`natural_persons.ssn`, `corporations.ein`) before they
hit the database, and decrypt them on read.

The schema must remain vanilla SQL; all crypto is in Go.

## Location
`core-module/api/internal/fieldcrypto/`

- `fieldcrypto.go`      — the Cipher type + constructors.
- `fieldcrypto_test.go` — unit tests.

Use `internal/` so it is private to `core-module/api`.

## Public API (exactly)

```go
package fieldcrypto

// Cipher encrypts and decrypts small field values with AES-256-GCM.
// A single Cipher holds one 32-byte key; rotation is out of scope here.
type Cipher struct { /* aead cipher.AEAD */ }

// NewFromEnv reads CORE_FIELD_KEY_HEX (64 hex chars = 32 bytes) and
// returns a Cipher. Returns an error if the env var is missing, not hex,
// or wrong length.
func NewFromEnv() (*Cipher, error)

// NewFromKey builds a Cipher from a raw 32-byte key. Useful in tests.
func NewFromKey(key []byte) (*Cipher, error)

// Encrypt returns nonce(12) || ciphertext || tag(16) as a single blob.
// Returns ([]byte{}, nil) if plaintext == "" so callers do not have to
// special-case empty input. NOTE: an empty-input encryption still MUST
// return a zero-length blob — not an encrypted empty string — so the db
// layer can store NULL. Documented in the godoc.
func (c *Cipher) Encrypt(plaintext string) ([]byte, error)

// Decrypt reverses Encrypt. Returns ("", nil) when blob is empty or nil.
// Returns an error when the blob is non-empty but malformed / auth fails.
func (c *Cipher) Decrypt(blob []byte) (string, error)
```

## Implementation notes
- Use `crypto/aes`, `crypto/cipher`, `crypto/rand`.
- Nonce size: `aead.NonceSize()` (12 for GCM). Read from `crypto/rand.Reader`.
- No key derivation (the env var is already the raw key).
- No authenticated additional data. (We do not yet bind ciphertext to a
  specific row id; that is a later hardening step. Note this in a comment.)
- Keep the package dependency-free beyond stdlib.

## Tests (required)

1. Round-trip: `Encrypt("123-45-6789")` then `Decrypt(blob)` returns the
   same string.
2. Two encryptions of the same plaintext produce **different** blobs
   (nonce uniqueness).
3. `Encrypt("")` returns `len == 0` blob and `Decrypt(nil)` → `""`.
4. Tampering: flip one byte of a valid blob → `Decrypt` returns a non-nil
   error.
5. `NewFromEnv` errors: missing env var, non-hex, wrong length.
6. Concurrent use: run `Encrypt` + `Decrypt` from 32 goroutines, assert no
   data race (run with `-race`).

## Acceptance
- Files compile under `go vet ./...` and `go test -race ./...` green.
- Package imports only `crypto/*`, `encoding/hex`, `errors`, `fmt`, `os`.
- No package-level state; Cipher is the only exported type plus the two
  constructors.

## How to verify
```sh
cd core-module/api
go build ./...
go test -race ./internal/fieldcrypto/...
go vet ./internal/fieldcrypto/...
```

## Notes for the implementer
- If the module path differs from what you see in sibling packages, use
  whatever `go.mod` declares — the import path for the new package is
  `<module>/internal/fieldcrypto`.
- Do not wire this package into any service yet. That is task.4.
