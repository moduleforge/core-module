import type { Story } from '@ladle/react';
import {
  ToastProviderPrimitive,
  ToastViewport,
  ToastRoot,
  ToastTitle,
  ToastDescription,
  ToastClose,
} from './toast';

/**
 * Task-002 primitive audit: `toast` had no Ladle coverage before this pass. Toasts are rendered
 * statically open here (real usage goes through `useToast()` in `lib/toast-context`, which
 * manages the open/close lifecycle) purely to exercise the visual variants. See
 * `../tokens/CONTRACT.md` for the `data-mf-theme` scoping attribute these stories exercise.
 */

function ExampleToasts() {
  return (
    <ToastProviderPrimitive>
      <ToastRoot open onOpenChange={() => {}} className="static mb-3 translate-x-0">
        <div className="grid gap-1">
          <ToastTitle>Saved</ToastTitle>
          <ToastDescription>Your changes have been saved.</ToastDescription>
        </div>
        <ToastClose />
      </ToastRoot>
      <ToastRoot
        variant="destructive"
        open
        onOpenChange={() => {}}
        className="static translate-x-0"
      >
        <div className="grid gap-1">
          <ToastTitle>Error</ToastTitle>
          <ToastDescription>Could not save your changes.</ToastDescription>
        </div>
        <ToastClose />
      </ToastRoot>
      <ToastViewport className="static max-w-sm p-0" />
    </ToastProviderPrimitive>
  );
}

export const Variants: Story = () => <ExampleToasts />;

/**
 * Demonstrates both variants' token-level color scoping when the subtree is flipped via
 * `data-mf-theme="inverse"`. This confirms the token flip itself, not full `dark:`-variant
 * parity: the destructive variant also carries a `dark:border-destructive` modifier that does
 * not retrigger under `data-mf-theme="inverse"` — a known, documented gap (see
 * `../tokens/CONTRACT.md`'s "Known limitation").
 */
export const InverseSection: Story = () => (
  <div className="flex flex-col gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">Page mode (default)</span>
      <ExampleToasts />
    </div>
    <div
      className="flex flex-col gap-2 rounded-lg bg-background p-4"
      data-mf-theme="inverse"
    >
      <span className="text-sm font-medium">data-mf-theme=&quot;inverse&quot;</span>
      <ExampleToasts />
    </div>
  </div>
);
