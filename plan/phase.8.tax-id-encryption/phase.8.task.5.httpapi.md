# Phase 8, Task 5 — HTTP API and OpenAPI fragment

## Context
Blocked by task.4 (service layer).

Expose the new tax-id fields at the HTTP boundary and update the OpenAPI
fragment so consuming services see them.

## Location
- `core-module/api/httpapi/natural_persons.go`
- `core-module/api/httpapi/corporations.go`
- `core-module/api/httpapi/response.go` (Profile serialization — check
  whether `profileResponse` lives here; follow what already exists)
- `core-module/api/openapi.fragment.yaml`

## Request changes

### createNaturalPersonRequest
```go
type createNaturalPersonRequest struct {
    GivenName  string `json:"given_name"`
    FamilyName string `json:"family_name"`
    SSN        string `json:"ssn,omitempty"`   // optional plaintext
}
```

### updateNaturalPersonRequest
```go
type updateNaturalPersonRequest struct {
    GivenName  *string `json:"given_name"`
    FamilyName *string `json:"family_name"`
    SSN        *string `json:"ssn"` // nil = unchanged, "" = clear, else set
}
```

Mirror for Corporation with `EIN` / `ein`.

Pass these through to the service input structs 1:1.

## Response gating

Current `profileResponse(profile)` helper is the natural place to enforce
the admin-or-self rule. Change it to take the caller principal as a
parameter (or add a sibling helper `profileResponseFor(principal, profile)`).
Whichever the existing codebase prefers — do not fork styles.

Rule:

```
include tax_id / tax_id_type in the response body IFF
    principal.IsAdmin
 OR principal.EntityID == profile.Entity.ID
```

When not included, OMIT the keys entirely (do not emit `null`). The
OpenAPI schema documents these as optional.

Because the service layer always populates `Profile.TaxID` /
`Profile.TaxIDType` (see task.4), stripping is a simple zero-out before
JSON encode when the caller fails the gate.

## OpenAPI fragment

Extend the schemas in `openapi.fragment.yaml`:

### CreateNaturalPersonRequest (add one property)
```yaml
ssn:
  type: string
  description: |
    Optional plaintext Social Security Number. Stored encrypted at rest.
    Returned on read only to admins or to the subject themselves.
  nullable: false
```

### UpdateNaturalPersonRequest
Add `ssn` with the same shape but describe the three-state behavior:
omit = unchanged, empty string = clear, non-empty = set. The schema
cannot express "omitted vs null" distinction fully; document it in the
`description:` block.

### CreateCorporationRequest / UpdateCorporationRequest
Analogous with `ein`.

### Profile
Add two optional fields:

```yaml
tax_id:
  type: string
  description: |
    Plaintext tax id (SSN for natural persons, EIN for corporations).
    Present only when the caller is an admin or the subject of the
    profile. Absent otherwise.
tax_id_type:
  type: string
  enum: [SSN, EIN]
  description: |
    Kind of tax id held. Present only when tax_id is present.
```

Neither field is in the `required:` list.

## Error handling
- Encryption failure during create/update → 500, log with
  `slog.ErrorContext` but do not echo the plaintext in the log. Message:
  `"encrypt failure"` + request id is enough.
- Cipher not configured (env var missing at boot) → the server should
  fail to start. This is a service-construction concern (task.4). The
  http layer does not need a runtime check for nil cipher.

## Acceptance
- Create NP with `{"given_name":"A","family_name":"B","ssn":"123-45-6789"}`
  as admin → 201; response includes `tax_id` and `tax_id_type:"SSN"`.
- Same entity read back as a stranger (non-admin, not the subject) →
  response omits `tax_id` / `tax_id_type`.
- Update with `{"ssn":""}` → subsequent admin read returns empty-string
  `tax_id` (or omits it — clarify in task.6 tests which reflects reality).
- openapi.fragment.yaml validates (if a validator is wired into the
  Makefile, use it; otherwise at least run `yq .` to confirm well-formed
  YAML).

## How to verify
```sh
cd core-module/api
go build ./...
go test ./httpapi/...
```

## Notes
- Do NOT echo the stored ciphertext in any error / log line.
- Do NOT add a dedicated `/entities/natural-persons/{uuid}/tax-id`
  route. Keep it on the existing profile payload; the admin-or-self
  gate is simpler there.
- If the existing `profileResponse` has no access to the principal,
  wire it through at the handler callsite rather than reaching into
  the request context from inside the helper.
