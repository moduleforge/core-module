# Rotation On Read Call Sites

## Purpose and scope

Answers the question "is there a single choke point where re-encrypt-on-read can be wired, or does
every caller need updating individually?" — surveyed across mod-core during initial planning.
Records the call-site inventory and the structural obstacles; the choice between the wiring options
listed at the end is an open decision, not settled here.

## Answer: there is no single choke point

`Cipher.Decrypt` is a pure function over a byte slice. It has no database handle, no knowledge of
which table, column, or row a blob came from, and no way to write anything back. Re-encrypt-on-read
requires all three. Every decrypt site in mod-core is therefore a distinct write-back context.

## Inventory of decrypt sites (all in `api/service/`)

| # | File | Function | Blob source | Write-back target | Has a `Querier`? |
|---|------|----------|-------------|-------------------|------------------|
| 1 | `profile.go` | `ResolveProfileByEntityID` | `np.Ssn` | `natural_persons.ssn` | yes (`coredb.Querier` param) |
| 2 | `profile.go` | `ResolveProfileByEntityID` | `corp.Ein` | `corporations.ein` | yes (same param) |
| 3 | `legal_entity.go` | `LegalEntityService.GetTaxID` | `np.Ssn` | `natural_persons.ssn` | yes (param) |
| 4 | `legal_entity.go` | `LegalEntityService.GetTaxID` | `corp.Ein` | `corporations.ein` | yes (param) |
| 5 | `natural_person.go` | `NaturalPersonService.GetDecryptedSSN` | `np.Ssn` | `natural_persons.ssn` | yes (param) |
| 6 | `corporation.go` | `CorporationService.GetDecryptedEIN` | `corp.Ein` | `corporations.ein` | yes (param) |

Two distinct columns (`natural_persons.ssn`, `corporations.ein`), six call sites, four files.

`NaturalPersonService.GetByEntityUUID` funnels into site 1 by forwarding `s.cipher` to
`ResolveProfileByEntityID`, so the HTTP read path for a natural person reaches rotation through
`profile.go` rather than through its own decrypt call. `ResolveProfileByEntityID` is a
package-level function, not a service method — it takes the cipher as a variadic optional argument
and has no `txhelper.DB`, no observer group, and no authorizer.

## Structural obstacles the design must resolve

1. **No write handle, only a `Querier`.** Every site receives a `coredb.Querier`, which is
   satisfied by both a pool-backed and a tx-backed querier, so an `UPDATE` is *syntactically*
   reachable at each site. But nothing guarantees the caller's querier is writable, and nothing
   tells the site whether it is inside someone else's transaction.
2. **No blob-only update query exists.** `UpdateNaturalPerson` and `UpdateCorporation` set the
   name/jurisdiction columns alongside the encrypted column, using a `COALESCE($4, ssn)` idiom for
   "leave unchanged". Rotating a blob through those would require re-supplying every other column.
   New narrow queries (set only the encrypted column, by `entity_id`) are almost certainly needed —
   which is `model/queries/` work plus an sqlc regeneration.
3. **Observer semantics are undefined for a rotation write.** `ObserverGroup` fires on
   `create`/`update` and writes audit rows. A re-encryption changes stored bytes without changing
   the plaintext value. Whether that is an auditable `update` is an open question.
4. **Failure policy is undefined.** A read that cannot write back (read-only replica, read-only
   transaction, permission failure, concurrent writer) must either fail the read or silently skip
   rotation. Failing the read turns an infrastructure hiccup into a user-visible error on a plain
   `GET`; skipping it silently means rotation progress is unobservable.
5. **Rotation is lazy by construction.** A row that is never read is never rotated, so retired keys
   can never be discarded on a schedule — only after a full sweep proves no blob still carries the
   old version. This is a real operational consequence worth stating in the docs, and it argues for
   some way to observe rotation progress (a count of un-rotated rows, or a log/metric).

## Wiring options considered (not yet chosen)

- **A — rotation-aware decrypt returning a replacement blob.** `Decrypt` gains a sibling (e.g.
  `DecryptWithRotation(blob) (plaintext string, rotated []byte, err error)`) returning a non-nil
  `rotated` only when the blob's version is not the active version. Each of the six sites performs
  its own narrow `UPDATE`. Keeps fieldcrypto free of any persistence concern; costs six call-site
  edits plus two new queries.
- **B — write-back callback.** `DecryptAndRotate(ctx, blob, func(newBlob []byte) error)`. Same
  edits at the call sites, but the rotation policy (when to re-encrypt, what to do on write
  failure) lives in one place inside fieldcrypto.
- **C — a rotating wrapper around the column accessor.** A small helper in `api/service/` that
  pairs a decrypt with the matching narrow update query, one per encrypted column, and is called by
  all six sites. Concentrates the six sites onto two helpers.

All three still require the same two new sqlc queries and still touch all four service files.
None of them makes the change a one-file edit.
