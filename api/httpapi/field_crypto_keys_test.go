package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/fieldcrypto"
	"github.com/moduleforge/core-api/observer"
	coredb "github.com/moduleforge/core-model/db"
)

// --- in-memory field_crypto_keys table ---

// fckKey is one row of the fake field_crypto_keys table.
type fckKey struct {
	version          int32
	material         []byte
	createdAt        time.Time
	updatedAt        time.Time
	retiredAt        *time.Time
	decryptableUntil *time.Time
	compromisedAt    *time.Time
}

// fckFakeQuerier is an in-memory field_crypto_keys table implementing the six
// key queries FieldCryptoKeyHandler and the cipher use. The embedded nil
// coredb.Querier satisfies the rest of the interface: a method this handler
// is not supposed to call panics rather than quietly returning a zero value.
//
// It enforces the same invariants the schema does — one active key, unique
// key material, flags only on retired rows — so the handler's 409 paths are
// exercised against the real failure shapes (pgx.ErrNoRows on a zero-row
// UPDATE, a 23505 pgconn.PgError on duplicate material) rather than stubs.
type fckFakeQuerier struct {
	coredb.Querier

	keys        []fckKey
	nextVersion int32

	listMetadataErr error
	listUsableErr   error
	retireErr       error
	insertErr       error
	markErr         error
	setGraceErr     error

	// listUsableCalls counts key-set loads, which is how the tests observe
	// whether the handler reloaded the cipher; onListUsable fires on each
	// one so a test can sample state at reload time.
	listUsableCalls int
	onListUsable    func()
}

func newFCKFakeQuerier() *fckFakeQuerier { return &fckFakeQuerier{} }

// seedKey appends a key row, returning its version.
func (q *fckFakeQuerier) seedKey(material []byte, retired bool) int32 {
	q.nextVersion++
	now := time.Now()
	k := fckKey{
		version:   q.nextVersion,
		material:  append([]byte(nil), material...),
		createdAt: now,
		updatedAt: now,
	}
	if retired {
		retiredAt := now.Add(-time.Hour)
		k.retiredAt = &retiredAt
	}
	q.keys = append(q.keys, k)
	return k.version
}

func (q *fckFakeQuerier) keyByVersion(version int32) *fckKey {
	for i := range q.keys {
		if q.keys[i].version == version {
			return &q.keys[i]
		}
	}
	return nil
}

func (q *fckFakeQuerier) activeKey() *fckKey {
	for i := range q.keys {
		if q.keys[i].retiredAt == nil {
			return &q.keys[i]
		}
	}
	return nil
}

func (q *fckFakeQuerier) row(k fckKey) coredb.FieldCryptoKey {
	return coredb.FieldCryptoKey{
		Version: k.version,
		// Freshly allocated on every call: the cipher zeroes the key bytes
		// it is handed once it has built its AEADs.
		KeyBytes:         append([]byte(nil), k.material...),
		CreatedAt:        pgtype.Timestamptz{Time: k.createdAt, Valid: true},
		UpdatedAt:        pgtype.Timestamptz{Time: k.updatedAt, Valid: true},
		RetiredAt:        k.retiredAt,
		DecryptableUntil: k.decryptableUntil,
		CompromisedAt:    k.compromisedAt,
	}
}

func (q *fckFakeQuerier) ListFieldCryptoKeyMetadata(_ context.Context) ([]coredb.ListFieldCryptoKeyMetadataRow, error) {
	if q.listMetadataErr != nil {
		return nil, q.listMetadataErr
	}
	rows := make([]coredb.ListFieldCryptoKeyMetadataRow, 0, len(q.keys))
	for _, k := range q.keys {
		rows = append(rows, coredb.ListFieldCryptoKeyMetadataRow{
			Version:          k.version,
			CreatedAt:        pgtype.Timestamptz{Time: k.createdAt, Valid: true},
			UpdatedAt:        pgtype.Timestamptz{Time: k.updatedAt, Valid: true},
			RetiredAt:        k.retiredAt,
			DecryptableUntil: k.decryptableUntil,
			CompromisedAt:    k.compromisedAt,
		})
	}
	return rows, nil
}

func (q *fckFakeQuerier) ListUsableFieldCryptoKeys(_ context.Context) ([]coredb.FieldCryptoKey, error) {
	q.listUsableCalls++
	if q.onListUsable != nil {
		q.onListUsable()
	}
	if q.listUsableErr != nil {
		return nil, q.listUsableErr
	}
	rows := make([]coredb.FieldCryptoKey, 0, len(q.keys))
	for _, k := range q.keys {
		if k.retiredAt != nil && k.decryptableUntil != nil && !k.decryptableUntil.After(time.Now()) {
			continue
		}
		rows = append(rows, q.row(k))
	}
	return rows, nil
}

func (q *fckFakeQuerier) InsertInitialFieldCryptoKey(_ context.Context, material []byte) (coredb.FieldCryptoKey, error) {
	if len(q.keys) > 0 {
		return coredb.FieldCryptoKey{}, pgx.ErrNoRows
	}
	return q.row(*q.keyByVersion(q.seedKey(material, false))), nil
}

