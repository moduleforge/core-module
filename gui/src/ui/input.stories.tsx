import type { Story } from '@ladle/react';
import { Input } from './input';
import { Label } from './label';

/**
 * Task-002 primitive audit: `input` had no Ladle coverage before this pass. See
 * `../tokens/CONTRACT.md` for the `data-mf-theme` scoping attribute these stories exercise.
 */

export const States: Story = () => (
  <div className="flex max-w-sm flex-col gap-4">
    <div className="flex flex-col gap-1.5">
      <Label htmlFor="input-default">Default</Label>
      <Input id="input-default" placeholder="you@example.com" />
    </div>
    <div className="flex flex-col gap-1.5">
      <Label htmlFor="input-disabled">Disabled</Label>
      <Input id="input-disabled" defaultValue="Locked value" disabled />
    </div>
    <div className="flex flex-col gap-1.5">
      <Label htmlFor="input-invalid">Invalid</Label>
      <Input id="input-invalid" defaultValue="not-an-email" aria-invalid="true" />
    </div>
  </div>
);

/**
 * Proves the input's border/ring/placeholder colors (all semantic-token-backed) still resolve
 * correctly when the subtree is flipped via `data-mf-theme="inverse"`.
 */
export const InverseSection: Story = () => (
  <div className="flex flex-col gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">Page mode (default)</span>
      <div className="max-w-sm">
        <Label htmlFor="input-page">Email</Label>
        <Input id="input-page" placeholder="you@example.com" />
      </div>
    </div>
    <div
      className="flex flex-col gap-2 rounded-lg bg-background p-4"
      data-mf-theme="inverse"
    >
      <span className="text-sm font-medium">data-mf-theme=&quot;inverse&quot;</span>
      <div className="max-w-sm">
        <Label htmlFor="input-inverse">Email</Label>
        <Input id="input-inverse" placeholder="you@example.com" />
      </div>
    </div>
  </div>
);
