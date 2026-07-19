# Plan Summary: API Response Action-Required Writer And Conflict Constructor

## What was planned and why

Extend `mod-core`'s shared `api/apiresp` package (`github.com/moduleforge/core-api`) to implement
the **action-required response contract** finalized in
`docs/mf-standards/architecture/api-response-design.md`, and to add the missing public
`Conflict(details ...FieldError) error` constructor.

This is Wave 0 of a 3-wave, 3-repo cross-repo effort (moduleforge/mod-users followup `eiF8`, tag
`users-apiresp-migration`). Wave -1 (`docs-mf-standards`, completed) designed the action-required
contract and the `Conflict()` constructor's shape. This plan (Wave 0, `mod-core`) implements the
Go layer both were designed against. The downstream mod-users plan (Wave 1: migrate five deferred
backend handler sites onto this package plus the GUI-side handling for action-required responses)
has **not yet been authored** — it is the natural next step once this plan's output is available.

The plan comprised a single phase with two parallel-eligible tasks, both confined to
`api/apiresp` and touching disjoint files:

1. Add the action-required machinery (`ActionCode`, `ActionBody`/`ActionEnvelope` wire types,
   `WriteActionRequired`) as a third response kind alongside `WriteJSON` and `WriteError` — a
   peer, not a variant of `WriteError`.
2. Add the public `Conflict()` constructor mirroring the existing `InvalidInput()`, folding in
   mod-users followup `ZVum` (tag `go-apiresp-foundation`).

Out of scope: any `mod-core/gui` changes, any other repo, and edits to the `mf-standards`
submodule doc itself (the already-finalized spec this plan implements against).

## What shipped

**Task 001 — Add Action-Required Writer** (merge `d4d06b426eca822f3be1626891b697eda8c74c53`).
Implemented the action-required response kind as the third apiresp writer in a single new file
`api/apiresp/action.go`: `ActionBody`/`ActionEnvelope` wire types matching the design doc's JSON
shape exactly; `ActionCode{Code, Status}` documented as carrying no package-level
registry/validation (ownership stays per-module); and `WriteActionRequired(w, r, action, message,
path, data)`, which writes `action.Status` verbatim (503 included, never remapped) via the
existing `WriteJSON`, sets `Action.Data` only when `data != nil`, and panics via an unexported
`isAppRelativePath` helper when `path` is not application-relative — implementing the structural
open-redirect guard decided in the companion decisions note. Doc comments state both caller
obligations (path relative — enforced; data minimal/non-sensitive — caller-discipline only).
Table-driven tests in `api/apiresp/action_test.go` cover the three registered mod-users flows
end-to-end at their bound statuses, the `data` omitempty behavior, and the path guard's
panic/no-panic cases. `make build`, `make lint`, and `make test` all passed; only the two new
files were touched.

**Task 002 — Add Conflict Constructor** (merge `7303ab533011309db7a1321b87da47954f6f43d4`). Added
`api/apiresp/conflict.go` with a `conflictError` type and `Conflict(details ...FieldError) error`
constructor mirroring `invalidinput.go`'s `invalidInputError`/`InvalidInput` one-for-one, bound to
`ErrConflict` instead of `ErrInvalidInput`. Extracted an unexported `fieldDetailCarrier` interface
(`fieldDetails() []FieldError`) in `invalidinput.go`, implemented by both `invalidInputError` and
`conflictError`, and rewrote the shared `fieldErrors` accessor to match via
`errors.As(err, &carrier)` instead of a two-type fallback — future detail-carrying error types
need no further edits to `fieldErrors`. Added `api/apiresp/conflict_test.go` with
`TestConflict_WithDetails` and `TestConflict_NoDetails` mirroring the existing
`TestInvalidInput_*` tests. `go build`, `go vet`, `gofmt -l`, and `go test ./...` all passed
clean; pre-existing `TestInvalidInput_*` tests still passed unchanged, confirming the
`fieldErrors` refactor was behavior-preserving.

Both tasks validated against `cd api && make test` and `cd api && make lint` per `AGENTS.md`.

## Key decisions

Recorded in full in `plan/notes/action-required-writer-decisions.md`, written explicitly so the
downstream mod-users Wave-1 plan can treat this plan's output as the literal spec without
re-deriving them:

- **`action.path` gets a panic-based structural guard, not silent neutralization or pure
  caller-discipline.** The design doc treats the server-side `action.path` constraint (no scheme,
  no `//`-authority) as a real open-redirect boundary and presumes it exists for the GUI's
  client-side rejection to backstop. `WriteActionRequired` performs a minimal,
  application-agnostic structural check and **panics** on violation, because the writer's
  signature returns nothing and `action.path` is a required, non-empty field — there is no safe
  substitute value (an empty path violates the contract; an invented route like `/` silently
  misnavigates the user). Panic surfaces the bug immediately in the caller's own tests/CI, is
  unreachable for the three registered, statically-literal mod-users call sites, and — recovered
  by standard HTTP middleware — degrades to a safe 500 rather than ever emitting an open-redirect
  to the client. The guard is additive within the documented signature, not a divergence from it.