func (q *fckFakeQuerier) InsertActiveFieldCryptoKey(_ context.Context, material []byte) (coredb.FieldCryptoKey, error) {
	if q.insertErr != nil {
		return coredb.FieldCryptoKey{}, q.insertErr
	}
	for _, k := range q.keys {
		if bytes.Equal(k.material, material) {
			return coredb.FieldCryptoKey{}, &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "field_crypto_keys_key_bytes_key",
				Message:        "duplicate key value violates unique constraint",
			}
		}
	}
	if q.activeKey() != nil {
		return coredb.FieldCryptoKey{}, &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "field_crypto_keys_one_active",
			Message:        "duplicate key value violates unique constraint",
		}
	}
	return q.row(*q.keyByVersion(q.seedKey(material, false))), nil
}

func (q *fckFakeQuerier) RetireActiveFieldCryptoKey(_ context.Context, arg coredb.RetireActiveFieldCryptoKeyParams) (int32, error) {
	if q.retireErr != nil {
		return 0, q.retireErr
	}
	active := q.activeKey()
	if active == nil {
		return 0, pgx.ErrNoRows
	}
	now := time.Now()
	active.retiredAt = &now
	active.updatedAt = now
	active.decryptableUntil = nil
	active.compromisedAt = nil
	if arg.GraceDays.Valid {
		until := now.AddDate(0, 0, int(arg.GraceDays.Int32))
		active.decryptableUntil = &until
	}
	if arg.Compromised {
		compromisedAt := now
		active.compromisedAt = &compromisedAt
	}
	return active.version, nil
}

func (q *fckFakeQuerier) MarkFieldCryptoKeyCompromised(_ context.Context, version int32) (coredb.MarkFieldCryptoKeyCompromisedRow, error) {
	if q.markErr != nil {
		return coredb.MarkFieldCryptoKeyCompromisedRow{}, q.markErr
	}
	k := q.keyByVersion(version)
	if k == nil || k.retiredAt == nil {
		return coredb.MarkFieldCryptoKeyCompromisedRow{}, pgx.ErrNoRows
	}
	if k.compromisedAt == nil {
		now := time.Now()
		k.compromisedAt = &now
		k.updatedAt = now
	}
	return coredb.MarkFieldCryptoKeyCompromisedRow{Version: version, CompromisedAt: k.compromisedAt}, nil
}

func (q *fckFakeQuerier) SetFieldCryptoKeyDecryptableUntil(_ context.Context, arg coredb.SetFieldCryptoKeyDecryptableUntilParams) (coredb.SetFieldCryptoKeyDecryptableUntilRow, error) {
	if q.setGraceErr != nil {
		return coredb.SetFieldCryptoKeyDecryptableUntilRow{}, q.setGraceErr
	}
	k := q.keyByVersion(arg.Version)
	if k == nil || k.retiredAt == nil {
		return coredb.SetFieldCryptoKeyDecryptableUntilRow{}, pgx.ErrNoRows
	}
	k.decryptableUntil = nil
	if arg.GraceDays.Valid {
		until := time.Now().AddDate(0, 0, int(arg.GraceDays.Int32))
		k.decryptableUntil = &until
	}
	k.updatedAt = time.Now()
	return coredb.SetFieldCryptoKeyDecryptableUntilRow{Version: arg.Version, DecryptableUntil: k.decryptableUntil}, nil
}

var _ coredb.Querier = (*fckFakeQuerier)(nil)

// --- recording observer ---

type fckObserveCall struct {
	op       string
	resource string
	target   *int64
	before   any
	after    any
}

type fckRecordingObserver struct {
	calls []fckObserveCall
}

func (o *fckRecordingObserver) Observe(_ context.Context, _ pgx.Tx, op, resource string, target *int64, before, after any) error {
	o.calls = append(o.calls, fckObserveCall{op: op, resource: resource, target: target, before: before, after: after})
	return nil
}

func (o *fckRecordingObserver) ObserveAfterCommit(_ context.Context, _, _ string, _ *int64, _ any) {}

var _ observer.MutationObserver = (*fckRecordingObserver)(nil)

// --- test harness ---

type fckHarness struct {
	q      *fckFakeQuerier
	az     *appsFakeAuthorizer
	obs    *fckRecordingObserver
	cipher *fieldcrypto.Cipher
	tx     *fakeAppsTx
	router chi.Router
}

// testKeyMaterial returns 32 non-zero bytes distinguishable by seed.
func testKeyMaterial(seed byte) []byte {
	material := make([]byte, 32)
	for i := range material {
		material[i] = seed + byte(i)
	}
	if material[0] == 0 {
		material[0] = 1
	}
	return material
}

