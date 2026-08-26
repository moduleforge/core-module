# Give banner-level errors the same shared-surface treatment toast-level errors already have

**Status:** designed, not implemented. No code has been changed for this issue anywhere —
this document is the handoff for whoever picks up the fix.

**Discovered:** 2026-07-30, while planning a fix for two `mod-tasks/gui` components
(`TaskEditor`, `RecurrenceEditor`) that render their own `<ErrorBanner>` in-form, with no way
for a consuming app to relocate it to a consistent, app-owned location. Investigating that
found the identical pattern independently duplicated in `mod-tags/gui`'s `TagEditor.tsx`, and
that the codebase already has a working precedent for exactly this class of problem — this
document proposes generalizing that precedent instead of patching each widget separately.

**Repo:** `mod-core/gui` (the shared component/hook library `mod-tasks/gui`, `mod-tags/gui`,
`mod-users/gui`, and every consuming app already depend on). Downstream repos
(`mod-tasks/gui`, `mod-tags/gui`, and every app that renders their editor components) will
need a small, mechanical follow-up change once this lands — see §4.

All file paths below are relative to `/Users/zane/playground/moduleforge/` unless noted.

---

## 1. The problem

`mod-core/gui/src/lib/use-api-error.ts`'s `useApiError` hook is, by its own doc comment,
*"the single place [the] design doc's surface-classification rule lives"*
(`docs-mf-standards/architecture/api-response-design.md` §Surface classification). It splits
every inline-classified API error into exactly three tiers:

| Tier | Trigger | Where it renders today |
|---|---|---|
| **Field** | a `details[]` entry bound to a rendered input (`options.fields`) | `<FieldError>`, positioned next to that input by the widget — **correctly local**, since only the widget knows where its own inputs are. |
| **Banner** | `forbidden`/`not_found`/`conflict`/`invalid_input` with no field match, or any unbound `details[]` entry | `<ErrorBanner>` — **rendered by the widget itself**, wherever it happens to sit in that widget's own markup. |
| **Toast** | `network_error`, `internal_error`, unrecognized codes | A **shared, app-mounted `ToastProvider`** (`mod-core/gui/src/lib/toast-context.tsx`) — already bubbles to one consistent, app-controlled surface, not rendered by the widget. |

The field tier is correctly local (an input's error belongs next to that input — no widget
should ever bubble this). The toast tier is already correctly shared (a fixed-position
overlay the app mounts once via `<ToastProvider>`, and every widget reports into via
`useToast()`). **The banner tier is the one inconsistent case**: it is classified as
"not a field error" — conceptually the same "this doesn't belong to one specific input"
judgment that puts toast errors in a shared surface — but it is still rendered by whichever
widget happens to compute it, in that widget's own layout position, with no way for the
consuming app to say "render all of these in one consistent place."

### Confirmed duplication

Both known consumers hit this identically:

- `mod-tasks/gui/src/TaskEditor.tsx:227`: `<ErrorBanner error={validationError ?? bannerError} />`
- `mod-tasks/gui/src/RecurrenceEditor.tsx:726`: same shape.
- `mod-tags/gui/src/TagEditor.tsx` (~line 260 `loadBannerError`, ~line 380 `submitBannerError`):
  same pattern, independently re-implemented.

`app-mftodo` (the concrete app that surfaced this) wants a single, consistent banner location
— directly below its top nav, at the top of body content (`app-mftodo/gui/src/App.tsx`,
between `<Header />` and `<main>`, lines ~101-103) — for *every* general error the app
produces, not just whichever widget happens to be mounted when one fires. Today that's
impossible without editing every widget that might ever render a banner.

---

## 2. The precedent already in this codebase

`mod-core/gui/src/lib/toast-context.tsx`'s `ToastProvider`/`useToast()` already solves this
exact class of problem for the toast tier:

```tsx
export function ToastProvider({ children }: ToastProviderProps) {
  const [toasts, setToasts] = React.useState<ActiveToast[]>([]);
  // ... owns the queue, renders the radix Toast.Provider + viewport itself ...
}

export function useToast(): ToastContextValue {
  const context = React.useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return context;
}
```

