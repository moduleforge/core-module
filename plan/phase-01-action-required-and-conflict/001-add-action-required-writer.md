# Add Action-Required Writer

## Purpose and scope

Implement the **action-required response kind** in `mod-core`'s shared `api/apiresp` package — the
third response writer alongside `WriteJSON` (success) and `WriteError` (error), per
`docs/mf-standards/architecture/api-response-design.md`, sections `## Action-required responses` and
`## Go-layer ownership` (item 4). Add the `action` envelope wire types, the `ActionCode` value type,
and the `WriteActionRequired` writer, with matching unit test coverage.

All new code lives in a single new file `api/apiresp/action.go` (mirroring how
`api/apiresp/invalidinput.go` keeps one feature's type + function together), with tests in a new
`api/apiresp/action_test.go`. This task touches **no existing package file**, so it is
parallel-eligible with task 002.

This is standard Go implementation work — no dedicated skill. Follow the Go design standards and
match the existing package conventions (naming, doc-comment style, `omitempty` field semantics)
already established in `api/apiresp/types.go`, `errors.go`, and `writer.go`.

## Requirements

### 1. Wire types (match existing `Envelope`/`ErrorBody` conventions in `types.go`)

- **`ActionBody`** — the nested `action` object. Fields, with these exact JSON tags:
  - `Code string` → `json:"code"` (required)
  - `Message string` → `json:"message"` (required)
  - `Path string` → `json:"path"` (required)
  - `Data any` → `json:"data,omitempty"` (optional; omitted — not `null`, not `{}` — when absent,
    exactly mirroring how `ErrorBody.Details` uses `omitempty`)
- **`ActionEnvelope`** — the top-level body: a single `Action ActionBody` member → `json:"action"`.
  This is the flow-control parallel to `Envelope{Error: ErrorBody}`.

The emitted JSON MUST match the design doc's shape exactly: a top-level `action` object with
`action.code`, `action.message`, `action.path` always present and `action.data` present only when
supplied. Cross-check against the three worked examples in the doc's `### The action object`.

### 2. `ActionCode` value type

```go
type ActionCode struct {
    Code   string // namespaced, e.g. "users.email_unverified"
    Status int    // the code's bound status: 403, 409, or 503
}
```

This is the doc's item-4 sketch verbatim. It pairs a registered, module-namespaced action code with
its single bound HTTP status so that unregistered codes and off-status writes are unrepresentable at
the call site. Do **not** add a package-level registry or validation of `Status` against the {403,
409, 503} set — the closed set is owned per-module in each module's registry (per
`### Action-code vocabulary`), not by this shared package; this package's job is only to carry and
emit the code+status the caller supplies. Document this ownership boundary in the type's doc comment.

### 3. `WriteActionRequired` writer

Implement the doc's item-4 sketch precisely, with this exact signature:

```go
func WriteActionRequired(w http.ResponseWriter, r *http.Request, action ActionCode, message, path string, data any)
```

Behavior:
- Build `ActionEnvelope{Action: ActionBody{Code: action.Code, Message: message, Path: path}}`.
- Set `Action.Data` only when `data != nil` (so a `nil` data omits the member via `omitempty`).
- Write the body and **`action.Status` verbatim** via the existing `WriteJSON` — including **503**
  for `oidc_not_confirmed`. It MUST NOT remap the status onto an error status (403/409/500) under
  any circumstance. This is the load-bearing behavior the doc's `### Action-required status set`
  emphasizes ("503 is a first-class, reserved action-required status ... MUST NOT be remapped").
- This writer is a **peer** of `WriteError`, **not** a variant of it: its normal path MUST NOT call
  `WriteError` and MUST NOT ever emit the error envelope. It always emits the `action` envelope.
- The `r *http.Request` parameter is part of the documented signature; keep it even though the
  normal path does not need it (parity with `WriteError`; available for future logging).

### 4. `action.path` structural guard (decided — see references)

`WriteActionRequired` MUST validate that `path` is application-relative before writing, and **panic**
on a violation. Reject a path that has a URL scheme (e.g. `http://…`, `javascript:…`) or is
`//`-authority-prefixed (protocol-relative, e.g. `//evil.example/x`). Accept ordinary relative paths
including query strings (e.g. `/verify-email`, `/step-up?return=%2Fself%2Fidentities`, `/setup/oidc`).

