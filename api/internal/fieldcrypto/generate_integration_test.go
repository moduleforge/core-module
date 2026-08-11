//go:build integration

package fieldcrypto_test

// generate_integration_test.go exercises NewFromEnvOrGenerate against a real,
// ephemeral Postgres database, proving the properties only a real DB-level
// uniqueness constraint can actually demonstrate: that concurrent first-boot
// callers converge on exactly one persisted key via the ON CONFLICT DO
// NOTHING race documented in fieldcrypto.go, and that the
// octet_length(key_bytes) = 32 CHECK constraint from
// model/migrations/0017_field_crypto_keys.sql genuinely rejects a corrupt
// key at the schema level.
//
// Structural precedent: api/authz/setup/grant_table_integration_test.go
// establishes the TestMain / prerequisite-check / host-resolution /
// shadow-DB-reset pattern this file follows, including its own load-bearing
// adaptations (own-migrations-directory resolution via runtime.Caller, and
// "localhost"-first Postgres host resolution verified by a real TCP dial).
// This file reuses that same reasoning without hand-copying it verbatim
// beyond what's needed for its own distinct shadow DB
// (core_field_key_verify_dev, distinct from core_grant_verify_dev on the
// same shared users-module-postgres container).
//
// Run with:
//
//	cd mod-core/api && \
//	  CORE_DEV_PG_HOST=<resolved-per-precedent> \
//	  go test -tags=integration -p 1 -v ./internal/fieldcrypto/...
//
// The -p 1 flag matches the authz/setup convention: TestMain drops and
// recreates the shadow database, so concurrent runs against the same
// Postgres host would conflict.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
	coredb "github.com/moduleforge/core-model/db"
)

// ---------------------------------------------------------------------------
// Package-level state
// ---------------------------------------------------------------------------

// pool is the connection pool for the core_field_key_verify_dev shadow
// database. Initialized by TestMain; closed after all tests complete.
var pool *pgxpool.Pool

// coreFieldKeyVerifyDB is this suite's dedicated shadow database, distinct
// from core_grant_verify_dev (authz/setup) and any other module's shadow DB
// on the same shared Postgres host.
const coreFieldKeyVerifyDB = "core_field_key_verify_dev"

// pgContainerName is the shared Postgres container reused by every
// integration test suite in this environment; this suite does not start its
// own container.
const pgContainerName = "users-module-postgres"

const (
	pgPort     = "5432"
	pgUser     = "users"
	pgPassword = "users"
)

// ---------------------------------------------------------------------------
// TestMain
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	if err := checkIntegrationPrerequisites(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: skipping mod-core field-key tests — %v\n", err)
		os.Exit(0) // exit 0 so `go test` reports skipped, not failed.
	}

	pgHost := resolvePostgresHost()
	if !postgresReachable(pgHost, 2*time.Second) {
		fmt.Fprintf(os.Stderr,
			"integration: skipping mod-core field-key tests — postgres not reachable at %s:%s\n",
			pgHost, pgPort)
		os.Exit(0)
	}

	if err := resetCoreFieldKeyVerifyDB(pgHost); err != nil {
		fmt.Fprintf(os.Stderr, "integration: shadow DB reset failed: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", pgUser, pgPassword, pgHost, pgPort, coreFieldKeyVerifyDB)
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: open pool: %v\n", err)
		os.Exit(1)
	}
	pool = p

	code := m.Run()
	pool.Close()
	os.Exit(code)
}

// checkIntegrationPrerequisites verifies that docker and goose are available
// and the shared Postgres container is running. Matches the structural
// precedent's prerequisite check.
func checkIntegrationPrerequisites() error {
	cmd := exec.Command("docker", "inspect", "--format={{.State.Running}}", pgContainerName)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("docker inspect: %w", err)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("container %s is not running", pgContainerName)
	}
	if _, err := exec.LookPath("goose"); err != nil {
		return fmt.Errorf("goose not in PATH: %w", err)
	}
	return nil
}

