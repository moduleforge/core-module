# Phase 8 — Report

Tax-id (SSN / EIN) with at-rest encryption. Plan lives in
`phase.8.tax-id-encryption/`.

## Status: complete

All 6 tasks executed. Verification green across both modules:

- `core-module/model make build` — clean.
- `atlas migrate validate` — clean.
- `core-module/api` — `go build ./...`, `go vet ./...`, `go test ./...` all green.
- `users-module/api` — `go build ./...`, `go vet ./...`, `go test ./...` all green.
- Secret-leakage grep: `CORE_FIELD_KEY_HEX` never appears as a literal
  assignment. The test fixture `"123-45-6789"` only appears in
  `api/service/tax_id_test.go`.

## What shipped

**Schema**
- `natural_persons.ssn BYTEA NULL` (migration `0012_natural_persons_ssn.sql`)
- `corporations.ein BYTEA NULL` (migration `0013_corporations_ein.sql`)
- `atlas.sum` refreshed.
- Columns hold a single opaque blob: `nonce(12) || ciphertext || gcm-tag(16)`.
- `NULL` = not recorded. Never an empty bytea in the application write path.

**Crypto package**
- `core-module/api/internal/fieldcrypto/` — AES-256-GCM `Cipher` + tests
  (round-trip, nonce uniqueness, tamper detection, concurrent use).
- `core-module/api/fieldcrypto/` — thin public re-export so `users-module`
  can import it without violating the `internal/` restriction.
- Key source: `CORE_FIELD_KEY_HEX` env var, 64 hex chars (32 bytes).
  Missing / malformed key → `NewFromEnv` error; service construction fails
  fast at startup.

**Service layer**
- `CreateNaturalPersonInput.SSN` (plaintext string, optional).
- `UpdateNaturalPersonInput.SSN *string` — three-state:
  - `nil` → leave unchanged (SQL `COALESCE($4, ssn)`).
  - `&""` → clear (store empty bytea).
  - `&"val"` → encrypt and store.
- Mirror API for `CorporationService.EIN`.
- `NaturalPersonService.GetDecryptedSSN` / `CorporationService.GetDecryptedEIN`
  return plaintext, `""` for NULL blobs, error only on decrypt failure.
- `LegalEntityService.GetTaxID` — abstract accessor returning
  `LegalEntityTaxID{Type, Value}` where `Type ∈ {"SSN", "EIN", ""}`.
  Dispatches on the entity's fundamental type; returns zero value (no error)
  for non-leaf-typed entities.
- `Profile` gained `TaxID` + `TaxIDType` fields, populated by
  `ResolveProfileByEntityID` when a cipher is supplied.
- Audit entries never include plaintext or ciphertext — only the markers
  `"set"`, `"cleared"`, `"unchanged"`.

**HTTP API**
- Create and Update request bodies accept `ssn` / `ein` (plaintext).
- Response gating: `tax_id` / `tax_id_type` are included IFF
  `principal.IsAdmin || principal.EntityID == profile.Entity.ID`.
  Otherwise the keys are omitted entirely (no `null`).
- `profileResponseFor(principal, profile)` is the new canonical helper;
  `profileResponse` delegates to it with a zero principal so legacy paths
  never emit tax_id.
- OpenAPI fragment (`core-module/api/openapi.fragment.yaml`) updated with:
  - `ssn` on Create/UpdateNaturalPersonRequest
  - `ein` on Create/UpdateCorporationRequest
  - `tax_id` + `tax_id_type` on Profile (both optional, not in `required`)

**users-module hookup**
- `users-module/api/cmd/server/main.go` now calls `fieldcrypto.NewFromEnv()`
  at startup, exits with code 1 on error, and passes the cipher to
  `coreservice.New(q, aw, cipher)`.

## Deviations from the original plan

1. **Public fieldcrypto façade.** The plan placed the package under
   `internal/fieldcrypto`. Because `users-module/api` is a separate Go
   module and must be able to call `NewFromEnv()` at startup, a public
   wrapper was added at `core-module/api/fieldcrypto/` that re-exports
   the internal API. The underlying implementation remains in the
   internal package. This is a mechanical accommodation of Go's internal
   visibility rules; the security posture is unchanged.

2. **Query column order.** Task 3's draft placed `ssn` / `ein` between
   `family_name` (or `jurisdiction`) and `created_at` in SELECT /
   RETURNING lists. Because the physical columns were appended by ALTER
   TABLE (migrations 0012/0013), sqlc emitted separate `*Row` types for
   each query — breaking four call sites in the service layer. Task 4
   reordered the SELECT / RETURNING lists to match physical column order
   (ssn/ein last), which restored use of the base `NaturalPerson` /
   `Corporation` structs. Semantics are unchanged.

3. **`ResolveProfileByEntityID` cipher propagation.** The variadic
   cipher parameter wasn't wired through `NaturalPersonService.GetByEntityUUID`
   or `CorporationService.GetByEntityUUID` by Task 4 — profiles read
   through the HTTP layer would have had empty `TaxID`. Task 5 fixed
   this by passing `s.cipher` into the resolver.

## Out of scope / flagged

- **Key rotation.** No mechanism for rotating the AES key. A future
  hardening step would embed a key-id prefix in the blob and introduce
  a re-encryption migration.
- **AAD binding.** Ciphertext is not bound to the row's primary key, so
  a ciphertext could in principle be copied between rows. Documented in
  a source comment as future work.
- **Clear-on-empty round-trip at the DB.** Integration-test confirmation
  needed against a live Postgres to verify `Cipher.Decrypt` on an
  empty-but-non-NULL bytea cleanly yields `""`. The unit tests assume
  this; the fake harness confirms the service branch; a live-DB check
  is still outstanding.
- **Corporation GET access control.** `getCorporation` has no
  admin-or-self enforcement gate (pre-existing behavior, not introduced
  by Phase 8). The tax_id response gate still correctly withholds `ein`
  for non-admin non-subject callers. Pre-existing issue worth raising
  separately — not a Phase 8 regression.

## How to try it

```sh
# one-time: generate a dev key and put it in your env
export CORE_FIELD_KEY_HEX=$(openssl rand -hex 32)

# start users-module/api as normal; it will now refuse to boot without
# the key. Then:
curl -X POST http://localhost:8080/v1/entities/natural-persons \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"given_name":"Jane","family_name":"Doe","ssn":"123-45-6789"}'

# Admin read — response contains tax_id + tax_id_type:"SSN":
curl http://localhost:8080/v1/entities/natural-persons/<uuid> \
  -H "Authorization: Bearer <admin-token>"
```

Verify encryption at rest by inspecting the raw column:

```sql
SELECT encode(ssn, 'hex') FROM natural_persons WHERE entity_id = <id>;
-- Must NOT contain the UTF-8 bytes of the plaintext SSN.
```

## Files of note

- Plan: `core-module/plan/phase.8.tax-id-encryption/`
- Migrations: `core-module/model/migrations/0012_natural_persons_ssn.sql`,
  `0013_corporations_ein.sql`
- Crypto: `core-module/api/internal/fieldcrypto/fieldcrypto.go`,
  `core-module/api/fieldcrypto/fieldcrypto.go`
- Services: `core-module/api/service/{natural_person,corporation,legal_entity,profile}.go`
- HTTP: `core-module/api/httpapi/{natural_persons,corporations,response}.go`
- Tests: `core-module/api/service/tax_id_test.go`,
  `core-module/api/httpapi/taxid_test.go`
- users-module: `users-module/api/cmd/server/main.go`
