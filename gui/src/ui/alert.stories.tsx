import type { Story } from '@ladle/react';
import { Terminal, AlertCircle } from 'lucide-react';
import { Alert, AlertTitle, AlertDescription } from './alert';

/**
 * Task-002 primitive audit: `alert` had no Ladle coverage before this pass. See
 * `../tokens/CONTRACT.md` for the `data-mf-theme` scoping attribute these stories exercise.
 */

function ExampleAlerts() {
  return (
    <div className="flex flex-col gap-4">
      <Alert>
        <Terminal className="size-4" />
        <AlertTitle>Heads up</AlertTitle>
        <AlertDescription>You can add components to your app using the CLI.</AlertDescription>
      </Alert>
      <Alert variant="destructive">
        <AlertCircle className="size-4" />
        <AlertTitle>Error</AlertTitle>
        <AlertDescription>Your session has expired. Please log in again.</AlertDescription>
      </Alert>
    </div>
  );
}

export const Variants: Story = () => <ExampleAlerts />;

/**
 * Proves both variants still render correctly when the subtree is flipped via
 * `data-mf-theme="inverse"`.
 */
export const InverseSection: Story = () => (
  <div className="flex flex-col gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">Page mode (default)</span>
      <div className="max-w-md">
        <ExampleAlerts />
      </div>
    </div>
    <div
      className="flex flex-col gap-2 rounded-lg bg-background p-4"
      data-mf-theme="inverse"
    >
      <span className="text-sm font-medium">data-mf-theme=&quot;inverse&quot;</span>
      <div className="max-w-md">
        <ExampleAlerts />
      </div>
    </div>
  </div>
);
