import type { Story } from '@ladle/react';
import { Button } from './button';

/**
 * Task-002 primitive audit: `button` had no Ladle coverage at all before this pass. These
 * stories exercise every `variant`/`size` combination and prove the button renders correctly
 * both under the page-level light/dark toggle (Ladle's theme addon, which sets the global
 * `data-mf-theme`) and inside an explicit `data-mf-theme="inverse"` subtree (see
 * `../tokens/CONTRACT.md`).
 */

const VARIANTS = ['default', 'outline', 'secondary', 'ghost', 'destructive', 'link'] as const;
const SIZES = ['default', 'xs', 'sm', 'lg', 'icon', 'icon-xs', 'icon-sm', 'icon-lg'] as const;

export const Variants: Story = () => (
  <div className="flex flex-wrap items-center gap-3">
    {VARIANTS.map((variant) => (
      <Button key={variant} variant={variant}>
        {variant}
      </Button>
    ))}
  </div>
);

export const Sizes: Story = () => (
  <div className="flex flex-wrap items-center gap-3">
    {SIZES.map((size) => (
      <Button key={size} size={size}>
        {size.startsWith('icon') ? '+' : size}
      </Button>
    ))}
  </div>
);

export const States: Story = () => (
  <div className="flex flex-wrap items-center gap-3">
    <Button>Default</Button>
    <Button disabled>Disabled</Button>
    <Button aria-invalid="true">Invalid</Button>
  </div>
);

/**
 * Demonstrates every variant's token-level color scoping under `data-mf-theme="inverse"` — the
 * scoping value that reassigns the `--mf-*` color custom properties for a subtree independent of
 * the page's own light/dark mode. This confirms the token flip itself, not full `dark:`-variant
 * parity: `button` also carries `dark:`-gated opacity/emphasis modifiers (e.g.
 * `dark:border-input dark:bg-input/30`) that do not retrigger under `data-mf-theme="inverse"` — a
 * known, documented gap (see `../tokens/CONTRACT.md`'s "Known limitation").
 */
export const InverseSection: Story = () => (
  <div className="flex flex-col gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">Page mode (default)</span>
      <div className="flex flex-wrap items-center gap-3">
        {VARIANTS.map((variant) => (
          <Button key={variant} variant={variant}>
            {variant}
          </Button>
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
          <Button key={variant} variant={variant}>
            {variant}
          </Button>
        ))}
      </div>
    </div>
  </div>
);
