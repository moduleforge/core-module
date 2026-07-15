import * as React from 'react';
import {
  ToastProviderPrimitive,
  ToastViewport,
  ToastRoot,
  ToastTitle,
  ToastDescription,
  ToastClose,
} from '../ui/toast';

/** Visual treatment for a toast. Mirrors `Alert`'s variant set. */
export type ToastVariant = 'default' | 'destructive';

/** The default auto-dismiss duration (ms) applied when a toast omits one. */
const DEFAULT_TOAST_DURATION = 5000;

/** Upper bound on simultaneously queued toasts; oldest entries are dropped past this. */
const MAX_TOASTS = 5;

/** Options accepted by `useToast().toast(...)`. */
export interface ToastOptions {
  /** Optional short heading. */
  title?: string;
  /** The toast body copy. */
  description: string;
  /** Visual treatment; defaults to `'default'`. */
  variant?: ToastVariant;
  /** Auto-dismiss duration in ms; defaults to `DEFAULT_TOAST_DURATION`. */
  duration?: number;
}

interface ActiveToast extends ToastOptions {
  id: string;
}

/** Dispatch API returned by `useToast()`. */
export interface ToastContextValue {
  /** Enqueues a toast and returns its id (usable with `dismiss`). */
  toast: (options: ToastOptions) => string;
  /** Dismisses (removes) the toast with the given id, if still active. */
  dismiss: (id: string) => void;
}

const ToastContext = React.createContext<ToastContextValue | null>(null);

function createToastId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  // Fallback for environments without the Web Crypto API (e.g. older SSR
  // runtimes); uniqueness within a single provider instance is all that's
  // required, since ids are only ever compared to other ids it generated.
  return `toast-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

export interface ToastProviderProps {
  children: React.ReactNode;
}

/**
 * Owns the toast queue and renders the radix `Toast.Provider` + viewport.
 * Wrap any app root in this once; it is not coupled to a specific shell.
 */
export function ToastProvider({ children }: ToastProviderProps) {
  const [toasts, setToasts] = React.useState<ActiveToast[]>([]);

  const dismiss = React.useCallback((id: string) => {
    setToasts((current) => current.filter((t) => t.id !== id));
  }, []);

  const toast = React.useCallback((options: ToastOptions) => {
    const id = createToastId();
    setToasts((current) => [...current, { id, ...options }].slice(-MAX_TOASTS));
    return id;
  }, []);

  const value = React.useMemo<ToastContextValue>(
    () => ({ toast, dismiss }),
    [toast, dismiss],
  );

  return (
    <ToastContext.Provider value={value}>
      <ToastProviderPrimitive duration={DEFAULT_TOAST_DURATION}>
        {children}
        {toasts.map(({ id, title, description, variant, duration }) => (
          <ToastRoot
            key={id}
            variant={variant}
            duration={duration}
            onOpenChange={(open) => {
              if (!open) dismiss(id);
            }}
          >
            <div className="grid gap-1">
              {title ? <ToastTitle>{title}</ToastTitle> : null}
              <ToastDescription>{description}</ToastDescription>
            </div>
            <ToastClose />
          </ToastRoot>
        ))}
        <ToastViewport />
      </ToastProviderPrimitive>
    </ToastContext.Provider>
  );
}

/**
 * Returns the toast dispatch API. Must be called within a `ToastProvider`;
 * throws a clear error otherwise so misuse fails fast in development.
 */
export function useToast(): ToastContextValue {
  const context = React.useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return context;
}
