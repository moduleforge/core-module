# Regenerate Model DB And Update Querier Stubs

## Purpose and scope

Regenerate the sqlc output for the query changes made by tasks 001 and 002, commit it, and repair
every in-repo implementation of the `coredb.Querier` interface so the `api` module builds and its
tests pass again. `model/sqlc.yaml` sets `emit_interface: true`, so adding or removing a query
changes an interface every stub in `api/` must satisfy.

Files this task owns:

- `model/db/` — regenerated, committed.
- The six `api/` test files that declare `var _ coredb.Querier = (*someStub)(nil)`.

Depends on both [`001-replace-field-crypto-keys-schema.md`](./001-replace-field-crypto-keys-schema.md)
and [`002-add-blob-rotation-write-back-queries.md`](./002-add-blob-rotation-write-back-queries.md)
being merged first. No standard skill covers this work.

## Requirements

1. **Regenerate and commit `model/db/`** with `cd model && make gen` (equivalently `sqlc generate`).
   Generated code is committed in this repo and must never be hand-edited — if the output looks
   wrong, fix the query or the schema and regenerate.
2. **Confirm the generated `FieldCryptoKey` model struct** matches the shape the design predicts:
   `Version int32`, `KeyBytes []byte`, `CreatedAt`/`UpdatedAt` as `pgtype.Timestamptz`, and
   `RetiredAt`/`DecryptableUntil`/`CompromisedAt` as `*time.Time` (the nullable-`timestamptz`
   override in `model/sqlc.yaml`). Report any deviation rather than working around it — Phase 2's
   adapter and Phase 4's handler are both written against this shape.
3. **Confirm the five key-material-bearing queries reuse `FieldCryptoKey`** rather than emitting
   per-query `...Row` structs, and that `ListFieldCryptoKeyMetadata` emits its own distinct row type
   with **no** `KeyBytes` field. If sqlc emitted a `...Row` type for a query that should have reused
   the model struct, fix the query's column list in `model/queries/field_crypto_keys.sql` and
   regenerate.
4. **Update every in-repo `coredb.Querier` implementation.** There are **six**, not the four the
   design notes estimated — find them fresh with
   `grep -rn "coredb.Querier = " api/` rather than trusting any list:
   - `api/types/types_test.go`
   - `api/entity/resolver_test.go`
   - `api/display/registry_test.go`
   - `api/httpapi/apps_test.go`
   - `api/httpapi/masked_lookup_test.go`
   - `api/service/mock_test.go`

   Each needs the two deleted methods (`GetFieldCryptoKey`, `InsertFieldCryptoKeyIfAbsent`) removed
   and the nine added methods (seven key queries plus the two blob CAS updates) added as no-op stubs
   returning zero values, in the style each file already uses for its other unused methods.
5. **Do not add real behavior to the stubs.** They exist to satisfy the interface. Any stub that a
   later phase's test actually needs to do something is that phase's task to change.
6. **Leave `api/internal/fieldcrypto` alone.** It declares its own `FieldKeyQuerier` interface and
   does not import `coredb`, so it still compiles against the old two-method contract. Phase 2
   replaces it.

## Validation

- `cd model && make verify` passes.
- `cd api && make build` passes (`go build ./...`).
- `cd api && make test` passes (`go test ./...`).
- `cd api && make lint` passes (`go vet ./...` plus the `gofmt` check).
- `git status` shows `model/db/` changes staged and committed — the generated code is not left
  untracked or reverted.
- `grep -rn "GetFieldCryptoKey\|InsertFieldCryptoKeyIfAbsent" api/ model/db/` returns nothing.
- `grep -rn "ListFieldCryptoKeyMetadata" model/db/` shows a generated row type carrying no
  `KeyBytes` field.
- `grep -rln "coredb.Querier = " api/` still returns six files, each of which compiles.

## Assumptions

- `api/internal/fieldcrypto/generate_integration_test.go` carries the `integration` build tag, so it
  is excluded from the default `go build ./...`, `go test ./...`, and `go vet ./...` runs. It
  references the deleted `GetFieldCryptoKey` / `InsertFieldCryptoKeyIfAbsent` queries and will not
  compile under `-tags integration` after this task. That is expected and accepted: Phase 2's cipher
  task rewrites that file wholesale. Do not attempt to repair it here, but do record the breakage in
  the task report.
