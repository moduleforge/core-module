// Package service — rotating_cipher.go
//
// RotatingCipher is the persistence half of re-encrypt-on-read. The split is
// deliberate: fieldcrypto answers "was this blob written under the active key,
// what is the replacement, and is persisting it mandatory?" without knowing a
// database exists, and this file answers "how do I persist a replacement blob
// for column X, and what do I do when I can't?" — once, in one four-branch
// core, rather than once per call site. Every decrypt call site becomes a
// one-line call carrying no policy logic at all.
//
// A write-back updates the subtype row (natural_persons / corporations), so
// natural_persons.updated_at and corporations.updated_at now mean "row last
// written", which includes a re-encryption that changed no domain field.
// Nothing in mod-core reads those columns, and the only client-observable
// timestamp — entities.updated_at, which the write-back never touches — still
// carries domain-modification semantics.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/moduleforge/core-api/internal/fieldcrypto"
	"github.com/moduleforge/core-api/txhelper"
	coredb "github.com/moduleforge/core-model/db"
)

// setRotationLockTimeout bounds how long the write-back transaction waits for
// a row lock before giving up. A caller-owned transaction may already hold a
// write lock on the very row being rotated; because that wait is not a lock
// cycle, Postgres's deadlock detector will never break it, and the read would
// hang for as long as the caller holds the lock. The timeout converts that
// hang into an ordinary write-back failure, which the policy branches below
// already know how to adjudicate. It is a fixed statement with nothing
// interpolated into it.
const setRotationLockTimeout = "SET LOCAL lock_timeout = '250ms'"

// pgerrcodeLockNotAvailable is Postgres' SQLSTATE for a statement that gave up
// waiting on a row lock — exactly what setRotationLockTimeout's SET LOCAL
// lock_timeout produces when it fires. writeBack retries once on this code
// before handing the outcome to decryptRotating's policy branches; see
// isLockTimeout and writeBack.
const pgerrcodeLockNotAvailable = "55P03"

// writeBackTimeout bounds writeBack's call to txhelper.Run end to end —
// acquiring a pool connection to begin the transaction, running the
// compare-and-swap, and committing — not merely the in-transaction row-lock
// wait that setRotationLockTimeout bounds. Matched to that same 250ms: if
// rc.db's pool is saturated (see the db field's doc comment on why that
// should never happen, but might), acquiring a second connection can block
// indefinitely with nothing in Postgres itself to time it out, since that
// wait happens before any statement — including the SET LOCAL above — ever
// runs. This timeout converts that hang into an ordinary write-back failure,
// which the policy branches in decryptRotating already know how to
// adjudicate.
const writeBackTimeout = 250 * time.Millisecond

// rotateOnReadEvent tags every structured log line the write-back emits, so
// rotation progress (or the lack of it) is greppable as one event stream.
const rotateOnReadEvent = "fieldcrypto.rotate_on_read"

// The three outcomes a rotate-on-read log line reports, at debug, warn, and
// error level respectively. outcomeStale is the single label for every
// tolerated miss — a lost compare-and-swap, a read-only replica, a permission
// error — with the underlying cause carried in the line's error field.
const (
	outcomePersisted = "persisted"
	outcomeStale     = "stale"
	outcomeError     = "error"
)

var (
	// errNoWriteHandle reports a RotatingCipher built without a write handle,
	// which disables write-back entirely. Under a standard rotation that is a
	// tolerated miss; under a compromised key it fails the read, exactly as a
	// write handle that could not commit would.
	errNoWriteHandle = errors.New("rotating cipher holds no write handle, so a re-encrypted blob cannot be persisted")

	// errStaleCAS reports a compare-and-swap that matched no row: the stored
	// blob is no longer the blob that was read.
	errStaleCAS = errors.New("the compare-and-swap matched no row, so the stored blob changed under the reader")

	// errStillCompromised reports that a lost compare-and-swap under a
	// compromised key left the stored blob still carrying the compromised
	// version — a genuinely unpersisted rotation rather than a benign race.
	errStillCompromised = errors.New("the stored blob still carries the compromised key version, so the replacement is genuinely unpersisted")
)

