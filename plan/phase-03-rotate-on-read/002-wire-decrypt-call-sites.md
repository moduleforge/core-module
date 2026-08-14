# Wire Rotation Through Decrypt Call Sites

## Purpose and scope

Route every decrypt in `api/service/` through the `RotatingCipher` so each one re-encrypts and
persists a stale blob, and move the four `Encrypt` sites onto the same wrapper. Each call site
becomes a one-line call with no policy logic of its own.

Files this task owns:

- `api/service/profile.go`, `legal_entity.go`, `natural_person.go`, `corporation.go`.
- `api/service/service.go` — composes the `RotatingCipher` internally.
- `api/service/mock_test.go` and any service test touching the cipher field.

Depends on [`001-rotating-cipher-helper.md`](./001-rotating-cipher-helper.md). No standard skill
covers this work.

## Requirements

1. **The six decrypt sites**, per the exact table in
   [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#the-six-call-sites-exactly):
   - `profile.go` `ResolveProfileByEntityID` — `c.Decrypt(np.Ssn)` → `c.DecryptSSN(ctx,
     np.EntityID, np.Ssn)`.
   - `profile.go` `ResolveProfileByEntityID` — `c.Decrypt(corp.Ein)` → `c.DecryptEIN(ctx,
     corp.EntityID, corp.Ein)`.
   - `legal_entity.go` `GetTaxID` — both the SSN and EIN branches.
   - `natural_person.go` `GetDecryptedSSN`.
   - `corporation.go` `GetDecryptedEIN`.

   `entity_id` is already on every row these sites load, so no site needs a new query or an extra
   parameter to identify its row.
2. **Field-type changes.** `NaturalPersonService.cipher`, `CorporationService.cipher`, and
   `LegalEntityService.cipher` move from `*fieldcrypto.Cipher` to `*RotatingCipher`.
3. **`ResolveProfileByEntityID`'s variadic parameter** moves from `...*fieldcrypto.Cipher` to
   `...*RotatingCipher`. This is safe: the function has no callers outside `api/service/` — every
   call is in `entity.go`, `corporation.go`, `natural_person.go`, or `service_account.go`. Verify
   that with a fresh grep before changing it, and update all in-package callers.
4. **The four `Encrypt` sites** (create and update in `natural_person.go`, create and update in
   `corporation.go`) go through `RotatingCipher.Encrypt` with the `ctx` already in scope.
5. **`service.New`'s exported signature must not change.** It already receives both `cipher
   *fieldcrypto.Cipher` and `db txhelper.DB`; compose the `RotatingCipher` inside it and hand that to
   the three services. This is load-bearing: `coreservice.New(coredb.New(pool), pool, az,
   observerGroup, fieldCipher, entityResolver, typeResolver)` is called verbatim from every
   composing app's generated `cmd/server/main.go`, so keeping it fixed means no app changes a line
   for this phase. Pass `nil` for the logger and let `NewRotatingCipher` default it.
6. **`api/service/mock_test.go`** constructs services by struct literal; pass
   `NewRotatingCipher(c, nil, nil)` wrapping the store-less cipher it already builds. A nil write
   handle means write-back is skipped, which is exactly right for a cipher that never reports a
   needed rotation.
7. **Call sites 3 and 4 are provably dead today** — `LegalEntityService` has no constructor, is
   absent from the `service.Services` aggregate, and its `cipher` field is unexported, so nothing can
   build one with a non-nil cipher. Update them so the package compiles and stays consistent, but do
   not invest in runtime testing for them, and do not "fix" the dead service as a side quest; note it
   in the task report instead.
8. **Service-layer tests.** Extend the existing service tests so that a decrypt of a blob written
   under a retired key returns correct plaintext through the service method, using the
   `RotatingCipher` with a nil write handle (the persistence path itself is covered by task 001's
   unit tests and task 003's integration test). Confirm no existing service test regressed.
9. **No policy logic at any call site.** No call site reads `Rotation`, `MustPersist`,
   `compromised_at`, or decides what to do on a write-back failure — all of that lives in
   `decryptRotating`. A reviewer should be able to see that each of the six sites is a single call.

## Validation

- `cd api && make build`, `cd api && make test`, and `cd api && make lint` all pass.
- `make build` at the repository root passes (all three sub-projects).
- `grep -rn "\.Decrypt(" api/service/ | grep -v _test` returns nothing — every decrypt in the service
  layer goes through `DecryptSSN`/`DecryptEIN`.
- `grep -rn "fieldcrypto.Cipher" api/service/ | grep -v _test` shows it only in `service.New`'s
  parameter list and in `rotating_cipher.go`'s struct field.
- `git diff api/service/service.go` shows no change to `func New(`'s parameter list or ordering.
- `grep -rn "MustPersist\|Rotation{" api/service/*.go | grep -v rotating_cipher` returns nothing — no
  policy leaked into a call site.
- All six sites from the design table are converted; confirm by grepping for `DecryptSSN` and
  `DecryptEIN` and counting the call sites.

## Metadata

architectural_impact: true

## Assumptions

- Task 001 has landed and `RotatingCipher` exposes `Encrypt`, `DecryptSSN`, and `DecryptEIN`.
- Phase 2 already threaded `ctx` mechanically through these same call sites; this task replaces those
  mechanical `Decrypt(ctx, …)` calls with the rotating equivalents. Expect to be editing lines Phase
  2 touched.
- `NaturalPersonService.GetByEntityUUID` reaches rotation through `profile.go` rather than through its
  own decrypt call, by forwarding `s.cipher` to `ResolveProfileByEntityID` — so converting site 1
  also converts that HTTP read path.

## References

- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#the-six-call-sites-exactly) — the
  exact before/after table and the list of supporting edits.
- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#findings-from-the-code-that-shape-the-design)
  — findings 1, 2, 3, and 5, which are what make these signature changes safe.
- [`../notes/rotation-on-read-call-sites.md`](../notes/rotation-on-read-call-sites.md#inventory-of-decrypt-sites-all-in-apiservice)
  — the original call-site inventory with blob sources and write-back targets.

## Checkpoint hints

- After the three service struct fields and `service.New` compose the `RotatingCipher`.
- After `profile.go`'s two sites and the variadic parameter change, with in-package callers updated.
- After `natural_person.go` and `corporation.go` (four decrypt/encrypt sites between them).
- After `legal_entity.go` and `mock_test.go`, with the full test suite green.

## Status

- **Outcome:** succeeded
- **Date:** 2026-08-13
- **Validation:** `cd api && make build`, `make test`, `make lint` all green; root `make build`
  (model, api, gui) green; all six documented grep checks pass (no bare `.Decrypt(` in a non-test
  service file; `fieldcrypto.Cipher` appears only in `service.New`'s parameter list and
  `rotating_cipher.go`; `git diff` on `service.go` shows no change to `func New(`'s parameter list;
  no `MustPersist`/`Rotation{` outside `rotating_cipher.go`; `DecryptSSN`/`DecryptEIN` each appear at
  exactly the six documented call sites plus their `rotating_cipher.go` definitions).
- **Files touched:** `api/service/service.go`, `api/service/profile.go`, `api/service/natural_person.go`,
  `api/service/corporation.go`, `api/service/legal_entity.go`, `api/service/mock_test.go`,
  `api/service/natural_person_test.go`, `api/service/corporation_test.go`, `api/service/tax_id_test.go`.
- **Assumptions applied:** task 001 had landed with `RotatingCipher` exposing `Encrypt`, `DecryptSSN`,
  `DecryptEIN`; `NaturalPersonService.GetByEntityUUID` reaches rotation through `profile.go`'s call
  site 1 by forwarding `s.cipher` to `ResolveProfileByEntityID` — confirmed unchanged by this task.
- **Decisions:** `service.New` composes exactly one `RotatingCipher` (via `NewRotatingCipher(cipher,
  db, nil)`) and shares it between `NaturalPersonService` and `CorporationService`, since both read
  through the same pool-backed write handle and default logger — no per-service instance needed.
  `LegalEntityService`'s `cipher` field was retyped to `*RotatingCipher` for compile-correctness only,
  per requirement 7; it remains unconstructed anywhere in the aggregate and received no new behavior,
  test coverage, or wiring. Added two new service-level rotation tests (`..._RotatesRetiredKey` in
  `tax_id_test.go`, one per column) per requirement 8, reusing `rotating_cipher_test.go`'s existing
  `newRotationTestCipher`/`rotationTestKey`/`rotationEncrypt` helpers rather than duplicating them.
- **Inline security review (review_focus):** applied inline against the diff from the task's start
  commit; no findings. Authorization ordering at every call site is unchanged (each site's existing
  `Authorize` call still runs before the profile/decrypt call it always did); `entity_id` threaded
  into `DecryptSSN`/`DecryptEIN` at every site is the same internal ID the caller already resolved and
  was authorized against, not new user input; no new sinks, secrets, or logging surface were
  introduced (`rotating_cipher.go`'s logging, which never logs plaintext or ciphertext, is unchanged
  by this task).
