# User question: rotation failure policy

Asked because when the opportunistic re-encrypt-on-read write-back fails (read replica, permission
error, concurrent writer), it's undefined today whether the read itself should fail or silently
proceed with the (still-valid) decrypted plaintext.

Presented options:

> Silently skip, log a warning (Recommended) — Return the decrypted plaintext to the caller
> regardless — rotation is opportunistic/best-effort, so a write-back hiccup shouldn't turn a plain
> GET into a user-visible error. Log so it's observable that rotation isn't progressing.

versus:

> Fail the read — If the re-encrypt write-back fails, the read fails too. Safer in the sense that
> rotation is never silently incomplete, but turns an infrastructure hiccup into a user-visible
> error on a plain read.

## Answer (verbatim)

"Make it an option in the key generation. For standard rotation, we might let the old keys work for
a set period of days. If we know a key is compromised, then we have to fail any reads that cannot
re-encrypt."

## Interpretation for planning

The user rejected picking a single global failure policy. Instead, failure policy is a **per-key
attribute set at key-generation/rotation time** (via the new admin API/CLI from
`key-lifecycle-policy.md`), with (at least) two rotation modes:

- **Standard rotation.** The retired key remains valid for decryption for a configurable grace
  period (a number of days, settable per rotation event — not a single hardcoded global constant).
  Within that window, a re-encrypt-on-read write-back failure is tolerated — read succeeds, skip +
  log (the "silently skip" behavior), matching the recommended default but scoped to this mode only.
- **Compromised-key rotation.** The retired key is flagged compromised at rotation time. For any
  blob still encrypted under a compromised key, if the re-encrypt-on-read write-back fails, **the
  read must fail** — serving plaintext (or leaving data at rest) under a known-compromised key
  without being able to immediately re-encrypt is unacceptable, per the user's explicit statement.

Open design points the research/design pass (`Re-encrypt-on-read API shape`, role `architect-backend`)
must resolve, since the user's answer sets policy but not full mechanics:

1. Where does "grace period" and "compromised" live in the schema — per-key-row columns in the
   redesigned `field_crypto_keys` table (e.g. `compromised boolean`, `grace_period_days int` or
   `retired_key_valid_until timestamptz`)? This should be designed together with
   `key-store-schema-design.md`.
2. What happens to a standard-rotation key once its grace period expires — does decryption of any
   remaining un-rotated blobs under that key simply start failing (forcing a hard error surfaced to
   the caller), or is there a separate reaping/alerting step? Not stated by the user; flag as still
   open if the design pass can't derive a clean answer from the stated policy alone.
3. Whether "compromised" can be set retroactively on an already-retired standard-rotation key (i.e.
   an operator discovers a key thought safe is actually compromised after the fact), not just at the
   moment of rotation — the user's phrasing ("if we know a key is compromised") suggests this should
   be settable independent of the rotation event itself, not only as a one-time flag chosen when the
   replacement key is generated. The design pass should propose a shape (e.g. a separate
   "mark-compromised" admin action distinct from "rotate") and flag it back to the user if genuinely
   ambiguous.

## Design