// RotatingCipher pairs the multi-key cipher with the write handle and the
// per-column update queries that re-encrypt-on-read needs. Services hold one
// of these where they would otherwise hold a bare *fieldcrypto.Cipher.
type RotatingCipher struct {
	// c performs the actual encryption, decryption, and rotation decision.
	c *fieldcrypto.Cipher

	// db is the write-back's own pool-backed handle — never the caller's
	// coredb.Querier. Issuing the update on the caller's querier is tempting
	// (no plumbing, inherits the caller's transaction) but it would make the
	// standard-rotation policy a lie: a failed statement inside a
	// caller-owned transaction aborts that transaction in Postgres, so
	// "tolerate the failure and return the plaintext" would leave the caller
	// holding a doomed transaction whose next statement fails with "current
	// transaction is aborted" — precisely on the read-only-replica and
	// read-only-transaction cases that motivated the tolerant policy in the
	// first place. A separate handle makes "log and skip" actually mean that.
	// A nil db disables write-back, which is what unit tests use.
	//
	// db must never be a handle a caller may already hold a connection from
	// (e.g. a pool a caller-owned transaction was checked out of). writeBack
	// opens its own transaction on db, which means acquiring a second pool
	// connection; if every connection in that pool is already held by
	// callers waiting on a rotating read of their own, the pool
	// self-deadlocks. writeBackTimeout bounds how long that acquisition (and
	// the transaction it begins) may block, which turns pool saturation into
	// an ordinary write-back failure instead of an indefinite hang — but it
	// mitigates the consequence, it does not eliminate the precondition:
	// callers are still expected to give RotatingCipher a handle backed by a
	// pool distinct from any transaction they may be holding a connection
	// from.
	db txhelper.DB

	// newQuerier derives a transaction-scoped querier; defaults to coredb.New.
	newQuerier func(pgx.Tx) coredb.Querier

	// log receives the rotate-on-read event stream; defaults to slog.Default().
	log *slog.Logger
}

// NewRotatingCipher constructs a RotatingCipher over c. A nil db disables
// write-back: rotation is then reported as an untolerated miss under a
// compromised key and a tolerated one otherwise. A nil log defaults to
// slog.Default().
func NewRotatingCipher(c *fieldcrypto.Cipher, db txhelper.DB, log *slog.Logger) *RotatingCipher {
	if log == nil {
		log = slog.Default()
	}
	return &RotatingCipher{
		c:          c,
		db:         db,
		newQuerier: func(tx pgx.Tx) coredb.Querier { return coredb.New(tx) },
		log:        log,
	}
}

// Encrypt forwards to the underlying cipher. Write paths always encrypt under
// the active key, so they need no rotation.
func (rc *RotatingCipher) Encrypt(ctx context.Context, plaintext string) ([]byte, error) {
	return rc.c.Encrypt(ctx, plaintext)
}

// DecryptSSN decrypts a natural_persons.ssn blob for entityID, re-encrypting
// and persisting it under the active key when it was written under an older
// one. It returns "" for an absent value, exactly as Cipher.Decrypt does.
func (rc *RotatingCipher) DecryptSSN(ctx context.Context, entityID int64, blob []byte) (string, error) {
	return rc.decryptRotating(ctx, ssnColumn, entityID, blob)
}

// DecryptEIN is the corporations.ein equivalent of DecryptSSN.
func (rc *RotatingCipher) DecryptEIN(ctx context.Context, entityID int64, blob []byte) (string, error) {
	return rc.decryptRotating(ctx, einColumn, entityID, blob)
}

