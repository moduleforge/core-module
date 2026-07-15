import type { FieldError as FieldErrorData } from './lib/api-types';
import { cn } from './lib/utils';

export interface FieldErrorProps {
  /**
   * The field-scoped error to render, or `undefined`/`null` for "no error"
   * (renders nothing). Matches the task-001 `FieldError` wire type — no
   * local redefinition here.
   */
  error?: FieldErrorData | null;
  /** Element `id`; wire it to the bound input's `aria-describedby` to associate the two. */
  id?: string;
  className?: string;
}

/**
 * Field-level (inline, per-input) error surface — the "Field-level" row of
 * the surface-classification table in
 * docs/mf-standards/architecture/api-response-design.md. Presentational
 * only: the caller decides which `FieldError` binds to which input (that
 * routing lives in `useApiError`, task 004), this component just renders it
 * with an accessible association a caller wires up via `id`/`aria-describedby`.
 */
export function FieldError({ error, id, className }: FieldErrorProps) {
  if (!error) return null;

  return (
    <p id={id} role="alert" className={cn('text-sm text-destructive', className)}>
      {error.message}
    </p>
  );
}
