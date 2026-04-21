# Phase 8, Task 4 — Service layer wiring

## Context
Blocked by task.1 (fieldcrypto) and task.3 (generated db types).

This task:
1. Extends `CreateNaturalPersonInput` / `UpdateNaturalPersonInput` /
   `CreateCorporationInput` / `UpdateCorporationInput` to accept a
   plaintext tax id.
2. Encrypts before writes; decrypts on reads; surfaces plaintext in
   returned profile structs only via explicit accessors (not as a raw
   field on the persisted record).
3. Adds an abstract accessor on LegalEntity:
   `GetTaxID(ctx, q, entityID) (LegalEntityTaxID, error)` which dispatches
   on the entity's fundamental type.

## Location
- `core-module/api/service/natural_person.go` — extend inputs + Create /
  Update.
- `core-module/api/service/corporation.go`    — same for corporations.
- `core-module/api/service/legal_entity.go`   — add `GetTaxID`.
- `core-module/api/service/profile.go`        — expose optional tax id
  on `Profile` (decrypted, only if caller has access — the service
  itself returns plaintext; the http layer decides whether to include
  it in the response).
- `core-module/api/service/service.go` (or wherever the shared service
  dependency struct lives) — add `FieldCipher *fieldcrypto.Cipher` and
  pass it into the NP / Corp services.

Read the existing service.go / service constructor first — the exact
name of the "deps" struct varies. Integrate there, don't invent a new
wiring path.

## New/changed types

```go
// LegalEntityTaxID is a uniform view of a legal entity's tax id.
// Type is "SSN" for natural persons, "EIN" for corporations, or "" if
// the entity is not one of those leaf kinds.
// Value is plaintext. Empty string means "not recorded".
type LegalEntityTaxID struct {
    Type  string
    Value string
}
```

Add these fields to the existing Create/Update input structs:

```go
// CreateNaturalPersonInput
SSN string // optional plaintext; "" means not recorded

// UpdateNaturalPersonInput
SSN *string // nil = leave unchanged; pointer-to-"" = clear; non-empty = set

// CreateCorporationInput / UpdateCorporationInput analogous with EIN
```

Justification for `*string` on update:
- `nil`              → omit from UPDATE (COALESCE keeps existing)
- `&""`              → clear (encrypt empty → empty bytea; stored)
- `&"12-3456789"`    → set (encrypt and store)

## Encryption glue

Inside `NaturalPersonService.Create`:

```go
var ssnBlob []byte
if strings.TrimSpace(in.SSN) != "" {
    blob, err := s.cipher.Encrypt(strings.TrimSpace(in.SSN))
    if err != nil { return ..., fmt.Errorf("encrypt ssn: %w", err) }
    ssnBlob = blob
}
// pass ssnBlob as CreateNaturalPersonParams.Ssn
```

Inside `Update`:

```go
var ssnParam []byte  // nil = leave unchanged
if in.SSN != nil {
    val := strings.TrimSpace(*in.SSN)
    if val == "" {
        ssnParam = []byte{}                           // clear
    } else {
        b, err := s.cipher.Encrypt(val)
        if err != nil { return fmt.Errorf("encrypt ssn: %w", err) }
        ssnParam = b
    }
}
```

Pass `ssnParam` through `UpdateNaturalPersonParams.Ssn`.

Mirror the above for corporations / EIN.

## Reads

Add a method on NaturalPersonService (and CorporationService):

```go
// GetDecryptedTaxID returns the plaintext SSN (or EIN) for an entity.
// Returns "" if not set. Returns an error only on decrypt failure
// (i.e. stored blob is corrupt or key is wrong) — not for NULL.
func (s *NaturalPersonService) GetDecryptedSSN(ctx context.Context, q coredb.Querier, entityID int64) (string, error)
```

Mirror for Corporation.

## Audit
When writing an audit entry on a create/update that touched the tax id,
log a **redacted** marker, never plaintext or ciphertext:

```go
auditFields["ssn"] = "set"    // or "cleared" or "unchanged"
```

## LegalEntity abstract accessor

In `legal_entity.go`:

```go
func (s *LegalEntityService) GetTaxID(
    ctx context.Context,
    q coredb.Querier,
    entityID int64,
    np  NaturalPersonServicer,  // for SSN
    corp CorporationServicer,    // for EIN
) (LegalEntityTaxID, error)
```

Dispatch on `entities.fundamental_type_id` via the existing type
registry (use `type_is_or_descends_from` semantics already available
through the querier — check `queries/types.sql` for the right helper;
if none, read the entity row and use its recorded type slug).

If the entity does not descend from `natural_person` or `corporation`,
return a zero `LegalEntityTaxID` (both fields empty) and nil error — it
is not an error for a non-leaf-typed entity to have no tax id.

Alternative if passing servicers feels wrong: pull the cipher directly
into LegalEntityService and issue the table-specific query here. Pick
whichever keeps the service wiring cleaner; document the choice at the
top of `legal_entity.go`.

## Profile

If `Profile` (see `service/profile.go`) surfaces NP or Corp details,
extend it with:

```go
type Profile struct {
    // ... existing fields
    TaxID     string // plaintext, optional; empty if none or hidden
    TaxIDType string // "SSN" | "EIN" | ""
}
```

Populate these in `ResolveProfileByEntityID` — but only if the caller
has permission. Since the service layer generally does not know the
caller, the cleanest pattern is:

- Always decrypt and populate `TaxID` / `TaxIDType` in the Profile
  returned by the service.
- The http layer strips them before serializing when the caller is not
  authorized.

This keeps service logic uniform; auth gating stays at the http edge.

## Acceptance
- NP Create/Update and Corp Create/Update accept the new field.
- Plaintext never ends up in audit rows (grep for `ssn|ein` in audit
  write callsites to confirm).
- `GetDecryptedSSN` / `GetDecryptedEIN` return `""` for NULL blobs
  without error.
- `LegalEntityService.GetTaxID` returns the right `Type`/`Value` for a
  natural-person entity, a corporation entity, and a non-leaf entity.
- All existing service tests still pass.
- New unit tests cover: encrypt-on-create, decrypt-on-read, clear via
  pointer-to-empty-string, leave-unchanged via nil pointer, audit
  redaction.

## How to verify
```sh
cd core-module/api
go build ./...
go test ./service/...
go vet  ./service/...
```

## Notes
- Do NOT add plaintext ssn/ein to any existing struct that is
  serialized directly to JSON today without http-layer review (task.5
  will audit that).
- Do NOT introduce a global singleton cipher. Inject via the existing
  deps struct.
- If you find yourself needing to change sqlc-generated code, stop —
  update the query in task.3 and regenerate instead.
