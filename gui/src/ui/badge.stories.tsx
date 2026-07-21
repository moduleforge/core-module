import type { Story } from '@ladle/react';
import { Badge } from './badge';

/**
 * Task-002 primitive audit: `badge` had no Ladle coverage before this pass. See
 * `../tokens/CONTRACT.md` for the `data-mf-theme` scoping attribute these stories exercise.
 */

const VARIANTS = ['default', 'secondary', 'destructive', 'outline'] as const;

export const Variants: Story = () => (
  <div className="flex flex-wrap items-center gap-3">
    {VARIANTS.map((variant) => (
      <Badge key={variant} variant={variant}>
        {variant}
      </Badge>
    ))}
  </div>
);

/**
 * Proves every variant still renders correctly when its subtree is flipped via
 * `data-mf-theme="inverse"`.
 */
export const InverseSection: Story = () => (
  <div className="flex flex-col gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">Page mode (default)</span>
      <div className="flex flex-wrap items-center gap-3">
        {VARIANTS.map((variant) => (
          <Badge key={variant} variant={variant}>
            {variant}
          </Badge>
        ))}
      </div>
    </div>
    <div
      className="flex flex-col gap-2 rounded-lg bg-background p-4"
      data-mf-theme="inverse"
    >
      <span className="text-sm font-medium">data-mf-theme=&quot;inverse&quot;</span>
      <div className="flex flex-wrap items-center gap-3">
        {VARIANTS.map((variant) => (
          <Badge key={variant} variant={variant}>
            {variant}
          </Badge>
        ))}
      </div>
    </div>
  </div>
);