- sqlc v1.28.0 is the pinned version (`model/Makefile`); if it is not installed, `make gen` is a
  no-op that silently leaves `model/db/` stale, which would make this task's validation pass while
  achieving nothing. Verify `sqlc version` before starting and halt if the tool is missing.

## References

- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#cost-this-imposes-on-coredbquerier-implementers)
  — why the stub churn exists and why the query list was kept short.
- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#generated-go-types) —
  the predicted `FieldCryptoKey` struct.
- `AGENTS.md` — [Code generation (sqlc)](../../../AGENTS.md) and the "generated code is committed and
  must not be edited by hand" convention.

## Checkpoint hints

- After `make gen` and committing `model/db/`.
- After the first three stub files compile.
- After all six stub files compile and `cd api && make test` passes.

## Status

- Outcome: succeeded (2026-08-13).
- `sqlc v1.31.1` (not the pinned v1.28.0) was used, per the dispatcher's operating-contract note; `cd model && make gen` ran cleanly against it with no compile errors.
- `model/db/` regenerated and committed. `FieldCryptoKey` matches the predicted shape exactly (`Version int32`, `KeyBytes []byte`, `CreatedAt`/`UpdatedAt` `pgtype.Timestamptz`, `RetiredAt`/`DecryptableUntil`/`CompromisedAt` `*time.Time`).
- Requirement 3 confirmed as corrected: `ListUsableFieldCryptoKeys`, `InsertInitialFieldCryptoKey`, `InsertActiveFieldCryptoKey` all reuse `FieldCryptoKey`; `RetireActiveFieldCryptoKey` returns a bare `int32`; `MarkFieldCryptoKeyCompromised` and `SetFieldCryptoKeyDecryptableUntil` emit narrow `...Row` types; `ListFieldCryptoKeyMetadataRow` carries no `KeyBytes` field. No query/schema changes were needed.
- All six `coredb.Querier` implementations found via `grep -rn "coredb.Querier = " api/` matched the task doc's predicted list exactly. Each had `GetFieldCryptoKey`/`InsertFieldCryptoKeyIfAbsent` removed and the nine new methods (seven key queries + `UpdateCorporationEINBlob`/`UpdateNaturalPersonSSNBlob`) added as no-op stubs returning zero values, in each file's existing alphabetical-by-method style. No real behavior was added.
- `gofmt` flagged a stray missing blank line in `api/httpapi/apps_test.go` left by the stub removal edit (mechanical whitespace artifact in a file already in this task's diff); fixed via `gofmt -w` as a same-diff self-fix.
- Affected source files: `model/db/*.go` (regenerated), `api/types/types_test.go`, `api/entity/resolver_test.go`, `api/display/registry_test.go`, `api/httpapi/apps_test.go`, `api/httpapi/masked_lookup_test.go`, `api/service/mock_test.go`.
- Confirmed expected/accepted breakage per `## Assumptions`: `api/internal/fieldcrypto` was left untouched (its own `FieldKeyQuerier` interface still compiles fine under default `go build`/`go test`/`go vet`), but `go vet -tags integration ./internal/fieldcrypto/...` now fails with `*db.Queries does not implement fieldcrypto.FieldKeyQuerier (missing method GetFieldCryptoKey)` in `generate_integration_test.go`, exactly as predicted. Phase 2's cipher task is expected to repair this.
- The task doc's `## Validation` grep check `grep -rn "GetFieldCryptoKey\|InsertFieldCryptoKeyIfAbsent" api/ model/db/` is not fully empty: it still matches `api/internal/fieldcrypto/fieldcrypto.go` and `api/internal/fieldcrypto/generate_test.go`, which declare and use those same method names on their own, separate `FieldKeyQuerier` interface (not `coredb.Querier`). This is the direct, unavoidable consequence of Requirement 6 ("leave `api/internal/fieldcrypto` alone") and is not a defect in this task's work — `model/db/` and all six `coredb.Querier` stub files are clean of those names.