// resolvePostgresHost returns the host to use for connecting to the shared
// Postgres container. CORE_DEV_PG_HOST always wins when set. Otherwise it
// tries, in order, "localhost" (published-port forwarding), the container's
// Docker-network IP (docker inspect), and finally the historical hard-coded
// sandbox IP used by the structural precedent tests — verifying each
// candidate with an actual TCP dial before returning it (see the file-level
// doc comment).
func resolvePostgresHost() string {
	if h := os.Getenv("CORE_DEV_PG_HOST"); h != "" {
		return h
	}

	const dialTimeout = 500 * time.Millisecond
	if postgresReachable("localhost", dialTimeout) {
		return "localhost"
	}

	cmd := exec.Command("docker", "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", pgContainerName)
	if out, err := cmd.Output(); err == nil {
		if ip := strings.TrimSpace(string(out)); ip != "" && postgresReachable(ip, dialTimeout) {
			return ip
		}
	}

	// Historical fallback IP from the structural precedent tests, for
	// environments shaped like theirs (test binary as a Docker-network peer).
	return "172.23.0.3"
}

// postgresReachable reports whether a TCP connection to host:pgPort succeeds
// within timeout. It does not verify the Postgres wire protocol or
// credentials — only that something is listening.
func postgresReachable(host string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, pgPort), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// resetCoreFieldKeyVerifyDB drops and recreates core_field_key_verify_dev,
// then applies mod-core's own migrations (model/migrations, the 1-99 range)
// via goose. Idempotent: safe to re-run on every test binary invocation.
func resetCoreFieldKeyVerifyDB(pgHost string) error {
	ctx := context.Background()
	adminURL := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable", pgUser, pgPassword, pgHost, pgPort)

	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connect admin: %w", err)
	}
	defer conn.Close(ctx) //nolint:errcheck // best-effort close on an admin connection we're done with

	for _, stmt := range []string{
		"DROP DATABASE IF EXISTS " + coreFieldKeyVerifyDB,
		"CREATE DATABASE " + coreFieldKeyVerifyDB,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", pgUser, pgPassword, pgHost, pgPort, coreFieldKeyVerifyDB)
	cmd := exec.Command("goose", "-dir", migrationsDir(), "postgres", dsn, "up") //nolint:gosec // fixed args/resolved paths, not user input
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("goose up: %w\n%s", err, out)
	}
	return nil
}