// blobColumn is everything the rotation core needs to know about one encrypted
// column. Adding a third encrypted column later is one more of these plus one
// more two-line method; decryptRotating, writeBack, and verifyStale do not
// change.
type blobColumn struct {
	// name identifies the column in log fields and error text only. It is
	// never interpolated into SQL — both closures below run generated,
	// parameterized queries.
	name string

	// swap runs the compare-and-swap update, returning the number of rows it
	// matched. Zero means the stored blob is no longer the blob that was read.
	swap func(ctx context.Context, q coredb.Querier, entityID int64, oldBlob, newBlob []byte) (int64, error)

	// current re-reads the stored blob. It runs only on the compromised-key
	// verification path, and reuses the existing per-entity read query rather
	// than needing one of its own.
	current func(ctx context.Context, q coredb.Querier, entityID int64) ([]byte, error)
}

var ssnColumn = blobColumn{
	name: "natural_persons.ssn",
	swap: func(ctx context.Context, q coredb.Querier, entityID int64, oldBlob, newBlob []byte) (int64, error) {
		return q.UpdateNaturalPersonSSNBlob(ctx, coredb.UpdateNaturalPersonSSNBlobParams{
			EntityID: entityID, OldSsn: oldBlob, NewSsn: newBlob,
		})
	},
	current: func(ctx context.Context, q coredb.Querier, entityID int64) ([]byte, error) {
		np, err := q.GetNaturalPersonByEntityID(ctx, entityID)
		return np.Ssn, err
	},
}

var einColumn = blobColumn{
	name: "corporations.ein",
	swap: func(ctx context.Context, q coredb.Querier, entityID int64, oldBlob, newBlob []byte) (int64, error) {
		return q.UpdateCorporationEINBlob(ctx, coredb.UpdateCorporationEINBlobParams{
			EntityID: entityID, OldEin: oldBlob, NewEin: newBlob,
		})
	},
	current: func(ctx context.Context, q coredb.Querier, entityID int64) ([]byte, error) {
		corp, err := q.GetCorporationByEntityID(ctx, entityID)
		return corp.Ein, err
	},
}

// decryptRotating is the whole of the per-key failure policy, in four
// branches, for every encrypted column. Its branch structure is the policy:
// a fast path that costs nothing when there is nothing to rotate, a persisted
// path, a compromised-key path that fails the read, and a tolerated-miss path
// that does not.
func (rc *RotatingCipher) decryptRotating(
	ctx context.Context, col blobColumn, entityID int64, blob []byte,
) (string, error) {
	plaintext, rot, err := rc.c.DecryptWithRotation(ctx, blob)
	if err != nil || !rot.Needed() {
		// Fast path: a failed decrypt, an absent value, or a blob already
		// under the active key. No transaction, no log line.
		return plaintext, err
	}

	persisted, werr := rc.writeBack(ctx, col, entityID, blob, rot)
	switch {
	case persisted:
		rc.log.DebugContext(ctx, "fieldcrypto: rotated blob on read",
			rotateLogFields(col, entityID, rot, outcomePersisted)...)
		return plaintext, nil

	case rot.MustPersist:
		// The blob was written under a key that is known leaked, the
		// replacement is not durably stored, and — on a lost compare-and-swap
		// — verifyStale could not establish that the compromised ciphertext is
		// already gone. Returning the plaintext would leave data at rest under
		// the leaked key with nothing recording that it must be repaired, so
		// the read fails.
		rc.log.ErrorContext(ctx, "fieldcrypto: rotation away from a compromised key failed",
			rotateLogFields(col, entityID, rot, outcomeError, "error", werr)...)
		return "", fmt.Errorf("fieldcrypto: %s: re-encrypt away from compromised key version %d failed: %w",
			col.name, rot.FromVersion, werr)

	default:
		// A standard rotation inside its grace window is opportunistic: the
		// plaintext is valid, the retired key still decrypts, and the next
		// read of this row retries the rotation. Log so that a rotation which
		// is not progressing is observable.
		rc.log.WarnContext(ctx, "fieldcrypto: rotation write-back skipped",
			rotateLogFields(col, entityID, rot, outcomeStale, "error", werr)...)
		return plaintext, nil
	}
}