And `useApiError` already calls `useToast()` **unconditionally on every render** (not just on
a toast-worthy error), dispatching via a `useEffect` keyed on the error instance so a
re-render with the same error doesn't re-enqueue. Every app consuming any component that
calls `useApiError` is therefore **already required** to wrap its tree in `<ToastProvider>` —
confirmed live in `app-mftodo/gui/src/App.tsx:91-99`, whose own code comment documents this
as a real landmine already hit once: *"without this provider ancestor those editors throw
immediately on mount. Discovered during Phase 04 ... flagged ... as a required app-mftodo
composition-root fix (mod-tasks plan/followups.yaml id aqXA)."*

This is directly relevant prior art for two reasons: it proves the "shared context every
`useApiError` caller unconditionally participates in" pattern already works and is already
load-bearing in this codebase, and it's a cautionary tale — forgetting to mount the provider
is a real, already-observed failure mode (`aqXA`), not a hypothetical one. Any banner
equivalent needs to either repeat that fail-fast contract deliberately (§3 has both options)
or explicitly avoid repeating that specific landmine.

---

## 3. Proposed fix

### One structural difference from `ToastProvider` worth calling out up front

`ToastProvider` both **owns the state** and **renders the UI** (a fixed-position viewport —
it doesn't matter where in the tree it's mounted, visually). A banner is different: it's
**layout-significant** — `app-mftodo` specifically wants it below the nav, not floating.
So a `BannerProvider` should probably only own the *state*, and let each app render the
actual `<ErrorBanner>` wherever it wants in its own layout, reading from the shared context.
That's a deliberate asymmetry from `ToastProvider`, not an oversight — flag it if whoever
implements this reaches for a literal copy-paste of `ToastProvider`'s shape.

### 3.1 `BannerProvider` / `useErrorBanner()` (new, in `mod-core/gui`)

```tsx
// mod-core/gui/src/lib/banner-context.tsx (proposed path, mirroring toast-context.tsx)

export interface BannerContextValue {
  /** The current banner-level error, or null. */
  bannerError: ErrorBannerData | null;
  /** Reports (replaces) the current banner error. */
  reportError: (error: ErrorBannerData | null) => void;
  /** Clears the current banner error. */
  clearError: () => void;
}

export function BannerProvider({ children }: { children: React.ReactNode }) {
  const [bannerError, setBannerError] = React.useState<ErrorBannerData | null>(null);
  const reportError = React.useCallback((e: ErrorBannerData | null) => setBannerError(e), []);
  const clearError = React.useCallback(() => setBannerError(null), []);
  const value = React.useMemo(() => ({ bannerError, reportError, clearError }), [...]);
  return <BannerContext.Provider value={value}>{children}</BannerContext.Provider>;
  // No <ErrorBanner> rendered here — see the callout above.
}

export function useErrorBanner(): BannerContextValue { /* mirrors useToast()'s shape */ }
```

The app mounts `<BannerProvider>` (likely wrapping the same root as `<ToastProvider>`, or
nested inside it — order shouldn't matter, they're independent contexts) and renders the
actual banner itself wherever it belongs in its layout:

```tsx
// app-mftodo/gui/src/App.tsx, illustrative — the concrete shell-banner task landing
// concurrently in app-mftodo may already be doing something equivalent; reconcile rather
// than duplicate.
const { bannerError, clearError } = useErrorBanner()
return (
  <BannerProvider>
    <ToastProvider>
      <div className="min-h-screen bg-background">
        <Header />
        <ErrorBanner error={bannerError} />   {/* <-- exactly where the original bug report asked for it */}
        <main>...</main>
      </div>
    </ToastProvider>
  </BannerProvider>
)
```

### 3.2 `useApiError` changes

Two designs, genuinely both defensible — **this is the one real decision point for whoever
implements this; make it deliberately, don't default silently:**

