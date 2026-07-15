# Error Widgets

## Purpose and scope

Build the two inline error-surface widgets the design doc's surface-classification table names:
`<FieldError>` (field-level, per-input) and `<ErrorBanner>` (banner-level, form/section). `<ErrorBanner>`
is a **promotion target** — the generalisation of mod-users' `ErrorMessage`, wrapping the existing
shared `Alert` (`destructive` variant); other repos' components switch to it in later plans (not this
plan).

Source of truth: the **GUI-facing error-data contract** → **Surface classification** and **Widget set
implied** subsections of `docs/mf-standards/architecture/api-response-design.md`.

Depends on: `phase-02-gui-error-widgets/001-client-foundation.md` (for the `FieldError` wire type).

Skill: `implement-task` (TypeScript/React).

## Requirements

Create the widgets under `gui/src/` (e.g. `gui/src/FieldError.tsx` and `gui/src/ErrorBanner.tsx`,
alongside the existing form components, or under a small `gui/src/errors/` folder — implementer's
choice; keep barrel-exportable and consistent with existing component placement).

1. **`<FieldError>`** (exported): binds one `FieldError` (from the task-001 wire types) to an input,
   rendered inline beside/below the field. Props accept a single `FieldError` (or `undefined`/`null`
   for "no error", rendering nothing). Render the `message` with an accessible association (e.g.
   `role="alert"` / an `id` a caller can wire to `aria-describedby`); keep it small and presentational.
   Style with existing Tailwind/`cn` conventions (destructive text color consistent with `Alert`
   destructive).

2. **`<ErrorBanner>`** (exported): wraps the existing `Alert` (`destructive` variant) from
   `gui/src/ui/alert.tsx` for banner-level errors. Accepts a banner error (an `ApiError`/message-bearing
   shape, or a plain string/title+description) and renders `Alert`/`AlertTitle`/`AlertDescription`
   with `variant="destructive"`. This is the shared generalisation of mod-users' `ErrorMessage` — keep
   its API general enough to cover the "top-level `forbidden`/`not_found`/`conflict`/`invalid_input`
   with no field-bound details" banner case from the surface table.

3. Export both from `gui/src/index.ts` alongside existing exports.

Both widgets are presentational only — the routing decision (which error goes to field vs banner vs
toast) lives in `useApiError` (task 004), not in these widgets. Do not add a dependency. No `any`.

## Validation

- `gui/src/FieldError.tsx` (or chosen path) and `gui/src/ErrorBanner.tsx` exist and are exported from
  `gui/src/index.ts`.
- `<ErrorBanner>` renders the shared `Alert` with `variant="destructive"` (grep/read confirms it
  imports and uses `Alert` from `./ui/alert` rather than re-implementing).
- `<FieldError>` consumes the task-001 `FieldError` type (no local redefinition).
- `cd gui && bun run typecheck` passes; `cd gui && bun run build` succeeds.

## Metadata

architectural_impact: true

## Assumptions

- Task 001 has landed: the `FieldError` wire type is exported from the gui client module.

## References

- `docs/mf-standards/architecture/api-response-design.md` — **Surface classification** table and
  **Widget set implied** (`<FieldError>`, `<ErrorBanner>` as the `mod-core/gui` promotion of
  `ErrorMessage`).
- `gui/src/ui/alert.tsx` — the `Alert`/`AlertTitle`/`AlertDescription` + `destructive` variant that
  `<ErrorBanner>` wraps.
- `gui/src/lib/utils.ts` — `cn` styling helper.
- `plan/phase-02-gui-error-widgets/001-client-foundation.md` — provides the `FieldError` type.
- `mod-users`' `ErrorMessage` — conceptual origin of `<ErrorBanner>` (reference only; different repo).

## Checkpoint hints

- After `<FieldError>` typechecks.
- After `<ErrorBanner>` wraps `Alert` and typechecks.
- After barrel exports and a clean typecheck.
