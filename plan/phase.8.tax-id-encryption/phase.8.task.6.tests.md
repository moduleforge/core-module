# Phase 8, Task 6 — Tests and final verify

## Context
Blocked by task.5. This task's job is end-to-end validation — not a
rewrite. Unit tests for fieldcrypto and service-layer tests are written
in the preceding tasks. Here we add the tests that cut across layers.

## Test additions

### httpapi round-trip (integration-style, using the existing fake/mock harness)

Follow the style of the existing tests under
`core-module/api/httpapi/*_test.go` — they already use `fake_tx_test.go`
/ `mock_test.go` harnesses. Do not invent new test infrastructure.

Cases:

1. **Admin creates NP with SSN, reads back as admin** — response
   contains `tax_id == "123-45-6789"` and `tax_id_type == "SSN"`.
2. **Admin creates NP with SSN, reads back as different non-admin user**
   — response has no `tax_id` / `tax_id_type` keys at all (not null,
   absent).
3. **Self read** — the subject of the NP, authenticating as themselves
   (non-admin), reads their own profile and sees the plaintext.
4. **Clear via empty string update** — admin `PUT` with `{"ssn":""}`,
   then admin `GET` returns no tax id (or empty-string; whichever is
   the implemented contract — match task.5).
5. **Leave unchanged via omit** — admin `PUT` with only given_name,
   SSN absent from JSON → value is unchanged.
6. **Corporation EIN** — one smoke test mirroring #1 for
   `/entities/corporations`.

### DB-level opacity test (if a live test db is available)

In whichever test file already drives real Postgres (look for a
`_integration_test.go` or `//go:build integration` tagged file), add:

```go
// After inserting an NP with SSN="123-45-6789", query the raw column
// and assert it does NOT contain the literal plaintext bytes.
row := db.QueryRow(ctx, "SELECT ssn FROM natural_persons WHERE entity_id = $1", entityID)
var blob []byte
require.NoError(t, row.Scan(&blob))
require.NotEmpty(t, blob)
require.False(t, bytes.Contains(blob, []byte("123-45-6789")))
```

If there is no integration test setup, skip this case and note it in
the final report.

### Audit redaction test

Verify the audit writer receives `"ssn": "set"` / `"cleared"` — never
the plaintext and never the ciphertext bytes — on create and update.
The existing audit-writer test pattern should make this
straightforward.

## Full verification

Run, from repo root:

```sh
cd core-module/model && make build              # sqlc clean
cd ../api            && go build ./...
cd ../api            && go test -race ./...
cd ../model          && atlas migrate validate
```

All must succeed.

## Report

Create `core-module/plan/report.phase8.md` summarizing:

- What shipped.
- Any deviations from the phase plan and why.
- Anything flagged for future work (key rotation, AAD binding, etc.).
- Open questions for the user, if any.

## Acceptance
- All new and existing tests pass (including `-race`).
- `report.phase8.md` written.
- No plaintext tax id appears in any log line, audit row, or error
  message in the test suite.

## Notes
- Do NOT commit a `CORE_FIELD_KEY_HEX` value into the repo. Tests use
  `fieldcrypto.NewFromKey(make([]byte, 32))` or a locally generated key.
- Do NOT widen integration tests to require a real database if they do
  not today; keep the suite runnable with `go test ./...` in CI.
