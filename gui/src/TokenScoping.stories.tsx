import type { Story } from '@ladle/react';
import * as React from 'react';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from './ui/card';

/**
 * Demonstrates the two mechanisms defined in `tokens/CONTRACT.md`:
 *   1. the `var(--mf-x, var(--mf-x-default))` fallback-chaining contract, and
 *   2. the unified `data-mf-theme` scoping attribute (light / dark / inverse).
 *
 * The global Ladle theme addon sets `data-mf-theme` on the page wrapper
 * (`.ladle/components.tsx`); these stories additionally scope subtrees to prove per-subtree
 * reassignment. None of these panels set a bare `--mf-*` except the explicit override demo.
 */

function Swatch({ label }: { label: string }) {
  return (
    <Card className="w-56">
      <CardHeader>
        <CardTitle>{label}</CardTitle>
        <CardDescription>surface / text / primary tokens</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col items-start gap-2">
        <Button>Primary action</Button>
        <Button variant="secondary">Secondary</Button>
        <Badge>Badge</Badge>
        <p className="text-muted-foreground text-sm">Muted body copy.</p>
      </CardContent>
    </Card>
  );
}

/**
 * Each panel scopes a subtree with `data-mf-theme`. The reassignment is local to the subtree:
 * a `dark` island on a light page renders dark, a `light` island on a dark page renders light,
 * and `inverse` flips to the opposite surface of the surrounding page mode.
 */
export const Scoping: Story = () => (
  <div className="flex flex-wrap gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">inherits page mode</span>
      <Swatch label="Default" />
    </div>
    <div className="flex flex-col gap-2" data-mf-theme="light">
      <span className="text-sm font-medium">data-mf-theme=&quot;light&quot;</span>
      <Swatch label="Light island" />
    </div>
    <div className="flex flex-col gap-2" data-mf-theme="dark">
      <span className="text-sm font-medium">data-mf-theme=&quot;dark&quot;</span>
      <Swatch label="Dark island" />
    </div>
    <div className="flex flex-col gap-2" data-mf-theme="inverse">
      <span className="text-sm font-medium">data-mf-theme=&quot;inverse&quot;</span>
      <Swatch label="Inverse section" />
    </div>
  </div>
);

/**
 * Proves the fallback chain. The left panel sets nothing, so every token resolves to its baked
 * `--mf-x-default`. The right panel sets a single bare `--mf-primary` inline — a stand-in for a
 * sparse style-package override — and only the primary surface changes; every other token still
 * falls back to its default. This is exactly the sparse-override behavior a style package relies
 * on: omit a token and it degrades to the default rather than to `unset`.
 */
export const FallbackChain: Story = () => (
  <div className="flex flex-wrap gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">no override — falls back to --mf-primary-default</span>
      <Swatch label="Default primary" />
    </div>
    <div
      className="flex flex-col gap-2"
      style={{ ['--mf-primary' as string]: 'oklch(0.55 0.22 25)' } as React.CSSProperties}
    >
      <span className="text-sm font-medium">--mf-primary overridden (sparse style pkg)</span>
      <Swatch label="Overridden primary" />
    </div>
  </div>
);