// writeBack persists rot.Blob over oldBlob in its own short read-write
// transaction on the helper's own handle, synchronously, in line with the
// read — never on the caller's querier (see RotatingCipher.db) and never in a
// goroutine, which would detach the failure from the read whose policy depends
// on it. It reports whether the replacement is durably stored; when it is not,
// the returned error is always non-nil, which is what lets the compromised-key
// branch above wrap a cause.
//
// The write-back dispatches no observer, deliberately, and a future reader
// should not add one: a re-encryption changes stored bytes and nothing else —
// the plaintext is identical and no domain field moved — the actor who
// triggered it may hold only read permission on the entity, and a rotation
// sweep would otherwise put one audit row in the log for every first read of
// every un-rotated row. Its record is the structured log line above.
//
// The write-back is not atomic with the read and does not need to be: the
// compare-and-swap on the exact blob that was read is what makes it safe,
// since the replacement lands only if the stored bytes are still the bytes
// that were decrypted. It commits or fails independently of any caller
// transaction; if the caller later rolls back, the rotation stands, which is
// harmless because the plaintext is unchanged. A rotation is never retried
// here either — a skipped one is retried by construction on the next read of
// that row.
//
// writeBack itself retries its single write-back attempt exactly once when
// that attempt fails on pgerrcodeLockNotAvailable (Postgres SQLSTATE 55P03,
// "lock_not_available") — the error setRotationLockTimeout's SET LOCAL
// lock_timeout produces when the compare-and-swap blocks behind a competing
// row lock. That contention is usually transient (e.g. a concurrent update of
// the same row's own subtype columns that releases its lock quickly), and a
// second attempt clears it far more often than not; this is a bounded,
// single retry, not a backoff loop, so sustained contention still fails the
// write-back exactly as it did before, just one attempt later.
func (rc *RotatingCipher) writeBack(
	ctx context.Context, col blobColumn, entityID int64, oldBlob []byte, rot fieldcrypto.Rotation,
) (bool, error) {
	persisted, err := rc.attemptWriteBack(ctx, col, entityID, oldBlob, rot)
	if err != nil && isLockTimeout(err) {
		persisted, err = rc.attemptWriteBack(ctx, col, entityID, oldBlob, rot)
	}
	return persisted, err
}

// attemptWriteBack runs the write-back's compare-and-swap exactly once. See
// writeBack, which retries this once on a lock-timeout error before returning
// to the caller.
func (rc *RotatingCipher) attemptWriteBack(
	ctx context.Context, col blobColumn, entityID int64, oldBlob []byte, rot fieldcrypto.Rotation,
) (bool, error) {
	if rc.db == nil {
		return false, errNoWriteHandle
	}

	// Bound the whole of Run — connection acquisition included, not just the
	// in-transaction lock wait setRotationLockTimeout covers — so pool
	// saturation degrades into an ordinary write-back failure the policy
	// branches in decryptRotating already handle, rather than hanging until
	// the caller's own request context expires. See writeBackTimeout and the
	// db field's doc comment.
	ctx, cancel := context.WithTimeout(ctx, writeBackTimeout)
	defer cancel()

	persisted := false
	err := txhelper.Run(ctx, rc.db, func(ctx context.Context, tx pgx.Tx) error {
		// First statement in the transaction, and the reason the write-back
		// uses an explicit transaction at all: a bare autocommit statement has
		// nowhere to hang a SET LOCAL.
		if _, e := tx.Exec(ctx, setRotationLockTimeout); e != nil {
			return fmt.Errorf("%s: bound the write-back lock wait: %w", col.name, e)
		}

		q := rc.newQuerier(tx)
		n, e := col.swap(ctx, q, entityID, oldBlob, rot.Blob)
		if e != nil {
			return fmt.Errorf("%s: compare-and-swap the re-encrypted blob: %w", col.name, e)
		}
		if n > 0 {
			persisted = true
			return nil
		}

		if !rot.MustPersist {
			// Under a standard rotation a lost compare-and-swap is a benign
			// skip and costs no extra query: whatever overwrote the row was
			// itself written under the active key, and anything left behind is
			// picked up by the next read.
			return errStaleCAS
		}
		if e := rc.verifyStale(ctx, q, col, entityID, rot); e != nil {
			return e
		}
		// verifyStale established that no ciphertext under the compromised
		// version remains in this column, which is what "persisted" is
		// protecting.
		persisted = true
		return nil
	})
	if err != nil {
		// A rollback or a failed commit means the replacement is not durably
		// stored, whatever the closure observed before it — so persisted must
		// never survive an error from Run. The compromised-key branch depends
		// on this.
		return false, err
	}
	return persisted, nil
}