**Option A — mirror `ToastProvider`'s fail-fast contract exactly.** `useApiError` calls
`useErrorBanner()` unconditionally (same as it already calls `useToast()` unconditionally
today) and reports `bannerError` into the shared context via a `useEffect` (same dedup
pattern already used for toast dispatch), **removing `bannerError` from
`UseApiErrorResult`'s return shape entirely**. Every current and future `useApiError` caller
must be wrapped in `<BannerProvider>` or it throws on mount — consistent with the existing
`ToastProvider` contract, but this **recreates the exact `aqXA` failure mode** for a second
provider: every consuming app must remember to add it, and nothing catches a forgotten one
until runtime.

**Option B — graceful fallback when no provider is present.** `useErrorBanner()` (or an
internal variant `useErrorBanner` calls) returns `null` instead of throwing outside a
`BannerProvider`; `useApiError` reports into the context when present, and falls back to
**returning `bannerError` from the hook** (today's behavior, unchanged) when absent. Safer
rollout — an app that hasn't added `<BannerProvider>` yet keeps working exactly as today,
banners just stay widget-local until it's updated — at the cost of `useApiError` carrying a
permanent branch and two ways banner errors can end up rendered.

This document takes no position between A and B; both are consistent with the rest of the
proposal below. Whoever implements this should pick one deliberately and say why, not
default to whichever is less code.

### 3.3 Widget-side change (mechanical, same shape in all three call sites)

Each of `TaskEditor.tsx:227`, `RecurrenceEditor.tsx:726`, `TagEditor.tsx` (~260, ~380)
currently does:

```tsx
<ErrorBanner error={validationError ?? bannerError} />
```

`validationError` is each widget's own **local, synchronous, non-API validation** (e.g.
`RecurrenceEditor`'s "at least one day must be selected") — that must stay local; it isn't
an API error `useApiError` ever sees, and there's nothing to bubble. Only the `bannerError`
half moves. Under Option A, `bannerError` no longer exists as a local variable at all, so
each site becomes `<ErrorBanner error={validationError} />` (the shared context handles the
rest, rendered elsewhere by the app). Under Option B, each site becomes conditional on
whether a provider was present — check `useErrorBanner()`'s presence and only fall back to
local `<ErrorBanner error={validationError ?? bannerError} />` rendering when it's absent.

### 3.4 Design-doc update

`docs-mf-standards/architecture/api-response-design.md`'s §Surface classification table
currently describes the banner tier as rendering "form/section-level," in-place — that
description becomes inaccurate under this change (same way the toast row already describes a
shared provider, not a widget-local render). Update it in lockstep, the same way the
`authz-create-operation` and `authz-single-row-own` plans both learned the hard way that a
hand-maintained description left to drift becomes a false-green trap for the next reader.

---

## 4. Rollout — every downstream repo needs a small follow-up

Once this lands in `mod-core/gui`:

- **`mod-tasks/gui`**: `TaskEditor.tsx`, `RecurrenceEditor.tsx` — apply §3.3's mechanical
  change (small, same shape at both sites).
- **`mod-tags/gui`**: `TagEditor.tsx` — same, at its two sites.
- **Every consuming app** (`app-mftodo` confirmed; check `app-mfmanager`, `app-mfdemo`, and
  any other app embedding `TaskEditor`/`RecurrenceEditor`/`TagEditor`) — mount
  `<BannerProvider>` (Option A: required, or it throws the same way a missing
  `<ToastProvider>` does today; Option B: optional, but banners stay non-relocatable until
  added) and render one `<ErrorBanner>` consumer wherever that app wants banners to appear.
  For `app-mftodo` specifically, that's `App.tsx`, between `<Header />` and `<main>` — the
  exact location its own shell-level-error-banner task (landing concurrently with this
  write-up) is independently trying to achieve; **reconcile the two rather than let them
  duplicate or fight over the same layout slot.**

**Not in scope for this document:** the actual `mod-tasks`/`mod-tags`/app-level follow-up
changes themselves — this is the `mod-core` design + fix only. File the downstream changes
as their own follow-ups/plans once this lands and its exact API shape (particularly the
Option A/B decision) is settled, since the downstream edit's exact shape depends on that
choice.