- **`action.data` sensitivity stays a caller-discipline-only contract obligation, with no runtime
  guard.** Whether an arbitrary `any` payload contains a token or internal identifier is a
  semantic property the writer cannot mechanically evaluate; a runtime check would be security
  theater. The obligation is instead stated explicitly in the writer's doc comment.
- **`ActionCode` ownership is module-namespaced, not package-validated.** `apiresp` deliberately
  keeps no package-level registry or validation of action codes — vocabulary ownership stays with
  the consuming module (e.g. mod-users), matching the design doc's `## Action-code vocabulary`
  section.
- **A `fieldDetailCarrier` interface extraction lets `InvalidInput` and `Conflict` share one
  detail-recovery accessor.** Rather than hand-rolling a second type check into `fieldErrors`,
  task 002 generalized it to `errors.As` against the new interface — any future detail-carrying
  error type slots in without touching `fieldErrors` again.

## Review-fix round

Phase 01's review gate did not pass on the first pass; one consolidated fix round (commit
`ea529f6`) was required before it did. Two independent review lenses — correctness and security —
converged on the same real, high-confidence bug in `isAppRelativePath`: the guard only checked
`net/url.Parse`'s `Scheme`/`Host` fields, missing two browser-parsing-divergence bypass classes
that WHATWG URL parsing (`window.location`, `<a href>`, JS routers) resolves off-origin but Go's
RFC-3986 parser does not flag — backslash-based host confusion (`/\evil.example`,
`\\evil.example`) and multi-slash authority collapsing (`///evil.example`, `////evil.example`).
This is the same class of divergence Django's own open-redirect protection explicitly works
around. The correctness lens independently found a second real bug, unrelated to the guard: a
typed-nil `data` argument (e.g. a nil `*T`) is non-nil as an interface in Go, so the writer's
`data != nil` check was letting it through and serializing `"data":null` instead of omitting the
field — a classic Go nil-interface gotcha.

Both were fixed in one consolidated pass, confined entirely to `api/apiresp/action.go` and its
tests: the path guard was hardened against both bypass classes via direct string checks ahead of
`net/url.Parse`, with new regression tests for each rejection case; the typed-nil case was fixed
via reflection-based detection; and the writer's doc comment was strengthened to state the
panic-recovery caller obligation explicitly (mod-core itself has no recovery middleware) and to
note that the guard-violation panic is not routed through the package's structured
`logServerError` path. Re-verification after the fix — `make build`, `make lint`, `make test` —
was clean.

## Follow-up items

Recorded in `plan/followups.yaml`; two were added during this session's phase-review round:

- **`b2nA` — WriteActionRequired hot-path unconfirmed.** `isAppRelativePath`'s `net/url.Parse`
  call allocates more than strictly necessary for a scheme/host-only check. Not a regression (all
  new code) and no caller of `WriteActionRequired` exists anywhere in mod-core yet, so real
  per-request call frequency is undetermined. Worth a quick look once a consuming module (e.g.
  mod-users Wave-1) wires in real call sites — if genuinely hot, consider a lighter string-based
  check instead of full `url.Parse`.
- **`Yghm` — docs-mf-standards: mark writer as built.** `api-response-design.md` still says "the
  full Go implementation is out of scope for this documentation repo" for `WriteActionRequired`'s
  Go-layer sketch, mirroring the pre-existing pattern for `WriteError`/`WriteJSON` before they
  were built. Now that `WriteActionRequired` is implemented and merged in mod-core, that line
  should be updated to reflect it's built, parallel to how the doc already reads for
  `WriteError`/`WriteJSON`. Cross-repo (`docs-mf-standards`), not fixable from mod-core.

The downstream mod-users Wave-1 plan (migrating the five deferred backend handler sites plus GUI
handling onto this package) is the natural next step and has not yet been started. That plan's
authoring **must be told the `action.path` guard's exact accept/reject rules** before it authors
call sites that build `path` from anything less static than literal strings: reject any path
carrying a URL scheme, a `//`-prefixed authority, a backslash-based host-confusion pattern
(`/\...`, `\\...`), or a multi-slash-collapsed authority (`///...`, `////...`), in addition to the
baseline empty-path rejection. All three registered mod-users action codes currently pass static,
relative string literals (`/verify-email`, `/step-up?return=%2Fself%2Fidentities`,
`/setup/oidc`), so today's correct callers never trip the guard — but Wave-1 call sites that build
`path` dynamically must be validated against these rules up front.