- A robust, application-agnostic implementation is to parse with `net/url.Parse` and reject when the
  parse errors, or when `u.Scheme != ""`, or when `u.Host != ""`. (An empty `path` is also not a
  valid application-relative navigation target; reject it too.) The exact mechanism is at the
  implementer's discretion as long as the accept/reject cases above hold.
- Put the check in a small unexported helper (e.g. `func isAppRelativePath(p string) bool`) and
  panic with a clear message identifying it as an open-redirect / programmer-error guard.
- **Do not** guard `action.data` at runtime — its "minimal, non-sensitive" constraint is a semantic
  property the writer cannot mechanically evaluate over an arbitrary `any`. State it as a caller
  obligation in the doc comment instead.

The rationale for both the path-guard-via-panic and the data-caller-discipline decisions is recorded
in [action-required writer decisions](../notes/action-required-writer-decisions.md) — read it before
implementing; it is the explicit judgment call this plan makes, and the doc comment on
`WriteActionRequired` should state both caller obligations (`action.path` relative;
`action.data` minimal/non-sensitive) at the call boundary.

### 5. Doc comments

Every exported symbol (`ActionBody`, `ActionEnvelope`, `ActionCode`, `WriteActionRequired`) carries
a doc comment per Go convention, matching the tone and detail of the existing comments in
`types.go`/`writer.go`. The `WriteActionRequired` comment must state the verbatim-status (incl. 503,
never remapped) behavior and the two `action.path` / `action.data` caller obligations.

## Validation

- **Files:** `api/apiresp/action.go` and `api/apiresp/action_test.go` created; no other existing
  `api/apiresp/*.go` file modified (`git status` shows only these two additions for this task).
- **Build + lint:** `cd api && make lint` passes (go vet + gofmt clean).
- **Tests:** `cd api && make test` passes. New table-driven tests in `action_test.go`
  (`package apiresp_test`, matching the existing external-test-package convention in
  `apiresp_test.go`) must cover:
  - Each of the three registered flows end-to-end via `httptest.NewRecorder`: `users.email_unverified`
    → **403**, `users.step_up_required` → **409**, `users.oidc_not_confirmed` → **503**. Assert the
    recorder status equals `ActionCode.Status` exactly (in particular assert **503 is emitted, not
    remapped**), `Content-Type: application/json`, a top-level `action` object present with matching
    `code`/`message`/`path`, and **no** top-level `error` member.
  - `action.data` present and correctly nested (e.g. `{"state":"awaiting_oidc_config"}`) when
    supplied, and the `data` key **absent** from the raw JSON (not `null`, not `{}`) when `data` is
    `nil` — assert on the raw body text, as `TestInvalidInput_NoDetails` does for `details`.
  - The `action.path` guard: `WriteActionRequired` **panics** for at least `http://evil.example/x`,
    `//evil.example/x`, and `javascript:alert(1)`, and does **not** panic for `/verify-email` and
    `/step-up?return=%2Fself%2Fidentities`. Use a `recover()`-based table assertion.
- **Doc conformance:** the emitted envelope matches the JSON in the design doc's
  `### The action object` examples (spot-check field names and nesting).

## Metadata

architectural_impact: true

## References

- `docs/mf-standards/architecture/api-response-design.md` — the finalized spec. Key sections:
  `## Action-required responses` (envelope + field semantics table), `## Go-layer ownership` item 4
  (the near-literal `ActionCode` / `WriteActionRequired` sketch), `### Action-required status set`
  (503 is reserved, never remapped), `### Action-code vocabulary` (the three registered `mod-users`
  codes and their bound statuses).
- `api/apiresp/types.go` — `Envelope`/`ErrorBody`/`FieldError`; match its type + JSON-tag +
  `omitempty` conventions and doc-comment style.
- `api/apiresp/writer.go` — `WriteJSON`/`WriteError`; `WriteActionRequired` is their peer and reuses
  `WriteJSON` to emit the body.
- `api/apiresp/apiresp_test.go` — the existing test conventions (external `apiresp_test` package,
  `httptest` recorders, raw-body assertions for `omitempty`) to mirror in `action_test.go`.
- `plan/notes/action-required-writer-decisions.md` — the recorded judgment call on the path guard
  and data caller-discipline.
- `AGENTS.md` — `cd api && make test` / `cd api && make lint`.

## Checkpoint hints

- After adding the wire types + `ActionCode` and confirming `cd api && make build` compiles.
- After adding `WriteActionRequired` + the path guard.
- After adding `action_test.go` and confirming `cd api && make test` passes.
