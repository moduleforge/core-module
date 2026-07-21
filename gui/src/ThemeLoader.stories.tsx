import type { Story } from '@ladle/react';
import * as React from 'react';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { loadStylePackage, unloadStylePackage, type StylePackageManifest } from './lib/theme-loader';

/**
 * Manual runtime-theme-loader harness (Phase 3 task `002`). Ladle serves
 * stories in a real browser, so this demonstrates the loader's actual
 * `<link>`-injection behavior end-to-end, not a mock:
 *
 * - "Load brand-a" injects a sparse `--mf-*` override bundle at runtime —
 *   only `--mf-primary`/`--mf-brand-highlight` change; every other token
 *   keeps falling back to mod-core's `-default` set (the fallback-chaining
 *   contract).
 * - "Swap to brand-b" replaces the injected link with a different bundle,
 *   in place — no page reload.
 * - "Remove (defaults)" calls `unloadStylePackage`, returning to mod-core's
 *   baked defaults.
 *
 * Each "brand" bundle is a self-contained `data:` URI — a real absolute URL
 * the loader resolves and injects exactly as it would a CDN-hosted bundle —
 * so this harness needs no manifest server. `loadStylePackage` is called
 * with a pre-fetched manifest object (the loader's alternate `source` form
 * for a caller that already has the manifest) rather than fetching
 * `style-package.json` over HTTP.
 */

function sparseOverrideBundle(primary: string, brandHighlight: string): string {
  // Mirrors STYLE-PACKAGE-CONTRACT.md's emission rules: colors re-asserted
  // across the data-mf-theme scope selectors so brand and mode compose.
  return (
    `:root, [data-mf-theme="light"] { --mf-primary: ${primary}; --mf-brand-highlight: ${brandHighlight}; }\n` +
    `[data-mf-theme="dark"], .dark, [data-mf-theme="inverse"] { --mf-primary: ${primary}; --mf-brand-highlight: ${brandHighlight}; }`
  );
}

function manifestFor(name: string, primary: string, brandHighlight: string): StylePackageManifest {
  return {
    formatVersion: 1,
    name,
    version: '1.0.0',
    targetContractVersion: '^1.0.0',
    styleBundle: `data:text/css,${encodeURIComponent(sparseOverrideBundle(primary, brandHighlight))}`,
  };
}

const BRAND_A = manifestFor('brand-a', 'oklch(0.55 0.22 25)', 'oklch(0.72 0.20 40)');
const BRAND_B = manifestFor('brand-b', 'oklch(0.60 0.20 150)', 'oklch(0.75 0.18 160)');

function ThemedPanel() {
  return (
    <Card className="w-64">
      <CardHeader>
        <CardTitle>Runtime-loaded brand</CardTitle>
        <CardDescription>--mf-primary / --mf-brand-highlight</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col items-start gap-2">
        <Button>Primary action</Button>
        <Badge>Badge</Badge>
      </CardContent>
    </Card>
  );
}

export const RuntimeSwap: Story = () => {
  const [active, setActive] = React.useState('none (mod-core defaults)');

  async function apply(manifest: StylePackageManifest | null): Promise<void> {
    if (!manifest) {
      unloadStylePackage();
      setActive('none (mod-core defaults)');
      return;
    }
    const result = await loadStylePackage(manifest, { baseUrl: window.location.href });
    setActive(result.status === 'loaded' ? manifest.name : `${manifest.name} (skipped: strict mismatch)`);
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex gap-2">
        <Button variant="secondary" onClick={() => void apply(BRAND_A)}>
          Load brand-a
        </Button>
        <Button variant="secondary" onClick={() => void apply(BRAND_B)}>
          Swap to brand-b
        </Button>
        <Button variant="secondary" onClick={() => void apply(null)}>
          Remove (defaults)
        </Button>
      </div>
      <p className="text-sm text-muted-foreground">
        Active style package: <strong>{active}</strong>. No reload between clicks — the loader replaces the
        injected &lt;link&gt; in place.
      </p>
      <ThemedPanel />
    </div>
  );
};