// newFCKHarness wires a handler over a table holding one active key, with a
// store-less cipher (whose Reload is a no-op). Tests that care about the
// post-rotation reload use newFCKHarnessWithStoreCipher instead.
func newFCKHarness(t *testing.T) *fckHarness {
	t.Helper()
	q := newFCKFakeQuerier()
	q.seedKey(testKeyMaterial(1), false)
	cipher, err := fieldcrypto.NewFromKey(testKeyMaterial(1))
	if err != nil {
		t.Fatalf("build cipher: %v", err)
	}
	return newFCKHarnessWith(t, q, cipher)
}

// newFCKHarnessWithStoreCipher wires a handler whose cipher is backed by the
// same in-memory table, so Cipher.Reload actually re-queries it.
func newFCKHarnessWithStoreCipher(t *testing.T) *fckHarness {
	t.Helper()
	clearFieldKeyEnv(t)
	q := newFCKFakeQuerier()
	q.seedKey(testKeyMaterial(1), false)
	cipher, err := fieldcrypto.NewFromEnvOrGenerate(context.Background(), q)
	if err != nil {
		t.Fatalf("build store-backed cipher: %v", err)
	}
	return newFCKHarnessWith(t, q, cipher)
}

func newFCKHarnessWith(t *testing.T, q *fckFakeQuerier, cipher *fieldcrypto.Cipher) *fckHarness {
	t.Helper()
	az := &appsFakeAuthorizer{}
	obs := &fckRecordingObserver{}
	tx := &fakeAppsTx{}
	h := NewFieldCryptoKeyHandler(&fakeAppsDB{tx: tx}, q, az, observer.NewObserverGroup(obs), cipher)
	h.newQuerier = func(_ pgx.Tx) coredb.Querier { return q }
	r := chi.NewRouter()
	RegisterFieldCryptoKeyRoutes(r, h)
	return &fckHarness{q: q, az: az, obs: obs, cipher: cipher, tx: tx, router: r}
}

// clearFieldKeyEnv removes CORE_FIELD_KEY_HEX for the duration of a test that
// constructs a store-backed cipher: the variable is a first-boot seed that
// fails construction loudly when it does not match the active key.
func clearFieldKeyEnv(t *testing.T) {
	t.Helper()
	prior, ok := os.LookupEnv("CORE_FIELD_KEY_HEX")
	if !ok {
		return
	}
	if err := os.Unsetenv("CORE_FIELD_KEY_HEX"); err != nil {
		t.Fatalf("unset CORE_FIELD_KEY_HEX: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("CORE_FIELD_KEY_HEX", prior) })
}

// do issues an authenticated (actor-bearing) request against the harness.
func (h *fckHarness) do(method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = adminReq(method, path, nil)
	} else {
		req = adminReq(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body
}

// errorDetails returns the error envelope's field-level details.
func errorDetails(t *testing.T, rec *httptest.ResponseRecorder) []apiresp.FieldError {
	t.Helper()
	var env apiresp.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
	}
	return env.Error.Details
}

// assertNoKeyMaterial fails when body contains any encoding of material, or
// names a key-material column at all. It deliberately does not forbid the
// string "key_hex": that is the name of a request field, which a 400 or 409
// is required to name back at the operator.
func assertNoKeyMaterial(t *testing.T, body string, material []byte) {
	t.Helper()
	for _, forbidden := range []string{
		hex.EncodeToString(material),
		strings.ToUpper(hex.EncodeToString(material)),
		base64.StdEncoding.EncodeToString(material),
		"key_bytes",
		"keyBytes",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("response body leaks key material (%q): %s", forbidden, body)
		}
	}
}

// --- authorization gates, every route ---

// fckRoutes is every route this handler registers, with a body that would be
// valid if the request got past the authorization gate.
var fckRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"inventory", http.MethodGet, "/field-crypto-keys", ""},
	{"rotate", http.MethodPost, "/field-crypto-keys/rotations", `{}`},
	{"mark-compromised", http.MethodPost, "/field-crypto-keys/1/mark-compromised", ""},
	{"grace", http.MethodPut, "/field-crypto-keys/1/grace", `{"grace_period_days":null}`},
}

func TestFieldCryptoKeyRoutes_401_Unauthenticated(t *testing.T) {
	for _, tc := range fckRoutes {
		t.Run(tc.name, func(t *testing.T) {
			h := newFCKHarness(t)

			var req *http.Request
			if tc.body == "" {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			} else {
				req = httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			}
			rec := httptest.NewRecorder()
			h.router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
			}
			if h.az.lastAction != "" {
				t.Errorf("authorizer was consulted (%q) for an unauthenticated request", h.az.lastAction)
			}
			if len(h.obs.calls) != 0 {
				t.Errorf("observer dispatched %d times on an unauthenticated request", len(h.obs.calls))
			}
		})
	}
}

