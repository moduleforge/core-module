# Phase 8 — Tax ID (SSN / EIN) with at-rest encryption

## Goal
Add government-issued tax identifiers to the legal-entity hierarchy:

- `natural_persons.ssn` — encrypted BYTEA column.
- `corporations.ein` — encrypted BYTEA column.
- Domain-level abstract pair on LegalEntity: `tax_id` and `tax_id_type`
  (`tax_id_type` = `"SSN"` for natural persons, `"EIN"` for corporations).
  The abstraction lives at the Go/API layer; there is no new physical column
  on the `legal_entities` table.

All at-rest storage is encrypted using AES-256-GCM at the application layer.
The schema remains vanilla SQL (no `pgcrypto`, no Postgres-specific features).

## Design

### Storage
- `natural_persons.ssn  BYTEA NULL`
- `corporations.ein     BYTEA NULL`
- Columns hold the output of `crypto.Encrypt(plaintext)`; format is
  `nonce(12) || ciphertext || gcm-tag(16)` as a single opaque blob.
- `NULL` = no tax id recorded. There is no empty-string convention.

### Abstract LegalEntity view
Callers that only care about "this entity's tax id, whatever kind" use the
LegalEntity domain accessor:

```go
type LegalEntityTaxID struct {
    Type  string // "SSN" | "EIN"
    Value string // plaintext; empty string if not set
}
```

The accessor dispatches on `entities.fundamental_type_id`:
- `natural_person` → reads `natural_persons.ssn`, returns `Type="SSN"`.
- `corporation`    → reads `corporations.ein`, returns `Type="EIN"`.

No SQL view is required; the dispatch is in Go.

### Encryption
- Package: `core-module/api/internal/fieldcrypto` (Go).
- Algorithm: AES-256-GCM, random 12-byte nonce per record, tag appended.
- Key source: env var `CORE_FIELD_KEY_HEX` (64 hex chars = 32 bytes).
  If unset, encryption calls return an error — no silent fallback.
- Package exposes:
  - `type Cipher struct { ... }`
  - `NewFromEnv() (*Cipher, error)`
  - `(*Cipher).Encrypt(plaintext string) ([]byte, error)`
  - `(*Cipher).Decrypt(blob []byte) (string, error)`
- Cipher is injected into services via the existing service-dependency
  struct (same pattern as `audit.Writer`).

### API surface
- Create / update endpoints accept `ssn` (NP) or `ein` (Corp) as plaintext
  JSON string. Empty string or omitted → unchanged / unset.
- Read endpoints include the plaintext `ssn`/`ein` in the response **only
  when** the caller is admin or is the subject themselves. Otherwise the
  field is omitted.
- The Profile response gains an optional unified `tax_id` + `tax_id_type`
  pair at the LegalEntity level for generic consumers.
- Audit writes log only `{"ssn": "set"}` / `{"ssn": "cleared"}` (never the
  plaintext or ciphertext).

## Tasks

1. **task.1.fieldcrypto** — Implement the encryption package + unit tests.
2. **task.2.migration**   — Add SQL migrations for `ssn`/`ein` columns.
3. **task.3.queries**     — Update sqlc queries + regenerate db package.
4. **task.4.service**     — Wire encryption into NP/Corp services; add
   LegalEntity `GetTaxID` accessor.
5. **task.5.httpapi**     — Expose ssn/ein in request/response JSON,
   enforce admin-or-self read gate, update the OpenAPI fragment.
6. **task.6.tests**       — Round-trip integration tests and final verify.

## Dependencies

```
task.1 ──┐
         ├─→ task.4 ─→ task.5 ─→ task.6
task.2 ─→ task.3 ───┘
```

Tasks 1 and 2 run in parallel. Task 3 waits on 2. Task 4 waits on 1 and 3.

## Hard rules
- No `pgcrypto`. No Postgres-specific extensions.
- No plaintext tax id in logs, audit rows, or error messages.
- Empty key env var = hard failure at service construction, not at first
  request.
- Integer IDs stay internal; UUIDs in responses (existing rule).
- BYTEA columns are `NULL` for "unset", never empty bytes.
- Do not modify `users-module/model/*` — Phase 3 already decoupled it.

## How to verify
- `cd core-module/model && make build` → sqlc clean.
- `cd core-module/api   && go test ./...` → all green.
- `atlas migrate validate` + `atlas migrate hash` succeed.
- New integration test creates a NP with an SSN, reads it back as admin
  (plaintext visible), reads it back as a stranger (field absent), and
  confirms the stored BYTEA in the db is not the plaintext UTF-8.

## Notes
- This phase is additive; no existing data is touched. All columns are
  nullable.
- Key rotation is out of scope for Phase 8. A follow-up would introduce a
  key-id prefix in the blob and a rotation migration.
- The two-valued `tax_id_type` enum is deliberately kept as a TEXT label
  in Go, not a SQL enum, so that future `passport`, `itin`, etc. can be
  added without a migration.
