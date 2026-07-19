# Action-required writer: design decisions

## Purpose and scope

Records the deliberate implementation decisions for the action-required writer, so the downstream
mod-users Wave-1 plan can treat this plan's output as the literal spec without re-deriving them.
The design source of truth is `docs/mf-standards/architecture/api-response-design.md`, sections
`## Action-required responses`, `## Go-layer ownership` (item 4), and `## Action-code vocabulary`.

## Decision 1 — `action.path` gets a structural guard in the writer

**The design doc's normative rule** (`### The action object`, `action.path` row): the path MUST be
application-relative — no scheme, no `//`-prefixed authority — and a handler MUST NOT construct it
from unvalidated external input, "which would open a redirect vector." The GUI-facing section frames
the client-side rejection of non-same-origin paths as "defense in depth alongside the **server-side
`action.path` constraint**," i.e. it presumes a server-side constraint exists to be backstopped.

**Decision: `WriteActionRequired` performs a minimal, application-agnostic structural check on
`path` and panics on violation.** A violating path (has a URL scheme, or is `//`-authority-prefixed)
is treated as a programmer error and fails fast.

**Rationale:**

- The check is purely structural (scheme / authority presence) and needs no route table, so it is
  cheap and application-independent — exactly the kind of boundary validation the Go design
  standards' "Validate input at boundaries" / "Security" guidance calls for on a shared library's
  public writer.
- It guards a real open-redirect security boundary that the design doc itself calls out.
- It is **transparent to the downstream contract**: the guard does not change the documented
  signature and never alters behavior for a valid (relative) path. All three registered mod-users
  action codes pass static, relative string literals (`/verify-email`,
  `/step-up?return=%2Fself%2Fidentities`, `/setup/oidc`), so correct callers never trip it.
- Panic (rather than silent neutralization) is chosen because the writer's signature returns
  nothing and `action.path` is a required, non-empty field: there is no safe substitute value to
  emit (an empty path violates the contract; an invented route like `/` silently misnavigates the
  user). Panic invents nothing, surfaces the bug immediately in the caller's own tests/CI, is
  unreachable for the static registered call sites, and — recovered by standard HTTP middleware —
  degrades to a safe 500 rather than ever emitting an open-redirect to the client. This mirrors the
  common Go idiom of panicking on structurally-invalid programmer input (e.g. `regexp.MustCompile`).

**Alternative considered and rejected:** pure caller-discipline with no writer guard, matching the
doc's guard-free sketch byte-for-byte. Defensible (the sketch writes `path` verbatim, and the doc
assigns the construction obligation to the handler + client). Rejected because a two-line structural
check on a shared response writer is worthwhile defense-in-depth for an open-redirect boundary, and
adds zero risk to correct callers. The guard is an addition *within* the documented signature, not a
change to it, so it does not diverge from the literal downstream spec.

## Decision 2 — `action.data` sensitivity stays a caller-discipline contract obligation

**The design doc's normative rule** (`action.data` row): data MUST contain only minimal,
non-sensitive state and MUST NOT carry internal identifiers, tokens, or server-internal state.

**Decision: no runtime guard; document the obligation in the writer's doc comment.** Whether an
arbitrary `any` payload contains a token or internal identifier is a semantic property the writer
cannot mechanically evaluate — a runtime check would be security theater. This is inherently a
caller-discipline obligation, stated explicitly at the call boundary via the doc comment.

## Follow-up note (out of scope here)

After this plan lands, `api-response-design.md`'s `## Go-layer ownership` section could be updated to
state that `WriteActionRequired` is now built and merged (as it already does for `WriteError` /
`WriteJSON`), replacing "the full Go implementation is out of scope for this documentation repo."
That doc lives in the `mf-standards` submodule (a separate repo) and is out of scope for this plan;
it is a candidate follow-up for the docs repo, not tracked as a phase here.