func TestFieldCryptoKeyRoutes_403_Forbidden(t *testing.T) {
	for _, tc := range fckRoutes {
		t.Run(tc.name, func(t *testing.T) {
			h := newFCKHarness(t)
			h.az.err = apiresp.ErrForbidden

			rec := h.do(tc.method, tc.path, tc.body)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			// Admin-only by construction: "manage" with a nil target is
			// denied for every actor except one holding a wildcard grant.
			if h.az.lastAction != "manage" {
				t.Errorf("authz action: got %q, want %q", h.az.lastAction, "manage")
			}
			if h.az.lastTarget != nil {
				t.Errorf("authz target: got %v, want nil", *h.az.lastTarget)
			}
			if len(h.q.keys) != 1 || h.q.activeKey() == nil {
				t.Errorf("denied request mutated the key table: %+v", h.q.keys)
			}
			if len(h.obs.calls) != 0 {
				t.Errorf("observer dispatched %d times on a denied request", len(h.obs.calls))
			}
		})
	}
}

// TestFieldCryptoKeyRotate_403_DoesNotReloadCipher pins the one amplification
// concern this handler owns: Cipher.Reload is unrate-limited, and this
// handler is the only caller of it, so a denied request must never reach it.
func TestFieldCryptoKeyRotate_403_DoesNotReloadCipher(t *testing.T) {
	h := newFCKHarnessWithStoreCipher(t)
	h.az.err = apiresp.ErrForbidden
	loadsBefore := h.q.listUsableCalls

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", `{}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
	if h.q.listUsableCalls != loadsBefore {
		t.Errorf("key-set loads: got %d, want %d — a denied request reached Cipher.Reload",
			h.q.listUsableCalls, loadsBefore)
	}
}

// --- GET /field-crypto-keys ---