// migrationsDir resolves mod-core's own model/migrations directory relative
// to this source file's location, so the test does not depend on a
// hand-typed absolute path tied to one machine's checkout layout. This file
// lives at <repo>/api/internal/fieldcrypto/generate_integration_test.go;
// migrations live at <repo>/model/migrations.
func migrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "model", "migrations")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// truncateFieldCryptoKeys empties field_crypto_keys so each test starts
// from a known "nothing persisted yet" state, independent of test order.
func truncateFieldCryptoKeys(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE field_crypto_keys"); err != nil {
		t.Fatalf("truncate field_crypto_keys: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestFieldKeyRealFetchOrGenerateRoundTrip proves the round trip through the
// real table, not just the Go-level control flow: a first call against an
// empty table generates and persists a key; a second call against the same
// DB fetches that same persisted key back, rather than generating another.
func TestFieldKeyRealFetchOrGenerateRoundTrip(t *testing.T) {
	ctx := context.Background()
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	truncateFieldCryptoKeys(t, ctx)

	q := coredb.New(pool)

	first, err := fieldcrypto.NewFromEnvOrGenerate(ctx, q)
	if err != nil {
		t.Fatalf("first NewFromEnvOrGenerate: %v", err)
	}

	second, err := fieldcrypto.NewFromEnvOrGenerate(ctx, q)
	if err != nil {
		t.Fatalf("second NewFromEnvOrGenerate: %v", err)
	}

	const plaintext = "round-trip-through-real-table"
	blob, err := first.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := second.Decrypt(blob)
	if err != nil {
		t.Fatalf("second.Decrypt(first's blob): %v (second call did not fetch the same persisted key)", err)
	}
	if got != plaintext {
		t.Errorf("second.Decrypt(first's blob) = %q, want %q", got, plaintext)
	}

	var rowCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM field_crypto_keys").Scan(&rowCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("field_crypto_keys row count = %d, want 1", rowCount)
	}
}

// TestFieldKeyRealConcurrentRace is the test that actually exercises the
// DB-level uniqueness constraint under real concurrency: N goroutines call
// NewFromEnvOrGenerate concurrently against independent *coredb.Queries
// values sharing the same pool, against an empty field_crypto_keys table.
// Every goroutine must succeed, exactly one row must exist afterward, and
// every goroutine's returned Cipher must have converged on that one row's
// key — not that Postgres serializes real concurrent writers (the unit
// tests can only prove the Go-level control flow handles a *simulated* lost
// race correctly).
func TestFieldKeyRealConcurrentRace(t *testing.T) {
	ctx := context.Background()
	unsetEnv(t, "CORE_FIELD_KEY_HEX")
	truncateFieldCryptoKeys(t, ctx)

	const goroutines = 8
	ciphers := make([]*fieldcrypto.Cipher, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			// Independent *coredb.Queries value sharing the pool, per the
			// requirement that this proves real concurrent writers converge,
			// not just that one shared Queries value is reused.
			q := coredb.New(pool)
			c, err := fieldcrypto.NewFromEnvOrGenerate(ctx, q)
			if err != nil {
				t.Errorf("goroutine %d: NewFromEnvOrGenerate: %v", i, err)
				return
			}
			ciphers[i] = c
		}(i)
	}
	wg.Wait()

	for i, c := range ciphers {
		if c == nil {
			t.Fatalf("goroutine %d did not produce a Cipher (see earlier error)", i)
		}
	}

	var rowCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM field_crypto_keys").Scan(&rowCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("field_crypto_keys row count after concurrent race = %d, want exactly 1", rowCount)
	}

	// Cross-check every goroutine's Cipher against the one persisted row's
	// actual key_bytes: build a reference Cipher directly from the row and
	// confirm every goroutine's Cipher round-trips against it.
	var persistedKey []byte
	if err := pool.QueryRow(ctx, "SELECT key_bytes FROM field_crypto_keys WHERE id = 1").Scan(&persistedKey); err != nil {
		t.Fatalf("fetch persisted key_bytes: %v", err)
	}
	reference, err := fieldcrypto.NewFromKey(persistedKey)
	if err != nil {
		t.Fatalf("NewFromKey(persisted key_bytes): %v", err)
	}

	for i, c := range ciphers {
		plaintext := fmt.Sprintf("probe-from-goroutine-%d", i)
		blob, err := c.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("goroutine %d cipher Encrypt: %v", i, err)
		}
		got, err := reference.Decrypt(blob)
		if err != nil {
			t.Errorf("goroutine %d: reference.Decrypt(its blob): %v (its Cipher did not converge on the winning key)", i, err)
			continue
		}
		if got != plaintext {
			t.Errorf("goroutine %d: reference.Decrypt(its blob) = %q, want %q", i, got, plaintext)
		}
	}
}

// TestFieldKeyRealCorruptionRejectedByCheckConstraint proves the DB-level
// defense-in-depth guard from 001-add-field-crypto-keys-table.md actually
// rejects a short key at the schema level, independent of the Go-level
// length check NewFromEnvOrGenerate/NewFromKey also performs. A normal
// insert path can never produce a corrupt row (the CHECK constraint rejects
// it before it lands) — so this test proves the constraint directly, via a
// raw INSERT attempting to bypass application-level validation entirely.
func TestFieldKeyRealCorruptionRejectedByCheckConstraint(t *testing.T) {
	ctx := context.Background()
	truncateFieldCryptoKeys(t, context.Background())

	shortKey := make([]byte, 16) // one CHECK (octet_length = 32) short
	_, err := pool.Exec(ctx, "INSERT INTO field_crypto_keys (id, key_bytes) VALUES (1, $1)", shortKey)
	if err == nil {
		t.Fatal("expected the octet_length(key_bytes) = 32 CHECK to reject a 16-byte key, but the insert succeeded")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected a *pgconn.PgError, got: %v", err)
	}
	const checkViolationCode = "23514" // Postgres SQLSTATE for check_violation
	if pgErr.Code != checkViolationCode {
		t.Errorf("expected SQLSTATE %s (check_violation), got %s: %v", checkViolationCode, pgErr.Code, err)
	}

	var rowCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM field_crypto_keys").Scan(&rowCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 0 {
		t.Errorf("expected no row to have been persisted by the rejected insert, got row count %d", rowCount)
	}
}
