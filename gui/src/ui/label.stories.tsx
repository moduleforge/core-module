import type { Story } from '@ladle/react';
import { Label } from './label';
import { Input } from './input';

/**
 * Task-002 primitive audit: `label` had no Ladle coverage before this pass. See
 * `../tokens/CONTRACT.md` for the `data-mf-theme` scoping attribute these stories exercise.
 */

export const Default: Story = () => (
  <div className="flex flex-col gap-1.5 max-w-sm">
    <Label htmlFor="label-default">First name</Label>
    <Input id="label-default" placeholder="Alex" />
  </div>
);

/** `group-data-[disabled=true]/field` styling, exercised via the `group/field` marker. */
export const DisabledField: Story = () => (
  <div className="group/field flex flex-col gap-1.5 max-w-sm" data-disabled="true">
    <Label htmlFor="label-disabled">First name</Label>
    <Input id="label-disabled" placeholder="Alex" disabled />
  </div>
);

/**
 * Proves the label's text color still resolves correctly when the subtree is flipped via
 * `data-mf-theme="inverse"`.
 */
export const InverseSection: Story = () => (
  <div className="flex flex-col gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">Page mode (default)</span>
      <div className="flex flex-col gap-1.5 max-w-sm">
        <Label htmlFor="label-page">First name</Label>
        <Input id="label-page" placeholder="Alex" />
      </div>
    </div>
    <div
      className="flex flex-col gap-2 rounded-lg bg-background p-4"
      data-mf-theme="inverse"
    >
      <span className="text-sm font-medium">data-mf-theme=&quot;inverse&quot;</span>
      <div className="flex flex-col gap-1.5 max-w-sm">
        <Label htmlFor="label-inverse">First name</Label>
        <Input id="label-inverse" placeholder="Alex" />
      </div>
    </div>
  </div>
);