func TestFieldCryptoKeyList_200(t *testing.T) {
	h := newFCKHarness(t)
	retired := h.q.seedKey(testKeyMaterial(7), true)

	rec := h.do(http.MethodGet, "/field-crypto-keys", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if h.az.lastAction != "manage" || h.az.lastTarget != nil {
		t.Errorf("authz: got (%q, %v), want (\"manage\", nil)", h.az.lastAction, h.az.lastTarget)
	}

	keys, ok := decodeBody(t, rec)["keys"].([]any)
	if !ok {
		t.Fatalf("keys: missing or not an array — body: %s", rec.Body.String())
	}
	if len(keys) != 2 {
		t.Fatalf("keys: got %d, want 2", len(keys))
	}
	first, _ := keys[0].(map[string]any)
	for _, field := range []string{"version", "created_at", "updated_at", "retired_at", "decryptable_until", "compromised_at"} {
		if _, present := first[field]; !present {
			t.Errorf("key row missing %q: %v", field, first)
		}
	}
	second, _ := keys[1].(map[string]any)
	if second["version"] != float64(retired) {
		t.Errorf("second version: got %v, want %d", second["version"], retired)
	}
	assertNoKeyMaterial(t, rec.Body.String(), testKeyMaterial(7))
	assertNoKeyMaterial(t, rec.Body.String(), testKeyMaterial(1))
}

func TestFieldCryptoKeyList_500_OnQueryError(t *testing.T) {
	h := newFCKHarness(t)
	h.q.listMetadataErr = errors.New("connection reset")

	rec := h.do(http.MethodGet, "/field-crypto-keys", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("500 body leaks the underlying error: %s", rec.Body.String())
	}
}

// --- POST /field-crypto-keys/rotations ---

func TestFieldCryptoKeyRotate_201_GeneratesKey(t *testing.T) {
	h := newFCKHarness(t)

	// An empty body is the recommended invocation: server-generated key,
	// standard rotation, no grace deadline.
	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	body := decodeBody(t, rec)
	retired, _ := body["retired"].(map[string]any)
	active, _ := body["active"].(map[string]any)
	if retired["version"] != float64(1) {
		t.Errorf("retired version: got %v, want 1", retired["version"])
	}
	if retired["retired_at"] == nil {
		t.Error("retired_at: got null, want a timestamp")
	}
	if retired["decryptable_until"] != nil {
		t.Errorf("decryptable_until: got %v, want null (the schema's safe default)", retired["decryptable_until"])
	}
	if retired["compromised_at"] != nil {
		t.Errorf("compromised_at: got %v, want null", retired["compromised_at"])
	}
	if active["version"] != float64(2) {
		t.Errorf("active version: got %v, want 2", active["version"])
	}
	if active["created_at"] == nil {
		t.Error("active created_at: got null, want a timestamp")
	}

	if len(h.q.keys) != 2 {
		t.Fatalf("key rows: got %d, want 2", len(h.q.keys))
	}
	newKey := h.q.keyByVersion(2)
	if len(newKey.material) != 32 {
		t.Errorf("generated key length: got %d, want 32", len(newKey.material))
	}
	if bytes.Equal(newKey.material, make([]byte, 32)) {
		t.Error("generated key is all zero bytes")
	}
	if bytes.Equal(newKey.material, testKeyMaterial(1)) {
		t.Error("generated key reuses the retired key's material")
	}
	assertNoKeyMaterial(t, rec.Body.String(), newKey.material)
}

func TestFieldCryptoKeyRotate_201_AcceptsSuppliedKeyHex(t *testing.T) {
	h := newFCKHarness(t)
	supplied := testKeyMaterial(42)

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations",
		fmt.Sprintf(`{"key_hex":%q}`, hex.EncodeToString(supplied)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if got := h.q.keyByVersion(2); !bytes.Equal(got.material, supplied) {
		t.Errorf("persisted key material does not match the supplied key_hex")
	}
	assertNoKeyMaterial(t, rec.Body.String(), supplied)
}

func TestFieldCryptoKeyRotate_201_Compromised(t *testing.T) {
	h := newFCKHarness(t)

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", `{"compromised":true}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	retired, _ := decodeBody(t, rec)["retired"].(map[string]any)
	if retired["compromised_at"] == nil {
		t.Error("compromised_at: got null, want a timestamp")
	}
	// A compromised rotation must leave the retired key decryptable
	// indefinitely: the maximum opportunity to re-encrypt away from it.
	if retired["decryptable_until"] != nil {
		t.Errorf("decryptable_until: got %v, want null", retired["decryptable_until"])
	}
}

func TestFieldCryptoKeyRotate_201_GracePeriod(t *testing.T) {
	h := newFCKHarness(t)

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", `{"grace_period_days":30}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	retired, _ := decodeBody(t, rec)["retired"].(map[string]any)
	if retired["decryptable_until"] == nil {
		t.Error("decryptable_until: got null, want a deadline 30 days out")
	}
	if until := h.q.keyByVersion(1).decryptableUntil; until == nil || until.Before(time.Now().AddDate(0, 0, 29)) {
		t.Errorf("persisted decryptable_until: got %v, want ~30 days out", until)
	}
}

// TestFieldCryptoKeyRotate_400_CompromisedWithGrace pins the schema-q5
// resolution: the combination is rejected, never silently overridden to NULL.
func TestFieldCryptoKeyRotate_400_CompromisedWithGrace(t *testing.T) {
	h := newFCKHarness(t)

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations",
		`{"compromised":true,"grace_period_days":30}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	details := errorDetails(t, rec)
	if len(details) != 1 || details[0].Field != "grace_period_days" {
		t.Errorf("details: got %+v, want one field error naming grace_period_days", details)
	}
	if h.q.activeKey() == nil || h.q.activeKey().version != 1 {
		t.Errorf("rejected request still rotated: %+v", h.q.keys)
	}
}

func TestFieldCryptoKeyRotate_400_InvalidInput(t *testing.T) {
	zeroHex := hex.EncodeToString(make([]byte, 32))
	cases := []struct {
		name  string
		body  string
		field string
	}{
		{"key_hex not hex", `{"key_hex":"zzzz"}`, "key_hex"},
		{"key_hex too short", `{"key_hex":"abcd"}`, "key_hex"},
		{"key_hex all zero", fmt.Sprintf(`{"key_hex":%q}`, zeroHex), "key_hex"},
		{"negative grace", `{"grace_period_days":-1}`, "grace_period_days"},
		{"grace beyond int32", fmt.Sprintf(`{"grace_period_days":%d}`, int64(math.MaxInt32)+1), "grace_period_days"},
		{"grace beyond tightened max", `{"grace_period_days":40000}`, "grace_period_days"},
		{"malformed json", `{"compromised":`, ""},
		{"unknown member", `{"compromize":true}`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newFCKHarness(t)

			rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if tc.field != "" {
				details := errorDetails(t, rec)
				if len(details) != 1 || details[0].Field != tc.field {
					t.Errorf("details: got %+v, want one field error naming %q", details, tc.field)
				}
			}
			if h.q.activeKey() == nil || h.q.activeKey().version != 1 {
				t.Errorf("rejected request still rotated: %+v", h.q.keys)
			}
			if strings.Contains(rec.Body.String(), zeroHex) {
				t.Errorf("400 body echoes the supplied key_hex: %s", rec.Body.String())
			}
		})
	}
}

// TestFieldCryptoKeyRotate_400_OversizedBody pins the local, per-route
// stopgap request-body cap (maxFieldCryptoKeyBodyBytes): a body well past
// what any valid rotation payload could need is rejected as invalid_input
// rather than being read into memory in full.
func TestFieldCryptoKeyRotate_400_OversizedBody(t *testing.T) {
	h := newFCKHarness(t)
	oversized := fmt.Sprintf(`{"key_hex":%q}`, strings.Repeat("a", maxFieldCryptoKeyBodyBytes+1))

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", oversized)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if h.q.activeKey() == nil || h.q.activeKey().version != 1 {
		t.Errorf("oversized request still rotated: %+v", h.q.keys)
	}
}

// TestFieldCryptoKeyGrace_400_OversizedBody covers the same cap on the
// handler's other body-accepting route.
func TestFieldCryptoKeyGrace_400_OversizedBody(t *testing.T) {
	h := newFCKHarness(t)
	retired := h.q.seedKey(testKeyMaterial(6), true)
	oversized := `{"grace_period_days":30,"padding":"` + strings.Repeat("a", maxFieldCryptoKeyBodyBytes+1) + `"}`

	rec := h.do(http.MethodPut, fmt.Sprintf("/field-crypto-keys/%d/grace", retired), oversized)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if h.q.keyByVersion(retired).decryptableUntil != nil {
		t.Error("oversized request still set decryptable_until")
	}
}

// TestFieldCryptoKeyRotate_409_NoActiveKey covers the lost concurrent
// rotation: the winner's UPDATE committed first, so this one's WHERE
// retired_at IS NULL matches nothing.
func TestFieldCryptoKeyRotate_409_NoActiveKey(t *testing.T) {
	h := newFCKHarness(t)
	now := time.Now()
	h.q.keys[0].retiredAt = &now

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", `{}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	details := errorDetails(t, rec)
	if len(details) != 1 || details[0].Code != "field_crypto_keys.no_active_key" {
		t.Errorf("details: got %+v, want the no_active_key conflict", details)
	}
	if len(h.q.keys) != 1 {
		t.Errorf("conflicting rotation still inserted a key: %+v", h.q.keys)
	}
}

// TestFieldCryptoKeyRotate_409_KeyMaterialInUse covers an operator
// re-supplying material already on file — notably a key retired as
// compromised, which key_bytes UNIQUE exists to keep out.
func TestFieldCryptoKeyRotate_409_KeyMaterialInUse(t *testing.T) {
	h := newFCKHarness(t)
	inUse := testKeyMaterial(1)

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations",
		fmt.Sprintf(`{"key_hex":%q}`, hex.EncodeToString(inUse)))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	details := errorDetails(t, rec)
	if len(details) != 1 || details[0].Field != "key_hex" {
		t.Errorf("details: got %+v, want one field error naming key_hex", details)
	}
	assertNoKeyMaterial(t, rec.Body.String(), inUse)
}

func TestFieldCryptoKeyRotate_500_OnInsertFailure(t *testing.T) {
	h := newFCKHarness(t)
	h.q.insertErr = errors.New("disk full")

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", `{}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "disk full") {
		t.Errorf("500 body leaks the underlying error: %s", rec.Body.String())
	}
	if !h.tx.rolledBack {
		t.Error("failed rotation did not roll back its transaction")
	}
}

func TestFieldCryptoKeyRotate_ObservesRotation(t *testing.T) {
	h := newFCKHarness(t)

	if rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", `{"compromised":true}`); rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if len(h.obs.calls) != 1 {
		t.Fatalf("observer calls: got %d, want 1", len(h.obs.calls))
	}
	call := h.obs.calls[0]
	if call.op != "rotate" {
		t.Errorf("op: got %q, want %q", call.op, "rotate")
	}
	if call.resource != "field_crypto_key" {
		t.Errorf("resource: got %q, want %q", call.resource, "field_crypto_key")
	}
	if call.target != nil {
		t.Errorf("target: got %v, want nil (key rows are not entities)", *call.target)
	}

	before, _ := call.before.(map[string]any)
	if before["version"] != int32(1) || before["retired_at"] != nil || before["compromised_at"] != nil {
		t.Errorf("before: got %+v, want version 1 in its pre-rotation state", before)
	}
	after, _ := call.after.(map[string]any)
	retired, _ := after["retired"].(map[string]any)
	if retired["compromised_at"] == nil || retired["version"] != int32(1) {
		t.Errorf("after.retired: got %+v, want version 1 flagged compromised", retired)
	}
	if active, _ := after["active"].(map[string]any); active["version"] != int32(2) {
		t.Errorf("after.active: got %+v, want version 2", after["active"])
	}

	// The audit payload is as key-material-free as the response body.
	encoded, err := json.Marshal(map[string]any{"before": call.before, "after": call.after})
	if err != nil {
		t.Fatalf("marshal observer payload: %v", err)
	}
	assertNoKeyMaterial(t, string(encoded), h.q.keyByVersion(2).material)
	assertNoKeyMaterial(t, string(encoded), testKeyMaterial(1))
}

