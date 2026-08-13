# Add Blob Rotation Write-Back Queries

## Purpose and scope

Add the two narrow, compare-and-swap update queries that re-encrypt-on-read uses to persist a
replacement blob without touching any other column. These are the only write path the rotation
helper in Phase 3 will have.

Files this task owns:

- `model/queries/natural_persons.sql` — one added query.
- `model/queries/corporations.sql` — one added query.

This task does **not** run `make gen`, does not touch `model/db/`, and does not touch anything under
`api/`. It is independent of
[`001-replace-field-crypto-keys-schema.md`](./001-replace-field-crypto-keys-schema.md) — different
files, different tables — and the two may run concurrently. No standard skill covers this work.

## Requirements

1. **Add `UpdateNaturalPersonSSNBlob`** to `model/queries/natural_persons.sql`, exactly as specified
   in
   [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#adjacent-the-write-back-queries-phase-1-also-owns):

   ```sql
   -- name: UpdateNaturalPersonSSNBlob :execrows
   UPDATE natural_persons
   SET ssn = sqlc.arg(new_ssn)
   WHERE entity_id = sqlc.arg(entity_id) AND ssn = sqlc.arg(old_ssn);
   ```

2. **Add the `corporations.ein` equivalent**, `UpdateCorporationEINBlob`, to
   `model/queries/corporations.sql` with the same shape.
3. **Compare-and-swap, not a blind write.** The `AND ssn = sqlc.arg(old_ssn)` /
   `AND ein = sqlc.arg(old_ein)` predicate is load-bearing: an opportunistic write-back must never
   clobber a value another writer changed between the read and the write-back. Do not drop it and do
   not replace it with a bare `WHERE entity_id = $1`.
4. **`:execrows`, not `:exec`.** The affected-row count is the signal the Phase 3 policy branch
   adjudicates: zero rows means the stored blob is no longer the one that was read, which is a
   benign skip under a standard-rotation key and triggers a verification re-read under a compromised
   key. An `:exec` query discards exactly the information the caller needs.
5. **Touch no other column.** These queries deliberately do not reuse the `COALESCE($n, ssn)`
   "leave unchanged" idiom that `UpdateNaturalPerson` / `UpdateCorporation` use, because that idiom
   cannot express "replace this blob" and those queries would force the rotation write-back to
   re-supply every domain column.
6. **Add a short SQL comment above each query** naming its single purpose (re-encrypt-on-read
   write-back) and stating that the old-blob predicate is a compare-and-swap guard, matching the
   comment style of the surrounding queries.

## Validation

- `cd model && make verify` passes (`goose validate` + `sqlc compile`), proving both queries
  type-check against the existing schema.
- `grep -n "UpdateNaturalPersonSSNBlob" model/queries/natural_persons.sql` and
  `grep -n "UpdateCorporationEINBlob" model/queries/corporations.sql` each return exactly one
  `-- name:` line.
- `grep -n "execrows" model/queries/natural_persons.sql model/queries/corporations.sql` confirms both
  new queries use `:execrows`.
- Both new queries mention the old-blob column in their `WHERE` clause (the CAS guard is present).
- `git diff --stat` shows exactly two changed files, both under `model/queries/`.
- The pre-existing `UpdateNaturalPerson` and `UpdateCorporation` queries are unchanged.

## Assumptions

- `natural_persons` and `corporations` both key on `entity_id`, and every read path that decrypts
  already has that value in hand — `GetNaturalPersonByEntityID` and `GetCorporationByEntityID` both
  select it — so no site needs an extra lookup to identify its row.
- The encrypted column names are `natural_persons.ssn` and `corporations.ein`; confirm against the
  existing query file rather than assuming, and match the actual column names.

## References

- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#adjacent-the-write-back-queries-phase-1-also-owns)
  — the query text and the two design points (CAS guard, `:execrows`).
- [`../notes/rotation-on-read-call-sites.md`](../notes/rotation-on-read-call-sites.md#structural-obstacles-the-design-must-resolve)
  — obstacle 2, why the existing update queries cannot serve this purpose.
- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#cas-returning-zero-rows--resolving-schema-open-question-6)
  — how Phase 3 consumes the affected-row count.