Produced by an `architect-backend` research pass, building directly on the completed
[`key-store-schema-design.md`](./key-store-schema-design.md). Everything from
[Decisions at a glance](#design-decisions-at-a-glance) through
[Phase task requirements](#phase-task-requirements) is implementable as written and is meant to be
lifted into Phase 2 (versioned cipher) and Phase 3 (rotate-on-read call sites) task requirements.
[Still open](#still-open) lists the two things a task author must not guess at.

Inputs treated as settled and not re-litigated: in-blob 4-byte big-endian version prefix bound as
AAD, DB-as-single-source-of-truth, `CORE_FIELD_KEY_HEX` as first-boot-only bootstrap that fails
loudly on mismatch thereafter, the replacement DDL and its six/seven sqlc queries, and the
clean-break ground rule (no backfill, no dual-format decode).

### Design decisions at a glance

| Question | Decision |
| --- | --- |
| Wiring option | Hybrid: option **A**'s rotation-aware decrypt primitive in `fieldcrypto`, option **C**'s per-column helper in `api/service/` calling it. Not B. |
| Where the write-back's DB handle comes from | A pool-backed `txhelper.DB` held by the helper — **not** the caller's `coredb.Querier` |
| Write-back transaction | Its own short read-write transaction, one per rotated blob, with `SET LOCAL lock_timeout` |
| Failure policy threading | `fieldcrypto` returns a `Rotation` value carrying `MustPersist`, derived from the source key's `compromised_at`; the call site never re-derives policy |
| CAS zero rows (schema q6) | Verify by re-reading the current blob's version prefix *inside the same write-back tx*; benign only if the stored blob is no longer under the compromised version |
| Observers on write-back | **None.** A re-encryption is not a domain mutation; structured log only |
| Admin surface | HTTP only — four routes on a new `FieldCryptoKeyHandler`. **No CLI**: mod-core ships no binary |
| Admin authz | `az.Authorize(ctx, "manage", nil)` — registered slug, nil target ⇒ wildcard-grant admins only |
| Active-key-compromised alarm state (schema q1) | **Not needed.** No schema change requested; the CHECK stays as designed |
| `updated_at` bump on write-back (schema q2) | **Accept and document.** No GUC suppression |
| Stale key sets (schema q3) | Reload-on-unknown-version (rate-limited) **plus** a TTL staleness bound on the encrypt path **plus** an explicit reload after a locally-served rotation |
| Compromised rotation forcing `decryptable_until = NULL` (schema q5) | **Yes**, and the API *rejects* `grace_period_days` alongside `compromised: true` rather than silently overriding |
| New queries beyond the schema note | One: `ListFieldCryptoKeyMetadata` (no `key_bytes`), for the admin inventory route |

### Findings from the code that shape the design

Four facts established by reading the current tree; each one removes an option or a risk.

1. **`LegalEntityService` is never constructed anywhere in the monorepo.** It has no constructor, is
   absent from the `service.Services` aggregate, and its `cipher` field is unexported — so no
   caller inside or outside mod-core can build one with a non-nil cipher. Call sites 3 and 4 of the
   six are therefore provably dead today (and would nil-panic if reached). Phase 3 must still update
   them so the package compiles, but they carry no runtime risk and need no integration test.
2. **`ResolveProfileByEntityID` has no callers outside `api/service/`.** Every call is in
   `entity.go`, `corporation.go`, `natural_person.go`, or `service_account.go`. Its exported,
   variadic-cipher signature can be changed without a cross-repo break.
3. **Nothing outside mod-core calls `Cipher.Encrypt` or `Cipher.Decrypt`.** mod-users and the apps
   construct a cipher and hand it to `coreservice.New`; they never invoke a method on it. Adding a
   `context.Context` parameter to both methods is a mod-core-internal change.
4. **`fieldcrypto.NewFromEnv()` *is* called by three out-of-repo composition roots** —
   `app-mfmanager/cmd/server/main.go:127`, `app-mfdemo/cmd/server/main.go:112`, and
   `mod-users/api/cmd/server/main.go:264` — even though mod-core's manifest declares
   `NewFromEnvOrGenerate`. This is the one exported symbol whose removal breaks other repositories.
5. **`natural_persons.updated_at` and `corporations.updated_at` are read by nothing.** The only
   `UpdatedAt` reference in `api/httpapi/` or `api/service/` is `entities.updated_at`, copied into
   `Profile.Entity`. A rotation write-back touches the subtype row, not the entities row. This is
   what makes schema open question 2 a clean "accept".

### A. Re-encrypt-on-read wiring

#### Why the hybrid, and why not B

Option **B** (write-back callback inside `fieldcrypto`) is rejected because it inverts the
dependency that the schema design worked to preserve: the callback's failure policy has to be
adjudicated inside `fieldcrypto`, which means `fieldcrypto` owns retry, logging, transaction, and
lock-timeout semantics for a database it deliberately knows nothing about. It also makes the
cipher's unit tests carry a persistence fake for every decrypt case.

Option **A** alone is rejected because "each of the six sites performs its own narrow `UPDATE`"
duplicates the failure-policy branch — the part that is easy to get subtly wrong and that a
compromised key makes security-relevant — six times.

The hybrid keeps each concern where it belongs:

- `api/internal/fieldcrypto` answers **"was this blob written under the active key, what is the
  replacement, and is persisting it mandatory?"** — pure, no DB, testable with a static key set.
- `api/service` answers **"how do I persist a replacement blob for column X, and what do I do when
  I can't?"** — one column-agnostic core plus one small `blobColumn` descriptor per encrypted
  column. Adding a third encrypted column later is a struct literal and a two-line method.
- The six call sites become one-line calls with no policy logic at all.

#### Phase 2 surface — `api/internal/fieldcrypto`

The `KeyRecord` / `KeyStore` split is exactly as
[`key-store-schema-design.md`](./key-store-schema-design.md#consequences-beyond-the-table)
specifies; nothing here widens it.

```go
// KeyRecord is fieldcrypto's own view of one field_crypto_keys row. It
// deliberately does not name any coredb type; the api/fieldcrypto façade
// adapts sqlc rows into it.
type KeyRecord struct {
	Version          uint32
	KeyBytes         []byte
	RetiredAt        *time.Time
	DecryptableUntil *time.Time
	CompromisedAt    *time.Time
}

// KeyStore is the persistence contract the multi-key cipher needs. Two
// methods, unchanged in count from today's FieldKeyQuerier.
type KeyStore interface {
	LoadUsableKeys(ctx context.Context) ([]KeyRecord, error)
	InsertInitialKey(ctx context.Context, keyBytes []byte) (KeyRecord, error)
}
```

`Cipher` stops being immutable and gains a reloadable key set:

```go
// keySet is an immutable snapshot. Reload swaps a whole new one in; readers
// never observe a partially-updated set.
type keySet struct {
	active   uint32
	byVersion map[uint32]*keyEntry // keyEntry pairs a KeyRecord with its built cipher.AEAD
	loadedAt time.Time
}

type Cipher struct {
	store    KeyStore                // nil for a static, store-less Cipher (NewFromKey)
	set      atomic.Pointer[keySet]  // swapped wholesale by reload
	reloadMu sync.Mutex              // collapses concurrent reloads into one query
	lastLoadAttempt atomic.Int64     // unix nanos; rate-limits unknown-version reloads
}
```

Three methods, all gaining `ctx` because any of them may trigger a reload:

```go
// Encrypt returns version(4) || nonce(12) || ciphertext || tag(16), with the
// 4 version bytes passed verbatim as the AEAD AAD. Returns ([]byte{}, nil)
// for empty plaintext, unchanged from today. May reload the key set first
// when the held set is older than keySetTTL.
func (c *Cipher) Encrypt(ctx context.Context, plaintext string) ([]byte, error)

// Decrypt returns plaintext only, for callers that cannot rotate. Still
// reloads on an unknown version, still enforces the grace deadline.
func (c *Cipher) Decrypt(ctx context.Context, blob []byte) (string, error)

// DecryptWithRotation decrypts blob and, when it was written under a
// non-active key version, also returns the replacement blob encrypted under
// the active key. Persisting the replacement is the caller's responsibility;
// Rotation.MustPersist states whether failing to do so must fail the read.
func (c *Cipher) DecryptWithRotation(ctx context.Context, blob []byte) (string, Rotation, error)
```

```go
// Rotation describes what a read owes the store about the blob it just read.
// The zero value means "nothing to do" — which is also what an empty blob and
// an already-current blob both produce.
type Rotation struct {
	// FromVersion is the key version that produced the blob; 0 for an empty blob.
	FromVersion uint32
	// ToVersion is the active key version at decrypt time.
	ToVersion uint32
	// Blob is the replacement, non-empty iff FromVersion != ToVersion and
	// FromVersion != 0.
	Blob []byte
	// MustPersist mirrors compromised_at on FromVersion's key row. When true,
	// a caller that cannot durably persist Blob MUST fail the read rather than
	// return the plaintext.
	MustPersist bool
}

// Needed reports whether Blob must be written back.
func (r Rotation) Needed() bool { return len(r.Blob) > 0 }
```

`MustPersist` is the whole of the policy threading: it is computed once, inside the cipher, from
the `CompromisedAt` field of the `KeyRecord` that decrypted the blob. No call site reads
`compromised_at`, none imports a key type, and adding a third policy mode later changes one
expression rather than six branches.

Behavioral requirements Phase 2 owes, in addition to the wire format the schema note already fixed:

- **Grace expiry is re-checked in Go at decrypt time** against `DecryptableUntil`, not only by the
  SQL load filter — a process that loaded a key hours ago must stop honoring it on expiry without a
  restart. An expired key behaves exactly like an unknown version.
- **Unknown version ⇒ one reload, then fail loudly.** Rate-limited by `lastLoadAttempt` (suggested
  minimum interval: 5s) so a stream of corrupt or hostile blobs cannot turn every read into a
  database query. After a reload that still does not know the version, the read fails.
- **A decoded version of `0` is a malformed blob** and fails without a reload attempt (the identity
  sequence starts at 1), per the schema note.
- **`Reload(ctx) error` is exported.** The admin rotation handler calls it on the process that just
  served a rotation so that process converges immediately rather than waiting for the TTL.
- **A store-less `Cipher` (from `NewFromKey`) pins version 1**, holds exactly that key, never
  reloads, and always reports `Rotation{}` from `DecryptWithRotation`. This is what keeps
  `api/service/mock_test.go` and the cipher's own round-trip tests working without a fake store.
- **Concurrency.** `Cipher` values stay safe for concurrent use; the `-race` test in
  `fieldcrypto_test.go` must be extended to exercise concurrent `Encrypt`/`Decrypt` *across a
  reload*, since that is the new hazard the atomic swap exists to close.

#### Phase 2 surface — the `api/fieldcrypto` façade

The façade absorbs the `coredb` import so the internal package keeps its zero-model-dependency
property, and — critically — so `moduleforge.module.yaml`'s `cipher` block stays byte-for-byte
unchanged (`constructor: fieldcrypto.NewFromEnvOrGenerate`, `args: [context, queries:coredb]`).

```go
// FieldKeyQuerier is the façade's persistence contract, satisfied structurally
// by *coredb.Queries and by coredb.Querier — so the manifest's queries:coredb
// arg source still type-checks unchanged.
type FieldKeyQuerier interface {
	ListUsableFieldCryptoKeys(ctx context.Context) ([]coredb.FieldCryptoKey, error)
	InsertInitialFieldCryptoKey(ctx context.Context, keyBytes []byte) (coredb.FieldCryptoKey, error)
}

func NewFromEnvOrGenerate(ctx context.Context, q FieldKeyQuerier) (*Cipher, error)
```

plus a ~30-line unexported `keyStoreAdapter` mapping `coredb.FieldCryptoKey` → `KeyRecord`
(`int32` → `uint32` on version, with a guard rejecting a negative value as corrupt).

**`NewFromEnv` must be deleted from both packages.** A cipher built from an env var alone has no
version number and therefore cannot legally encrypt anything under the settled DB-as-source-of-truth
model — keeping it would ship a constructor that produces blobs no other process can decrypt. This
is the one change in this plan that breaks compilation in other repositories:
`app-mfmanager/cmd/server/main.go:127`, `app-mfdemo/cmd/server/main.go:112`, and
`mod-users/api/cmd/server/main.go:264` each call it and must switch to `NewFromEnvOrGenerate(ctx,
coredb.New(pool))`. That is a **cross-repo followup** for the overview's deferred list, not a task
here — but it is a hard compile break rather than a doc staleness, so it should be filed distinctly
from the deploy-doc rewrites. `NewFromKey` stays exported (tests depend on it).

#### Phase 3 surface — `api/service/rotating_cipher.go`

One new file. `RotatingCipher` is what every service holds where it holds a `*fieldcrypto.Cipher`
today.

```go
// RotatingCipher pairs the multi-key cipher with the write handle and the
// per-column update queries that re-encrypt-on-read needs. Services hold one
// of these instead of a bare *fieldcrypto.Cipher.
type RotatingCipher struct {
	c          *fieldcrypto.Cipher
	db         txhelper.DB                  // write handle; nil disables write-back (tests)
	newQuerier func(pgx.Tx) coredb.Querier  // defaults to coredb.New
	log        *slog.Logger                 // defaults to slog.Default()
}

func NewRotatingCipher(c *fieldcrypto.Cipher, db txhelper.DB, log *slog.Logger) *RotatingCipher

// Encrypt forwards to the underlying cipher; write paths need no rotation.
func (rc *RotatingCipher) Encrypt(ctx context.Context, plaintext string) ([]byte, error)

// DecryptSSN decrypts a natural_persons.ssn blob for entityID, re-encrypting
// and persisting it under the active key when it was written under an older
// one. Returns "" for an absent value, exactly as Cipher.Decrypt does.
func (rc *RotatingCipher) DecryptSSN(ctx context.Context, entityID int64, blob []byte) (string, error)

// DecryptEIN is the corporations.ein equivalent.
func (rc *RotatingCipher) DecryptEIN(ctx context.Context, entityID int64, blob []byte) (string, error)
```

Both column methods are two lines over a shared core:

```go
func (rc *RotatingCipher) DecryptSSN(ctx context.Context, entityID int64, blob []byte) (string, error) {
	return rc.decryptRotating(ctx, ssnColumn, entityID, blob)
}

// blobColumn is everything the rotation core needs to know about one
// encrypted column. Adding a third encrypted column is one more of these.
type blobColumn struct {
	name    string // "natural_persons.ssn" — log field and error text only
	swap    func(ctx context.Context, q coredb.Querier, entityID int64, oldBlob, newBlob []byte) (int64, error)
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
```

`current` reuses the existing `GetNaturalPersonByEntityID` / `GetCorporationByEntityID` queries — the
zero-rows verification path needs no new query beyond the two CAS updates Phase 1 already owns.

The core, in full, because its branch structure *is* the failure policy:

```go
func (rc *RotatingCipher) decryptRotating(
	ctx context.Context, col blobColumn, entityID int64, blob []byte,
) (string, error) {
	plaintext, rot, err := rc.c.DecryptWithRotation(ctx, blob)
	if err != nil || !rot.Needed() {
		return plaintext, err            // fast path: empty, or already current
	}

	persisted, werr := rc.writeBack(ctx, col, entityID, blob, rot)
	switch {
	case persisted:
		rc.log.DebugContext(ctx, "fieldcrypto: rotated blob on read", /* fields */)
		return plaintext, nil
	case rot.MustPersist:
		// Compromised key, replacement not durably stored: the read fails.
		return "", fmt.Errorf("fieldcrypto: %s: re-encrypt away from compromised key version %d failed: %w",
			col.name, rot.FromVersion, werr)
	default:
		// Standard rotation inside its grace window: opportunistic, so the
		// read succeeds and the miss is observable.
		rc.log.WarnContext(ctx, "fieldcrypto: rotation write-back skipped", /* fields */)
		return plaintext, nil
	}
}
```

Everything policy-shaped is in those four branches, once.

#### The six call sites, exactly

| # | File | Current | Becomes |
| --- | --- | --- | --- |
| 1 | `profile.go` `ResolveProfileByEntityID` | `c.Decrypt(np.Ssn)` | `c.DecryptSSN(ctx, np.EntityID, np.Ssn)` |
| 2 | `profile.go` `ResolveProfileByEntityID` | `c.Decrypt(corp.Ein)` | `c.DecryptEIN(ctx, corp.EntityID, corp.Ein)` |
| 3 | `legal_entity.go` `GetTaxID` | `s.cipher.Decrypt(np.Ssn)` | `s.cipher.DecryptSSN(ctx, entityID, np.Ssn)` |
| 4 | `legal_entity.go` `GetTaxID` | `s.cipher.Decrypt(corp.Ein)` | `s.cipher.DecryptEIN(ctx, entityID, corp.Ein)` |
| 5 | `natural_person.go` `GetDecryptedSSN` | `s.cipher.Decrypt(np.Ssn)` | `s.cipher.DecryptSSN(ctx, entityID, np.Ssn)` |
| 6 | `corporation.go` `GetDecryptedEIN` | `s.cipher.Decrypt(corp.Ein)` | `s.cipher.DecryptEIN(ctx, entityID, corp.Ein)` |

`entity_id` is already on every row these sites load — `GetNaturalPersonByEntityID` and
`GetCorporationByEntityID` both select it — so no site needs a new query or an extra parameter to
identify its row.

Supporting edits, all mechanical:

- `NaturalPersonService.cipher`, `CorporationService.cipher`, and `LegalEntityService.cipher` change
  type from `*fieldcrypto.Cipher` to `*RotatingCipher`.
- `ResolveProfileByEntityID`'s variadic parameter changes from `...*fieldcrypto.Cipher` to
  `...*RotatingCipher`. Finding 2 above establishes this breaks no external caller.
- The two `Encrypt` sites (`natural_person.go` create/update, `corporation.go` create/update) gain
  `ctx` and go through `RotatingCipher.Encrypt`.
- **`service.New`'s signature does not change.** It already receives both `cipher
  *fieldcrypto.Cipher` and `db txhelper.DB`; it composes the `RotatingCipher` internally. This is
  load-bearing: `coreservice.New(coredb.New(pool), pool, az, observerGroup, fieldCipher,
  entityResolver, typeResolver)` is called verbatim from `mod-users/api/cmd/server/main.go:363`,
  `app-mftodo/cmd/server/main.go:177`, and every mfgen-generated main. Keeping it fixed means no
  composing app has to change a line for the call-site work.
- `api/service/mock_test.go` constructs services by struct literal; it passes
  `NewRotatingCipher(c, nil, nil)` with the store-less cipher it already builds. A nil `db` means
  write-back is skipped, which is exactly right for a cipher that never reports a needed rotation.

#### Where the write-back runs, and why not on the caller's querier

**Decision: a dedicated pool-backed `txhelper.DB` held by the `RotatingCipher`, in its own short
read-write transaction, synchronously, in line with the read. Never the caller's `coredb.Querier`,
never a goroutine.**

```go
func (rc *RotatingCipher) writeBack(
	ctx context.Context, col blobColumn, entityID int64, oldBlob []byte, rot fieldcrypto.Rotation,
) (persisted bool, err error) {
	if rc.db == nil {
		return false, errNoWriteHandle
	}
	err = txhelper.Run(ctx, rc.db, func(ctx context.Context, tx pgx.Tx) error {
		// A caller-owned transaction may hold a write lock on this row. Bound
		// the wait so a rotation can never hang a read; a timeout is just a
		// failed write-back, adjudicated by the policy branch above.
		if _, e := tx.Exec(ctx, "SET LOCAL lock_timeout = '250ms'"); e != nil {
			return e
		}
		q := rc.newQuerier(tx)
		n, e := col.swap(ctx, q, entityID, oldBlob, rot.Blob)
		if e != nil {
			return e
		}
		if n == 0 {
			return rc.verifyStale(ctx, q, col, entityID, rot) // see below
		}
		persisted = true
		return nil
	})
	return persisted, err
}
```

The alternative — issuing the `UPDATE` on the `coredb.Querier` the call site was handed — is
tempting because it needs no plumbing and inherits the caller's transaction. It is rejected for one
decisive reason and one supporting one:

1. **It makes the standard-rotation policy a lie.** A failed statement inside a caller-owned
   transaction aborts that transaction in Postgres. "Tolerate the write-back failure, log, and
   return the plaintext" would leave the caller holding a doomed transaction whose next statement
   fails with `current transaction is aborted`. The motivating failure mode from the question that
   produced this note — a read-only replica or read-only transaction — is precisely the case where
   this happens on *every* read. A separate handle makes "log and skip" actually mean that.
2. It also cannot distinguish a writable querier from a read-only one, so it has no way to decline
   cheaply.

The cost of the separate handle is a lock-ordering hazard: if the *caller's* transaction already
holds a row lock on the same row, an external CAS blocks behind it, and because that wait is not a
lock cycle Postgres's deadlock detector will not break it. `SET LOCAL lock_timeout = '250ms'`
converts that hang into a fast, ordinary write-back failure, which the policy branch already knows
how to adjudicate. This is why the write-back uses an explicit transaction at all — a bare
autocommit statement has nowhere to hang the `SET LOCAL`.

Consistency semantics that follow, and that the task doc should state:

- The write-back is **not atomic with the read**, and does not need to be. Compare-and-swap on the
  exact blob that was read is what makes it safe: the replacement lands only if the stored bytes are
  still the bytes that were decrypted.
- The write-back commits (or fails) independently of any caller transaction. If the caller later
  rolls back, the rotation stands — harmless, since the plaintext is unchanged.
- Rotation is **not retried**. A skipped rotation is retried by construction on the next read of
  that row.
- One rotated blob is one small transaction. `ResolveProfileByEntityID` rotates at most one column
  per call, so no read path issues more than one.

#### CAS returning zero rows — resolving schema open question 6

Zero rows affected means the stored blob is no longer the blob that was read. Under a
**standard-rotation** key this is a benign skip: log at `Warn` with `outcome=stale`, read succeeds,
done — no verification, no extra query.

Under a **compromised** key, the schema note's recommended blanket "not persisted ⇒ fail the read"
is rejected as too blunt. The overwhelmingly likely cause of a lost CAS is that another reader
already rotated the same row — two concurrent `GET`s on one profile, which is an ordinary event, not
an exotic one — and failing a legitimate read for it converts a benign race into a user-visible 500
under exactly the policy where reliability matters most. The refinement costs one query on a path
that only executes after a lost CAS under a compromised key:

```go
// verifyStale runs only when the CAS matched no row. It re-reads the current
// blob in the same transaction (a fresh statement snapshot under READ
// COMMITTED, so it sees the winner's committed value) and decides whether the
// compromised ciphertext is genuinely gone.
func (rc *RotatingCipher) verifyStale(...) error {
	cur, err := col.current(ctx, q, entityID)
	if err != nil { return err }
	if len(cur) == 0 { return nil }                       // value cleared: nothing left under the old key
	v, err := fieldcrypto.BlobVersion(cur)                 // new Phase 2 helper
	if err != nil { return err }
	if v != rot.FromVersion { return nil }                 // someone else re-encrypted it: done
	return errStillCompromised                             // genuinely unpersisted
}
```

A "success" from `verifyStale` sets `persisted = true`; the plaintext the reader already holds was
valid at read time, and no ciphertext under the compromised version remains in that column.

Two properties make this sound rather than a loophole. A CAS that matched no row cannot be a *lock*
miss — under READ COMMITTED an `UPDATE` blocks on an uncommitted writer and re-evaluates rather than
returning zero rows — so zero rows always means a committed change. And a stored blob still carrying
`FromVersion` after a committed change is not reachable by any legitimate writer, because every
write path encrypts under the active key; that case is correctly a hard failure.

The one conservative false negative: a caller running at `REPEATABLE READ` or `SERIALIZABLE` sees its
own snapshot on the re-read and will report a genuine race as unpersisted, failing the read. That is
the safe direction, and none of the six sites is reached from a serializable transaction today.

`BlobVersion(blob []byte) (uint32, error)` — decode the 4-byte prefix, reject length < 32 and
version 0 — is a small Phase 2 export that the rotation core, the tests, and any future operator
tooling all want.

#### Observers: none on the write-back

This resolves obstacle 3 of
[`rotation-on-read-call-sites.md`](./rotation-on-read-call-sites.md#structural-obstacles-the-design-must-resolve),
which no prior note assigned an owner.

A re-encryption changes stored bytes and nothing else: the plaintext is identical, no domain field
moved, and the actor who triggered it may hold only `read` permission on the entity. Firing the
`update` observer would attribute a mutation to a reader who is not permitted to mutate, and would
put one audit row in the log for every first read of every un-rotated row during a rotation sweep.
**The write-back dispatches no observer.** Its record is a structured log line:

```text
event=fieldcrypto.rotate_on_read column=natural_persons.ssn entity_id=… from_version=3
to_version=4 compromised=false outcome=persisted|stale|error
```

with levels `Debug` for `persisted`, `Warn` for a tolerated miss, and `Error` for a compromised-key
failure. A counter metric would be the right observability primitive, but mod-core has no metrics
facility today; that is a followup, not a task.

The *admin rotation event* is a different matter and **is** observed — see below.

### B. Admin rotation API

#### HTTP only. There is no CLI, and there cannot easily be one

mod-core contains no `cmd/` directory and produces no binary: it is a library module composed into a
host application whose `cmd/server/main.go` is generated. A `make`/CLI rotation command would mean
introducing mod-core's first binary, its own database configuration and connection handling, its own
packaging into every deploy image, and a second authorization story (a CLI has no bearer token and no
`opctx` actor). All of that to duplicate a route the host application already serves.

**Decision: four HTTP routes on a new `FieldCryptoKeyHandler`, following `AppsHandler` exactly.**
The operator runbook is `curl` against the deployed app; a future `mfmanager`-side CLI wrapper over
these routes is a reasonable cross-repo followup and needs nothing from mod-core.

#### Routes

Registered with `register:` onto the shared `/v1` group, the same way `RegisterAppRoutes` is, so
the paths resolve under `/v1/` without a second `Mount` (the gap-G14 duplicate-prefix panic the
manifest comments already document).

| Method | Path | Purpose | Success |
| --- | --- | --- | --- |
| `GET` | `/v1/field-crypto-keys` | Inventory: every version with its lifecycle timestamps. **Never key material.** | 200 |
| `POST` | `/v1/field-crypto-keys/rotations` | Rotate — standard or compromised | 201 |
| `POST` | `/v1/field-crypto-keys/{version}/mark-compromised` | Flag an already-retired key compromised after the fact | 200 |
| `PUT` | `/v1/field-crypto-keys/{version}/grace` | Extend, shorten, or clear a retired key's decrypt window | 200 |

**`POST /v1/field-crypto-keys/rotations`** — request:

```json
{ "key_hex": "…64 hex chars…", "compromised": false, "grace_period_days": 30 }
```

- `key_hex` — **optional**. Omitted (the recommended default) means the server generates 32
  cryptographically random bytes. Supplied means the operator brings their own material, decoded and
  length-validated exactly as `CORE_FIELD_KEY_HEX` is.
- `compromised` — default `false`. `true` stamps `compromised_at` on the key being retired.
- `grace_period_days` — optional non-negative integer, `null`/omitted meaning no expiry (the schema's
  safe default). **Rejected with 400 when `compromised` is `true`** — see the schema q5 resolution
  below.

Response (201), and the shape the inventory route reuses per key:

```json
{
  "retired": { "version": 3, "retired_at": "…", "decryptable_until": null, "compromised_at": null },
  "active":  { "version": 4, "created_at": "…" }
}
```

Implementation is the schema note's rotation transaction verbatim — `RetireActiveFieldCryptoKey`
then `InsertActiveFieldCryptoKey`, in that mandatory order, in one `txhelper.Run`, through
`h.querier(tx)`. After the transaction commits the handler calls `h.cipher.Reload(r.Context())` so
the process that served the rotation converges immediately; a reload error is logged and does not
fail the request, because the rotation itself is already durable.

Status mapping:

| Condition | Response |
| --- | --- |
| No actor on context | 401 (`opctx.ActorEntityID` miss, as every `AppsHandler` method starts) |
| Authorization denied | 403 |
| Malformed body, bad `key_hex`, negative days, or `compromised` + `grace_period_days` together | 400 `invalid_input` with a `FieldError` naming the offending field |
| `RetireActiveFieldCryptoKey` returns zero rows (concurrent rotation, or no active key) | 409 `apiresp.Conflict(...)` |
| `key_bytes` unique violation (operator re-supplied material already on file — notably a key retired as compromised) | 409 `apiresp.Conflict(...)`, message naming the cause |
| Otherwise | 500 |

The 409-on-zero-rows path is what makes the schema note's "two concurrent rotations: one wins, the
other errors and is retried" story reach the operator as something actionable rather than a 500.
`pg_advisory_xact_lock` stays unused; the unique index is already correct.

**`POST /v1/field-crypto-keys/{version}/mark-compromised`** — empty body, calls
`MarkFieldCryptoKeyCompromised`. Idempotent by query construction (`COALESCE(compromised_at,
now())`), so a repeat call returns 200 with the original timestamp. Zero rows means either an
unknown version or the still-active key; the handler distinguishes them with one `GET`-shaped lookup
against the inventory query and returns **404** for unknown and **409** for the active key, with a
message naming `POST /field-crypto-keys/rotations` as the action to take instead. This is the
"settable after the fact" action point 3 of the [Interpretation](#interpretation-for-planning) above
asked for, and the schema design already answered yes to.

**`PUT /v1/field-crypto-keys/{version}/grace`** — body `{ "grace_period_days": 30 }` or
`{ "grace_period_days": null }` to clear the deadline. Calls
`SetFieldCryptoKeyDecryptableUntil`. This is the operator recovery path when a window is about to
expire over data that has not been read yet, and the only supported way to put a deadline on a
compromised key (deliberate, explicit, after the fact — never as a silent side effect of a
rotation). 404/409 on zero rows, split the same way as `mark-compromised`.

**`GET /v1/field-crypto-keys`** needs one query the schema note deferred to this pass:

```sql
-- name: ListFieldCryptoKeyMetadata :many
SELECT version, created_at, updated_at, retired_at, decryptable_until, compromised_at
FROM field_crypto_keys
ORDER BY version;
```

Its narrower column list yields a distinct sqlc row type, which makes "no key material crosses this
boundary" a compile-time property rather than a review comment. It brings the Phase 1 query count
from six to seven, and adds one more no-op stub method to each of the four `coredb.Querier`
implementers in `api/` (`api/types/types_test.go`, `api/entity/resolver_test.go`,
`api/httpapi/apps_test.go`, `api/httpapi/masked_lookup_test.go`) — the churn the schema note warned
about. Task authors should budget it.

The inventory route is also the answer to "rotation is lazy, so how does an operator know when a
retired key can be discarded?" — it cannot answer that (nothing can, without scanning every
encrypted column in every module), but it does show which versions exist, when each was retired, and
which are flagged, which is what an operator needs before touching a grace window.

#### Handler shape and wiring

```go
type FieldCryptoKeyHandler struct {
	pool       txhelper.DB
	q          coredb.Querier
	newQuerier func(pgx.Tx) coredb.Querier // defaults to coredb.New
	az         authz.Authorizer
	observers  *observer.ObserverGroup
	cipher     *fieldcrypto.Cipher         // for the post-rotation local Reload
}

func NewFieldCryptoKeyHandler(
	pool txhelper.DB, q coredb.Querier, az authz.Authorizer,
	observers *observer.ObserverGroup, cipher *fieldcrypto.Cipher,
) *FieldCryptoKeyHandler

func RegisterFieldCryptoKeyRoutes(r chi.Router, h *FieldCryptoKeyHandler)
```

Field-for-field the `AppsHandler` shape, minus `entityResolver`/`typeResolver` (key rows are
deliberately not entities and have no type slug) and plus the cipher.

Manifest additions — a new service and a new route entry; **the `cipher` service block is
untouched**:

```yaml
    - name: coreFieldCryptoKeyHandler
      type: "*corehttpapi.FieldCryptoKeyHandler"
      constructor: corehttpapi.NewFieldCryptoKeyHandler
      args:
        - infra:pool
        - queries:coredb
        - service:authorizer
        - service:observerGroup
        - service:cipher

  routes:
    - prefix: /v1
      handler: coreFieldCryptoKeyHandler
      register: corehttpapi.RegisterFieldCryptoKeyRoutes
      scope: authenticated
```

This is additive: a composing app that does not regenerate simply does not expose the routes, and
nothing breaks. It is still a manifest change and belongs in the overview's regeneration note.

The four routes should also be added to `api/openapi.fragment.yaml`, which is where mod-core
documents its HTTP surface for composing specs.

#### Authorization

`AppsHandler`'s admin-only pattern is: check `opctx.ActorEntityID` for 401, then
`h.az.Authorize(ctx, verb, target)` where `target` is `nil` for collection-level operations. In the
reference `Authorizer` implementation (mod-users), **a nil target is denied outright for every actor
except one holding a wildcard grant** — the wildcard check short-circuits before the nil-target
denial. So "verb + nil target" *is* mod-core's admin-only idiom, and it is fail-closed by
construction.

**Every route in this handler authorizes as `h.az.Authorize(r.Context(), "manage", nil)`**, including
the read-only inventory route.

Two deliberate choices in that:

- **`"manage"`, not a new `"rotate"` verb.** Domain-specific verbs are permitted by the `Authorizer`
  contract, but the operation slug must be registered in the `OperationRegistry`, which lives in
  **mod-authz** — another repository, and out of this plan's scope. An unregistered slug falls
  through mod-users' `SatisfiedBy` error branch to a wildcard-`manage` check anyway, so inventing
  `"rotate"` would buy no additional granularity while depending on an error-handling fallback. If
  operations later wants a distinct grantable `rotate` operation, registering it in mod-authz and
  changing this one string is a clean, isolated follow-on.
- **`"manage"`, not `"list"`/`"read"`, for the inventory route.** The inventory returns no key
  material, but it does disclose the full rotation history and which keys are flagged compromised —
  security-operational information with no reason to be readable by anyone who cannot rotate. One
  authorization rule for the whole handler is also one fewer thing to get wrong.

The handler carries no ownership or entity-scoped path: `field_crypto_keys` rows are not entities,
have no `owner_id`, and get no `accessible_*_ids_for_actor` access function (the schema note is
explicit about this), so wildcard-grant admin is the only thing that can satisfy these routes.

#### Auditing the rotation itself

Unlike the read-path write-back, an admin rotation **is** a domain-significant, security-relevant
mutation and is dispatched to the observer group inside the rotation transaction, exactly as
`AppsHandler` does:

```go
h.observers.Observe(ctx, tx, "rotate", "field_crypto_key", nil /* no entity target */, before, after)
```

with `before`/`after` carrying versions, `retired_at`, `compromised_at`, and `decryptable_until` —
and **never `key_bytes`**. `ObserverGroup.Observe` accepts a `*int64` target, so `nil` is
well-formed for a non-entity resource. `mark-compromised` and `grace` observe as `"update"` on the
same resource string.

#### Schema open question 1 — active key known compromised, rotation pending

**Not needed. No schema change requested; keep `field_crypto_keys_retired_only_flags` exactly as
designed.**

The state would be an alarm with no behavior attached to it. Ask what the cipher should do when its
*active* key is flagged: it cannot stop encrypting (writes would fail application-wide), and it
cannot start encrypting under something else (there is nothing else). Meanwhile the two things an
operator actually wants — stop producing new ciphertext under the leaked key, and force reads to
re-encrypt away from it — are both delivered by rotating, which is a single API call with no
external prerequisite, since the endpoint generates the replacement key itself. There is no window
in which "we know it is compromised but we cannot rotate yet" is a real operational position.

Making it representable would also cost something concrete: the CHECK is what forces
"mark the active key compromised" to be atomic with introducing its replacement, and that atomicity
is exactly the property that stops a partially-applied panic response from leaving the system with
no active key.

### C. Cross-cutting resolutions

#### Schema open question 2 — `updated_at` bump on the write-back

**Accept and document. Do not build the GUC suppression.**

The evidence is stronger than the schema note assumed: `natural_persons.updated_at` and
`corporations.updated_at` are read by *nothing* in mod-core. The only `UpdatedAt` reference across
`api/httpapi/` and `api/service/` is `entities.updated_at`, copied into `Profile.Entity` — and the
rotation write-back updates the subtype row, not the entities row, so the timestamp any client can
actually observe is untouched. There is no ETag, no `If-Match`, no incremental-sync consumer.

The alternative would edit `set_updated_at()` in `0001_helpers.sql`, changing trigger behavior for
every table in the schema to suppress a timestamp nobody reads. The blast radius is wildly out of
proportion to the concern.

The task doc should carry one sentence: `natural_persons.updated_at` / `corporations.updated_at`
mean "row last written", which now includes a re-encryption; use `entities.updated_at` for
domain-modification semantics.

#### Schema open question 3 — stale key sets across processes

Three mechanisms, in order of necessity. The first two are Phase 2 requirements; the third is a
handler line.

1. **Reload on unknown version (mandatory).** The decrypt path, on meeting a version it does not
   hold, calls `LoadUsableKeys` once and retries before failing. Rate-limited by `lastLoadAttempt`
   (suggested 5s minimum interval) so corrupt or hostile blobs cannot amplify into a query storm.
   This alone makes a replica able to *read* everything a peer wrote after a rotation.
2. **A TTL staleness bound on the encrypt path (recommended, and I am specifying it as a
   requirement).** `Encrypt` reloads when the held set is older than `keySetTTL` (suggested 60s)
   before selecting the active key. Without it, reload-on-unknown-version leaves a replica happily
   *encrypting* new values under a retired key until it restarts — which for a **compromised**
   rotation means the system keeps writing fresh ciphertext under the leaked key indefinitely,
   defeating the entire point of the compromised mode. The cost is one trivial query per minute per
   process on a table with a handful of rows, on a code path (writing an SSN or EIN) that is
   inherently low-frequency. A reload failure here logs at `Error` and proceeds with the existing
   set: failing every write because the key table is briefly unreachable is worse than a bounded
   window of stale encryption.
3. **Explicit `Reload` after a locally-served rotation.** The rotation handler reloads its own
   process's cipher post-commit, so the replica the operator hit is correct immediately rather than
   within the TTL.

What a replica does when it discovers its own active key was retired underneath it: nothing special
— the reload replaces the whole `keySet`, the new active version becomes the encrypt target, and the
old key remains in the set as a retired-with-grace decrypt key. No error, no restart.

**Deferred, explicitly:** `LISTEN`/`NOTIFY` or any push-based invalidation; a configurable TTL
(the constant is fine until an operator asks); and any attempt to *prove* every replica has
converged — which would need process-level reporting mod-core has no channel for. The operational
consequence, which belongs in the app-mfmanager deploy-doc rewrite: after a **compromised** rotation,
restart the fleet rather than relying on the TTL, if you want a hard guarantee that no process is
still encrypting under the leaked key.

#### Schema open question 5 — does a compromised rotation force `decryptable_until = NULL`?

**Yes, and the API enforces it by rejection rather than by silent override.** A rotation request
carrying both `"compromised": true` and a non-null `grace_period_days` returns **400 invalid_input**
naming `grace_period_days`, rather than accepting the request and quietly writing `NULL`.

The reasoning for `NULL` is the schema note's and I agree with it: you want the maximum opportunity
to re-encrypt data away from a leaked key, and the compromised policy already guarantees that any
read which cannot re-encrypt fails — so nothing lingers readable-under-the-leaked-key without also
being repaired. Combining a compromise flag with a deadline converts un-rotated rows into
permanently unreadable rows on a timer: a data-destruction control wearing a security control's
clothes.

Rejection rather than override, because an operator who typed both had a model of the system that
disagrees with reality, and a 400 corrects it while a silent `NULL` leaves them believing the
deadline was set. An operator who genuinely wants a hard cutoff on a compromised key can still set
one, deliberately and visibly, via `PUT /v1/field-crypto-keys/{version}/grace` afterwards.

#### Schema open question 6 — CAS zero rows

Resolved in [section A](#cas-returning-zero-rows--resolving-schema-open-question-6): benign skip
under a standard key with no extra query; one same-transaction re-read of the stored blob's version
prefix under a compromised key, failing the read only when the compromised version is genuinely
still on disk. If a reviewer wants less code, the fallback is the schema note's blanket "not
persisted ⇒ fail the read", which is a strictly smaller change and differs only in that it produces
spurious 500s on concurrent reads of the same row during a compromised rotation.

### Phase task requirements

Additions to what the schema note's Phase 1 checklist already covers.

**Phase 1 (model layer) — two additions to the existing checklist**

1. Add `ListFieldCryptoKeyMetadata` (no `key_bytes`) as a seventh query.
2. Budget the `coredb.Querier` stub churn at seven added methods across four api test files, not
   six.

**Phase 2 (versioned cipher)**

1. `KeyRecord` / `KeyStore` in `api/internal/fieldcrypto`; the `coredb`→`KeyRecord` adapter and the
   `FieldKeyQuerier` interface in the `api/fieldcrypto` façade. Manifest `cipher` block unchanged.
2. `Cipher` reworked to a reloadable key set (`atomic.Pointer[keySet]`, reload mutex, load
   timestamp). Concurrency safety across a reload is a `-race` test requirement.
3. Wire format `version(4) || nonce(12) || ciphertext || tag(16)`, 4 prefix bytes as AAD, minimum
   blob length 32, version 0 rejected as malformed. `Encrypt("")` still returns `[]byte{}`.
4. `Encrypt(ctx, …)`, `Decrypt(ctx, …)`, `DecryptWithRotation(ctx, …)`, `Reload(ctx)`,
   `BlobVersion(blob)`, and the `Rotation` value type with `Needed()` and `MustPersist`.
5. Grace-deadline re-check in Go at decrypt time; reload-on-unknown-version with a minimum interval;
   `keySetTTL` reload on the encrypt path.
6. Bootstrap: `InsertInitialFieldCryptoKey` with the two-guard insert; zero rows ⇒ re-load and adopt;
   `CORE_FIELD_KEY_HEX` as first-boot seed only, exact-match-or-fail-loudly thereafter with an error
   naming `POST /v1/field-crypto-keys/rotations`.
7. **Delete `NewFromEnv` from both packages** (breaks three out-of-repo mains — file the cross-repo
   followup). Keep `NewFromKey` as a store-less, version-1, never-rotating cipher for tests.
8. Rewrite the package doc, which currently declares rotation out of scope and states "no AAD is
   bound to ciphertext" — the version prefix is now bound as AAD, so that paragraph needs
   re-scoping to "no *row identity* is bound", which remains true and remains a separate gap.
9. `fromPersistedOrGenerated` is rewritten wholesale here, which is the natural incidental place for
   followup `MnvB` (zeroing key byte slices) — non-blocking, not an acceptance criterion.

**Phase 3 (rotate-on-read call sites and the admin API)**

1. `api/service/rotating_cipher.go`: `RotatingCipher`, `NewRotatingCipher`, `blobColumn`,
   `ssnColumn`/`einColumn`, `decryptRotating`, `writeBack`, `verifyStale`.
2. Field-type changes on the three services, `ResolveProfileByEntityID`'s variadic parameter, and the
   six one-line call-site edits. `service.New`'s signature stays fixed.
3. `api/httpapi/field_crypto_keys.go`: the handler, four routes, request/response types,
   status mapping, post-rotation `Reload`, observer dispatch.
4. Manifest: add `coreFieldCryptoKeyHandler` service and its route entry. `cipher` block untouched.
5. `api/openapi.fragment.yaml`: document the four routes.
6. Tests worth naming as requirements, since they are the ones that would not be written by
   default: a compromised-key read whose CAS loses (must fail), a compromised-key read whose CAS
   loses to a *rotated* blob (must succeed), a standard-key read whose write-back errors (must
   succeed and log), and a write-back attempted with a nil write handle.
7. Integration-test the rotation endpoint's concurrency claim: two simultaneous rotations, one 201
   and one 409, never two active keys.

### Still open — now resolved by the user

1. **Transport-level protection for the rotation routes.** **Resolved: out of scope for this plan.**
   App-level authz (wildcard-grant-admin-only) is what this plan specifies. Network-level restriction
   (separate admin listener, ingress allow-list, mTLS) is a deployment concern, filed as a deferred
   cross-repo follow-on alongside the other app-mfmanager deploy-doc items this plan already defers —
   not a task in this plan.
2. **Whether `key_hex` should be accepted at all.** **Resolved: allow both.** The endpoint keeps the
   optional `key_hex` field for bring-your-own-key (HSM-derived, compliance-escrow) operators,
   alongside server-side generation as the default when it's omitted. No change needed to the design
   above — this was already the shape specified in section B; the user confirmed keeping it rather
   than the simpler generation-only alternative.

**Cross-repo followup to file, distinct from the deploy-doc items already listed in the overview:**
deleting `NewFromEnv` is a *compile break*, not documentation staleness, in
`app-mfmanager/cmd/server/main.go:127`, `app-mfdemo/cmd/server/main.go:112`, and
`mod-users/api/cmd/server/main.go:264`. Each must move to `NewFromEnvOrGenerate(ctx,
coredb.New(pool))` when it bumps its mod-core pin — and, because there is no backfill path, recreate
its database at the same time.