// TestFieldCryptoKeyRotate_ReloadsCipherAfterCommit asserts the process that
// served the rotation converges immediately rather than waiting out the
// key-set TTL — and that it does so only after the transaction committed.
func TestFieldCryptoKeyRotate_ReloadsCipherAfterCommit(t *testing.T) {
	h := newFCKHarnessWithStoreCipher(t)
	loadsBefore := h.q.listUsableCalls

	var sawCommitted bool
	h.q.onListUsable = func() { sawCommitted = h.tx.committed }

	if rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", `{}`); rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if h.q.listUsableCalls != loadsBefore+1 {
		t.Fatalf("key-set loads: got %d, want %d", h.q.listUsableCalls, loadsBefore+1)
	}
	if !sawCommitted {
		t.Error("cipher reloaded before the rotation transaction committed")
	}

	// The reloaded set must encrypt under the new active version.
	blob, err := h.cipher.Encrypt(context.Background(), "secret")
	if err != nil {
		t.Fatalf("encrypt after rotation: %v", err)
	}
	version, err := fieldcrypto.BlobVersion(blob)
	if err != nil {
		t.Fatalf("blob version: %v", err)
	}
	if version != 2 {
		t.Errorf("encrypting under version %d after rotation, want 2", version)
	}
}

// TestFieldCryptoKeyRotate_ReloadFailureDoesNotFailRequest: the rotation is
// already durable when the reload runs, so a reload error is logged and the
// request still succeeds — this process converges on the key-set TTL instead.
func TestFieldCryptoKeyRotate_ReloadFailureDoesNotFailRequest(t *testing.T) {
	h := newFCKHarnessWithStoreCipher(t)
	h.q.listUsableErr = errors.New("key table unreachable")

	rec := h.do(http.MethodPost, "/field-crypto-keys/rotations", `{}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !h.tx.committed {
		t.Error("rotation transaction did not commit")
	}
	if len(h.q.keys) != 2 {
		t.Errorf("key rows: got %d, want 2 — the rotation must stand", len(h.q.keys))
	}
	if strings.Contains(rec.Body.String(), "key table unreachable") {
		t.Errorf("response leaks the reload error: %s", rec.Body.String())
	}
}

// --- POST /field-crypto-keys/{version}/mark-compromised ---

func TestFieldCryptoKeyMarkCompromised_200_AndIdempotent(t *testing.T) {
	h := newFCKHarness(t)
	retired := h.q.seedKey(testKeyMaterial(5), true)

	rec := h.do(http.MethodPost, fmt.Sprintf("/field-crypto-keys/%d/mark-compromised", retired), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	first := decodeBody(t, rec)
	if first["compromised_at"] == nil {
		t.Fatal("compromised_at: got null, want a timestamp")
	}
	if len(h.obs.calls) != 1 || h.obs.calls[0].op != "update" || h.obs.calls[0].resource != "field_crypto_key" {
		t.Errorf("observer calls: got %+v, want one update on field_crypto_key", h.obs.calls)
	}

	// Idempotent by query construction: a repeat call returns the original
	// timestamp rather than moving it forward.
	repeat := h.do(http.MethodPost, fmt.Sprintf("/field-crypto-keys/%d/mark-compromised", retired), "")
	if repeat.Code != http.StatusOK {
		t.Fatalf("repeat status: got %d, want %d", repeat.Code, http.StatusOK)
	}
	if second := decodeBody(t, repeat); second["compromised_at"] != first["compromised_at"] {
		t.Errorf("compromised_at moved on repeat: %v then %v", first["compromised_at"], second["compromised_at"])
	}
	assertNoKeyMaterial(t, rec.Body.String(), testKeyMaterial(5))
}

func TestFieldCryptoKeyMarkCompromised_404_UnknownVersion(t *testing.T) {
	h := newFCKHarness(t)

	rec := h.do(http.MethodPost, "/field-crypto-keys/99/mark-compromised", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

// TestFieldCryptoKeyMarkCompromised_409_ActiveKey: marking the active key
// compromised is a rotation, not a flag update — the response must say so.
func TestFieldCryptoKeyMarkCompromised_409_ActiveKey(t *testing.T) {
	h := newFCKHarness(t)

	rec := h.do(http.MethodPost, "/field-crypto-keys/1/mark-compromised", "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	details := errorDetails(t, rec)
	if len(details) != 1 || !strings.Contains(details[0].Message, "/v1/field-crypto-keys/rotations") {
		t.Errorf("details: got %+v, want a message naming the rotations route", details)
	}
	if h.q.keyByVersion(1).compromisedAt != nil {
		t.Error("the active key was flagged compromised")
	}
}

func TestFieldCryptoKeyMarkCompromised_400_BadVersion(t *testing.T) {
	for _, raw := range []string{"abc", "0", "-1"} {
		t.Run(raw, func(t *testing.T) {
			h := newFCKHarness(t)

			rec := h.do(http.MethodPost, "/field-crypto-keys/"+raw+"/mark-compromised", "")

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if details := errorDetails(t, rec); len(details) != 1 || details[0].Field != "version" {
				t.Errorf("details: got %+v, want one field error naming version", details)
			}
		})
	}
}

// --- PUT /field-crypto-keys/{version}/grace ---

func TestFieldCryptoKeyGrace_200_SetThenClear(t *testing.T) {
	h := newFCKHarness(t)
	retired := h.q.seedKey(testKeyMaterial(6), true)
	path := fmt.Sprintf("/field-crypto-keys/%d/grace", retired)

	rec := h.do(http.MethodPut, path, `{"grace_period_days":45}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if decodeBody(t, rec)["decryptable_until"] == nil {
		t.Error("decryptable_until: got null, want a deadline")
	}
	if until := h.q.keyByVersion(retired).decryptableUntil; until == nil {
		t.Error("persisted decryptable_until: got nil, want a deadline")
	}

	// An explicit null clears the deadline — the operator recovery path when
	// a window is about to close over data that has not been read yet.
	rec = h.do(http.MethodPut, path, `{"grace_period_days":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear status: got %d, want %d — body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if decodeBody(t, rec)["decryptable_until"] != nil {
		t.Error("decryptable_until: want null after clearing")
	}
	if until := h.q.keyByVersion(retired).decryptableUntil; until != nil {
		t.Errorf("persisted decryptable_until: got %v, want nil", until)
	}

	if len(h.obs.calls) != 2 {
		t.Fatalf("observer calls: got %d, want 2", len(h.obs.calls))
	}
	for _, call := range h.obs.calls {
		if call.op != "update" || call.resource != "field_crypto_key" || call.target != nil {
			t.Errorf("observer call: got %+v, want update/field_crypto_key/nil", call)
		}
	}
}

func TestFieldCryptoKeyGrace_400_InvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		field string
		code  string
	}{
		{"absent body", "", "grace_period_days", "field_crypto_keys.grace_period_days_required"},
		// A present-but-empty object omits grace_period_days entirely,
		// which decodes to the same zero value as an explicit
		// {"grace_period_days": null}. Left uncaught, that collapse would
		// silently clear the deadline on a truncated request — exactly the
		// failure mode setFieldCryptoKeyGraceRequest's doc comment says an
		// absent body is rejected to prevent, just reached via "{}" instead
		// of a zero-byte body.
		{"empty object", "{}", "grace_period_days", "field_crypto_keys.grace_period_days_required"},
		{"negative days", `{"grace_period_days":-5}`, "grace_period_days", ""},
		{"grace beyond tightened max", `{"grace_period_days":40000}`, "grace_period_days", "field_crypto_keys.grace_period_days_invalid"},
		{"malformed json", `{"grace_period_days":`, "", ""},
		{"unknown member", `{"grace_days":5}`, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newFCKHarness(t)
			retired := h.q.seedKey(testKeyMaterial(6), true)
			h.q.keyByVersion(retired).decryptableUntil = nil

			rec := h.do(http.MethodPut, fmt.Sprintf("/field-crypto-keys/%d/grace", retired), tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if tc.field != "" {
				details := errorDetails(t, rec)
				if len(details) != 1 || details[0].Field != tc.field {
					t.Errorf("details: got %+v, want one field error naming %q", details, tc.field)
				}
				if tc.code != "" && (len(details) != 1 || details[0].Code != tc.code) {
					t.Errorf("details: got %+v, want one field error coded %q", details, tc.code)
				}
			}
			if len(h.obs.calls) != 0 {
				t.Errorf("rejected request still observed a mutation: %+v", h.obs.calls)
			}
			if h.q.keyByVersion(retired).decryptableUntil != nil {
				t.Error("rejected request still set decryptable_until")
			}
		})
	}
}

func TestFieldCryptoKeyGrace_404_UnknownVersion(t *testing.T) {
	h := newFCKHarness(t)

	rec := h.do(http.MethodPut, "/field-crypto-keys/99/grace", `{"grace_period_days":30}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestFieldCryptoKeyGrace_409_ActiveKey(t *testing.T) {
	h := newFCKHarness(t)

	rec := h.do(http.MethodPut, "/field-crypto-keys/1/grace", `{"grace_period_days":30}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d — body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if details := errorDetails(t, rec); len(details) != 1 || !strings.Contains(details[0].Message, "/v1/field-crypto-keys/rotations") {
		t.Errorf("details: got %+v, want a message naming the rotations route", details)
	}
	if h.q.keyByVersion(1).decryptableUntil != nil {
		t.Error("a deadline was set on the active key")
	}
}

// --- key material never crosses the HTTP boundary ---

// TestFieldCryptoKeyRoutes_NeverReturnKeyMaterial walks all four routes on a
// table holding both a retired and an active key and asserts no response body
// carries key bytes in any encoding, nor names a key-material field.
func TestFieldCryptoKeyRoutes_NeverReturnKeyMaterial(t *testing.T) {
	h := newFCKHarness(t)
	retired := h.q.seedKey(testKeyMaterial(3), true)
	supplied := testKeyMaterial(8)

	responses := []*httptest.ResponseRecorder{
		h.do(http.MethodGet, "/field-crypto-keys", ""),
		h.do(http.MethodPost, fmt.Sprintf("/field-crypto-keys/%d/mark-compromised", retired), ""),
		h.do(http.MethodPut, fmt.Sprintf("/field-crypto-keys/%d/grace", retired), `{"grace_period_days":10}`),
		h.do(http.MethodPost, "/field-crypto-keys/rotations", fmt.Sprintf(`{"key_hex":%q}`, hex.EncodeToString(supplied))),
	}

	for i, rec := range responses {
		if rec.Code >= 400 {
			t.Fatalf("response %d: unexpected status %d — body: %s", i, rec.Code, rec.Body.String())
		}
		for _, material := range [][]byte{testKeyMaterial(1), testKeyMaterial(3), supplied} {
			assertNoKeyMaterial(t, rec.Body.String(), material)
		}
	}
}
