import * as React from 'react';
import { Toast } from 'radix-ui';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '../lib/utils';

const toastVariants = cva(
  'group pointer-events-auto relative flex w-full items-center justify-between gap-2 overflow-hidden rounded-lg border p-4 pr-8 shadow-lg transition-all data-[swipe=cancel]:translate-x-0 data-[swipe=end]:translate-x-[var(--radix-toast-swipe-end-x)] data-[swipe=move]:translate-x-[var(--radix-toast-swipe-move-x)] data-[swipe=move]:transition-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[swipe=end]:animate-out data-[state=closed]:fade-out-80 data-[state=closed]:slide-out-to-right-full data-[state=open]:slide-in-from-top-full data-[state=open]:sm:slide-in-from-bottom-full',
  {
    variants: {
      variant: {
        default: 'border-border bg-background text-foreground',
        destructive:
          'border-destructive/50 bg-background text-destructive dark:border-destructive',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
);

/**
 * The radix `Toast.Provider` wrapper. One instance must sit above every
 * `ToastRoot` rendered by the queue; `ToastProvider` in `lib/toast-context`
 * owns placing this alongside the viewport.
 */
function ToastProviderPrimitive(props: React.ComponentProps<typeof Toast.Provider>) {
  return <Toast.Provider {...props} />;
}

function ToastViewport({
  className,
  ...props
}: React.ComponentProps<typeof Toast.Viewport>) {
  return (
    <Toast.Viewport
      data-slot="toast-viewport"
      className={cn(
        'fixed top-0 z-[100] flex max-h-screen w-full flex-col-reverse p-4 sm:top-auto sm:right-0 sm:bottom-0 sm:flex-col md:max-w-[420px]',
        className,
      )}
      {...props}
    />
  );
}

interface ToastRootProps
  extends React.ComponentProps<typeof Toast.Root>,
    VariantProps<typeof toastVariants> {}

function ToastRoot({ className, variant, ...props }: ToastRootProps) {
  return (
    <Toast.Root
      data-slot="toast"
      data-variant={variant ?? 'default'}
      className={cn(toastVariants({ variant }), className)}
      {...props}
    />
  );
}

function ToastTitle({
  className,
  ...props
}: React.ComponentProps<typeof Toast.Title>) {
  return (
    <Toast.Title
      data-slot="toast-title"
      className={cn('text-sm font-medium', className)}
      {...props}
    />
  );
}

function ToastDescription({
  className,
  ...props
}: React.ComponentProps<typeof Toast.Description>) {
  return (
    <Toast.Description
      data-slot="toast-description"
      className={cn('text-sm opacity-90', className)}
      {...props}
    />
  );
}

function ToastClose({
  className,
  ...props
}: React.ComponentProps<typeof Toast.Close>) {
  return (
    <Toast.Close
      data-slot="toast-close"
      aria-label="Close"
      className={cn(
        'absolute top-2 right-2 rounded-md p-1 text-foreground/50 opacity-0 transition-opacity outline-none hover:text-foreground focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-ring/50 group-hover:opacity-100',
        className,
      )}
      {...props}
    >
      <span aria-hidden="true" className="block text-base leading-none">
        &times;
      </span>
    </Toast.Close>
  );
}

export {
  ToastProviderPrimitive,
  ToastViewport,
  ToastRoot,
  ToastTitle,
  ToastDescription,
  ToastClose,
  toastVariants,
  type ToastRootProps,
};
