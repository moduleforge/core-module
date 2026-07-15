# Toast Provider

## Purpose and scope

Build a Toast provider and `useToast` hook for mod-core/gui, the transient/global surface for
toast-worthy errors (`network_error`, `internal_error`/500, optimistic-update rollbacks) that the
design doc's surface-classification table routes away from inline widgets. Built on the **radix-ui Toast
primitive, which is already a declared dependency** (`radix-ui` in `gui/package.json`, resolving
`@radix-ui/react-toast` in the lockfile) — **do not add any new external dependency**.

Source of truth: the **GUI-facing error-data contract** section of
`docs/mf-standards/architecture/api-response-design.md` (Toast provider row of the surface table;
"Widget set implied"). This task builds the provider surface; `useApiError` (task 004) dispatches to it.

Skill: `implement-task` (TypeScript/React).

## Requirements

Create the toast module (e.g. `gui/src/ui/toast.tsx` for the presentational primitives and
`gui/src/lib/toast-context.tsx` for the provider/hook, or a single cohesive module — implementer's
choice; keep it barrel-exportable):

1. **Toast primitives** wrapping radix Toast, following the existing namespace-import style
   (`import { Toast } from "radix-ui"`, matching `gui/src/ui/button.tsx`'s `import { Slot } from
   "radix-ui"`). Use `Toast.Provider`, `Toast.Root`, `Toast.Title`, `Toast.Description`, `Toast.Close`,
   `Toast.Viewport`. Style with the existing Tailwind/`cn` conventions (see `gui/src/lib/utils.ts`,
   `gui/src/ui/alert.tsx`); support at least a default and a `destructive`/error variant consistent
   with `Alert`'s `destructive` variant.

2. **`ToastProvider`** component (exported): wraps children in radix `Toast.Provider`, renders the
   `Toast.Viewport`, and owns the toast queue state (list of active toasts with id/title/description/
   variant/duration). Exposes an imperative `toast(...)` dispatch via context.

3. **`useToast` hook** (exported): returns the dispatch API — at minimum `toast({ title?, description,
   variant?, duration? })` to enqueue a toast, and a way to dismiss. The hook reads the provider
   context and throws a clear error if used outside a `ToastProvider`.

4. **Accessibility & cleanup**: radix handles ARIA/roving focus; ensure toasts auto-dismiss on a
   sensible default duration and are removable. Keep the surface minimal and typed (no `any`).

5. Export `ToastProvider`, `useToast`, and the toast option/type from `gui/src/index.ts` (and
   `gui/src/ui/index.ts` if the primitives live under `ui/`).

Do not add a new dependency. Do not couple to a specific app shell — the provider must be droppable at
any app root.

## Validation

- Toast module exists; `ToastProvider` and `useToast` are exported and reachable from
  `gui/src/index.ts`.
- `grep -rn "radix-ui" gui/src/**/toast*.tsx gui/src/**/*toast*.tsx 2>/dev/null` shows the Toast
  primitive is imported from the existing `radix-ui` package; `git diff gui/package.json` shows **no**
  new dependency added.
- `cd gui && bun run typecheck` (tsc --noEmit) passes.
- `cd gui && bun run build` (tsup) succeeds.
- Code review confirms `useToast` throws when used outside `ToastProvider`, and the destructive/error
  variant is available for toast-worthy errors.

## Metadata

architectural_impact: true

## References

- `docs/mf-standards/architecture/api-response-design.md` — **GUI-facing error-data contract**
  (Toast-worthy surface row; "Widget set implied" Toast provider).
- `gui/src/ui/button.tsx` — the existing `import { Slot } from "radix-ui"` namespace pattern to mirror.
- `gui/src/ui/alert.tsx`, `gui/src/lib/utils.ts` — styling/`cn`/variant conventions and the
  `destructive` variant to stay consistent with.
- `gui/package.json` / `gui/bun.lock` — confirms `radix-ui` (with `@radix-ui/react-toast`) is already a
  dependency; no addition permitted.

## Checkpoint hints

- After the radix Toast primitives render and typecheck.
- After `ToastProvider` + context state.
- After `useToast` + barrel export and a clean typecheck.
