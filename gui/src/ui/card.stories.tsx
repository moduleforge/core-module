import type { Story } from '@ladle/react';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from './card';
import { Button } from './button';

/**
 * Task-002 primitive audit: `card` had no Ladle coverage before this pass. See
 * `../tokens/CONTRACT.md` for the `data-mf-theme` scoping attribute these stories exercise.
 */

function ExampleCard() {
  return (
    <Card className="w-72">
      <CardHeader>
        <CardTitle>Team plan</CardTitle>
        <CardDescription>Billed monthly, cancel anytime.</CardDescription>
      </CardHeader>
      <CardContent>
        <p className="text-sm">Everything in Starter, plus unlimited seats.</p>
      </CardContent>
      <CardFooter>
        <Button className="w-full">Upgrade</Button>
      </CardFooter>
    </Card>
  );
}

export const Default: Story = () => <ExampleCard />;

/**
 * Proves the card's surface/border/foreground colors still resolve correctly when the subtree
 * is flipped via `data-mf-theme="inverse"`.
 */
export const InverseSection: Story = () => (
  <div className="flex flex-wrap gap-6">
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">Page mode (default)</span>
      <ExampleCard />
    </div>
    <div
      className="flex flex-col gap-2 rounded-lg bg-background p-4"
      data-mf-theme="inverse"
    >
      <span className="text-sm font-medium">data-mf-theme=&quot;inverse&quot;</span>
      <ExampleCard />
    </div>
  </div>
);