// isLockTimeout reports whether err is (or wraps) a *pgconn.PgError carrying
// pgerrcodeLockNotAvailable — the SQLSTATE setRotationLockTimeout's SET LOCAL
// lock_timeout produces when a write-back statement gives up waiting on a row
// lock. writeBack uses it to decide whether a failed attempt is worth one
// retry.
func isLockTimeout(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcodeLockNotAvailable
}

// verifyStale runs only when the compare-and-swap matched no row under a
// compromised key. It re-reads the stored blob inside the same transaction and
// returns nil when the compromised ciphertext is verifiably gone — the
// plaintext the reader already holds was valid at read time, and nothing under
// the compromised version remains in that column — or an error when the
// rotation is genuinely unpersisted.
//
// The blanket alternative ("not persisted implies fail the read") is too
// blunt: the overwhelmingly likely cause of a lost compare-and-swap is that
// another reader already rotated the same row, which is an ordinary event, and
// failing a legitimate read for it turns a benign race into a user-visible 500
// under exactly the policy where reliability matters most.
//
// Two properties make this sound rather than a loophole:
//
//   - A compare-and-swap that matched no row cannot be a lock miss. Under READ
//     COMMITTED an UPDATE blocks on an uncommitted writer and re-evaluates its
//     predicate rather than returning zero rows, so zero rows always means a
//     committed change.
//   - Every legitimate write path encrypts under the active key, so a stored
//     blob is verifiably safe only when its version is ToVersion. Any other
//     version — FromVersion itself, or a second, also-compromised retired
//     version — is not reachable by a legitimate writer and is correctly a
//     hard failure rather than a benign race.
//
// It carries one conservative false negative: a caller running at REPEATABLE
// READ or SERIALIZABLE sees its own snapshot on the re-read and so reports a
// genuine race as unpersisted, failing the read. That is the safe direction.
func (rc *RotatingCipher) verifyStale(
	ctx context.Context, q coredb.Querier, col blobColumn, entityID int64, rot fieldcrypto.Rotation,
) error {
	cur, err := col.current(ctx, q, entityID)
	if err != nil {
		return fmt.Errorf("%s: re-read the stored blob after a lost compare-and-swap: %w", col.name, err)
	}
	if len(cur) == 0 {
		// The value was cleared outright: nothing remains under the old key.
		return nil
	}
	version, err := fieldcrypto.BlobVersion(cur)
	if err != nil {
		return fmt.Errorf("%s: decode the stored blob's key version after a lost compare-and-swap: %w", col.name, err)
	}
	if version == rot.ToVersion {
		// Someone else already re-encrypted it onto the active key.
		return nil
	}
	return errStillCompromised
}

// rotateLogFields builds the structured record every rotate-on-read log line
// carries. It names the column, the row, and the key versions involved, and
// never the plaintext or either blob.
func rotateLogFields(col blobColumn, entityID int64, rot fieldcrypto.Rotation, outcome string, extra ...any) []any {
	fields := make([]any, 0, 16)
	fields = append(fields,
		"event", rotateOnReadEvent,
		"column", col.name,
		"entity_id", entityID,
		"from_version", rot.FromVersion,
		"to_version", rot.ToVersion,
		// MustPersist mirrors compromised_at on the source key's row, so it is
		// exactly "the source key was compromised".
		"compromised", rot.MustPersist,
		"outcome", outcome,
	)
	return append(fields, extra...)
}
