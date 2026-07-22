import { afterEach, describe, expect, mock, test } from 'bun:test';
import {
  getActiveStylePackage,
  loadStylePackage,
  setThemeMode,
  unloadStylePackage,
  type StylePackageManifest,
} from './theme-loader';
import { MF_TOKEN_CONTRACT_VERSION } from './token-contract-version';

const originalFetch = global.fetch;

const BASE_URL = 'https://cdn.example.com/style-liquid-labs/1.0.0';

function manifestResponse(manifest: Partial<StylePackageManifest>): Response {
  return new Response(JSON.stringify(manifest), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

function baseManifest(overrides: Partial<StylePackageManifest> = {}): StylePackageManifest {
  return {
    formatVersion: 1,
    name: 'liquid-labs',
    version: '1.0.0',
    targetContractVersion: `^${MF_TOKEN_CONTRACT_VERSION}`,
    styleBundle: './style.css',
    assets: {
      logos: {
        mark: { light: './logo-mark-light.svg', dark: './logo-mark-dark.svg' },
        wordmark: './logo-wordmark.svg',
      },
      fonts: [{ family: 'Liquid Sans', src: ['./fonts/liquid-sans.woff2'], weight: '100 900', display: 'swap' }],
    },
    ...overrides,
  };
}

function activeLinks(): HTMLLinkElement[] {
  return Array.from(document.head.querySelectorAll('link[data-mf-style-package="true"]'));
}

describe('loadStylePackage', () => {
  afterEach(() => {
    global.fetch = originalFetch;
    unloadStylePackage();
    document.documentElement.removeAttribute('data-mf-theme');
  });

  test('fetches the manifest from <base>/style-package.json and injects a versioned <link>', async () => {
    global.fetch = mock((input: string | URL) => {
      expect(input.toString()).toBe(`${BASE_URL}/style-package.json`);
      return Promise.resolve(manifestResponse(baseManifest()));
    }) as unknown as typeof fetch;

    const result = await loadStylePackage(BASE_URL);

    expect(result.status).toBe('loaded');
    expect(result.contractCompatible).toBe(true);
    expect(result.bundleUrl).toBe(`${BASE_URL}/style.css?mf-style-version=1.0.0`);
    expect(activeLinks()).toHaveLength(1);
    expect(activeLinks()[0].getAttribute('href')).toBe(result.bundleUrl ?? null);
  });

  test('resolves brand-asset manifest URLs (logos/fonts) to absolute URLs against the manifest location', async () => {
    global.fetch = mock(() => Promise.resolve(manifestResponse(baseManifest()))) as unknown as typeof fetch;

    const result = await loadStylePackage(BASE_URL);

    expect(result.assets?.logos?.mark).toEqual({
      light: `${BASE_URL}/logo-mark-light.svg`,
      dark: `${BASE_URL}/logo-mark-dark.svg`,
    });
    expect(result.assets?.logos?.wordmark).toBe(`${BASE_URL}/logo-wordmark.svg`);
    expect(result.assets?.fonts?.[0].src).toEqual([`${BASE_URL}/fonts/liquid-sans.woff2`]);
  });

  test('swapping to a different package replaces the injected link — only one link remains', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ name: 'liquid-labs', styleBundle: './style.css' }))),
    ) as unknown as typeof fetch;
    const first = await loadStylePackage(BASE_URL);

    global.fetch = mock(() =>
      Promise.resolve(
        manifestResponse(baseManifest({ name: 'other-brand', version: '2.0.0', styleBundle: './other.css' })),
      ),
    ) as unknown as typeof fetch;
    const second = await loadStylePackage(BASE_URL);

    expect(activeLinks()).toHaveLength(1);
    expect(activeLinks()[0].getAttribute('href')).toBe(second.bundleUrl ?? null);
    expect(second.bundleUrl).not.toBe(first.bundleUrl);
    expect(getActiveStylePackage()?.manifest.name).toBe('other-brand');
  });

  test('unloadStylePackage removes the injected link, returning to mod-core defaults', async () => {
    global.fetch = mock(() => Promise.resolve(manifestResponse(baseManifest()))) as unknown as typeof fetch;
    await loadStylePackage(BASE_URL);
    expect(activeLinks()).toHaveLength(1);

    unloadStylePackage();

    expect(activeLinks()).toHaveLength(0);
    expect(getActiveStylePackage()).toBeNull();
  });

  test('a package with no style package loaded leaves no link injected at all (loader is additive, never required)', () => {
    expect(activeLinks()).toHaveLength(0);
    expect(getActiveStylePackage()).toBeNull();
  });

  test('accepts a pre-fetched manifest object plus an explicit baseUrl, without calling fetch', async () => {
    global.fetch = mock(() => {
      throw new Error('fetch should not be called when a manifest object is provided');
    }) as unknown as typeof fetch;

    const result = await loadStylePackage(baseManifest(), { baseUrl: BASE_URL });

    expect(result.status).toBe('loaded');
    expect(result.bundleUrl).toBe(`${BASE_URL}/style.css?mf-style-version=1.0.0`);
  });

  test('throws when a manifest object is provided without baseUrl', async () => {
    await expect(loadStylePackage(baseManifest())).rejects.toThrow(/baseUrl is required/);
  });

  test('throws on a malformed manifest (missing required fields)', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse({ formatVersion: 1, name: 'incomplete' } as Partial<StylePackageManifest>)),
    ) as unknown as typeof fetch;

    await expect(loadStylePackage(BASE_URL)).rejects.toThrow(/Malformed style-package manifest/);
  });

  test('throws on an unsupported manifest formatVersion', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ formatVersion: 2 }))),
    ) as unknown as typeof fetch;

    await expect(loadStylePackage(BASE_URL)).rejects.toThrow(/Unsupported style-package manifest formatVersion/);
  });

  test('a MAJOR contract mismatch loads with a warning by default (non-strict)', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ targetContractVersion: '^2.0.0' }))),
    ) as unknown as typeof fetch;

    const mismatches: string[] = [];
    const result = await loadStylePackage(BASE_URL, {
      onContractMismatch: (info) => mismatches.push(`${info.packageName}@${info.packageVersion}`),
    });

    expect(result.status).toBe('loaded');
    expect(result.contractCompatible).toBe(false);
    expect(activeLinks()).toHaveLength(1);
    expect(mismatches).toEqual(['liquid-labs@1.0.0']);
  });

  test('strict mode refuses to load on a MAJOR contract mismatch, falling through to defaults', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ targetContractVersion: '^2.0.0' }))),
    ) as unknown as typeof fetch;

    const result = await loadStylePackage(BASE_URL, { strict: true });

    expect(result.status).toBe('skipped-strict-mismatch');
    expect(result.contractCompatible).toBe(false);
    expect(result.bundleUrl).toBeUndefined();
    expect(result.linkElement).toBeUndefined();
    expect(activeLinks()).toHaveLength(0);
    expect(getActiveStylePackage()?.status).toBe('skipped-strict-mismatch');
  });

  test('strict mode removes a previously-loaded package if a subsequent load is MAJOR-mismatched', async () => {
    global.fetch = mock(() => Promise.resolve(manifestResponse(baseManifest()))) as unknown as typeof fetch;
    await loadStylePackage(BASE_URL);
    expect(activeLinks()).toHaveLength(1);

    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ targetContractVersion: '^2.0.0' }))),
    ) as unknown as typeof fetch;
    const result = await loadStylePackage(BASE_URL, { strict: true });

    expect(result.status).toBe('skipped-strict-mismatch');
    expect(activeLinks()).toHaveLength(0);
  });

  test('MINOR/PATCH-satisfied ranges load silently (no mismatch callback invoked)', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ targetContractVersion: `^${MF_TOKEN_CONTRACT_VERSION}` }))),
    ) as unknown as typeof fetch;

    const mismatches: string[] = [];
    const result = await loadStylePackage(BASE_URL, { onContractMismatch: () => mismatches.push('called') });

    expect(result.contractCompatible).toBe(true);
    expect(mismatches).toEqual([]);
  });

  test('rejects a cross-origin styleBundle URL by default (same-origin pinning)', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ styleBundle: 'https://evil.example.com/style.css' }))),
    ) as unknown as typeof fetch;

    await expect(loadStylePackage(BASE_URL)).rejects.toThrow(/Malformed style-package manifest/);
    expect(activeLinks()).toHaveLength(0);
  });

  test('rejects a cross-origin asset URL (logo) by default (same-origin pinning)', async () => {
    global.fetch = mock(() =>
      Promise.resolve(
        manifestResponse(
          baseManifest({
            assets: { logos: { mark: 'https://evil.example.com/logo.svg' } },
          }),
        ),
      ),
    ) as unknown as typeof fetch;

    await expect(loadStylePackage(BASE_URL)).rejects.toThrow(/Malformed style-package manifest/);
  });

  test('allowCrossOriginAssets: true permits a cross-origin styleBundle URL', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ styleBundle: 'https://cdn.other-example.com/style.css' }))),
    ) as unknown as typeof fetch;

    const result = await loadStylePackage(BASE_URL, { allowCrossOriginAssets: true });

    expect(result.status).toBe('loaded');
    expect(result.bundleUrl).toBe('https://cdn.other-example.com/style.css?mf-style-version=1.0.0');
    expect(activeLinks()).toHaveLength(1);
  });

  test('allowCrossOriginAssets: true permits cross-origin asset (logo/font) URLs', async () => {
    global.fetch = mock(() =>
      Promise.resolve(
        manifestResponse(
          baseManifest({
            assets: {
              logos: { mark: 'https://cdn.other-example.com/logo.svg' },
              fonts: [{ family: 'Liquid Sans', src: ['https://cdn.other-example.com/liquid-sans.woff2'] }],
            },
          }),
        ),
      ),
    ) as unknown as typeof fetch;

    const result = await loadStylePackage(BASE_URL, { allowCrossOriginAssets: true });

    expect(result.assets?.logos?.mark).toBe('https://cdn.other-example.com/logo.svg');
    expect(result.assets?.fonts?.[0].src).toEqual(['https://cdn.other-example.com/liquid-sans.woff2']);
  });

  test('throws on a malformed assets.logos value (neither a string nor a light/dark object)', async () => {
    global.fetch = mock(() =>
      Promise.resolve(
        manifestResponse({
          ...baseManifest(),
          assets: { logos: { mark: 42 } },
        } as unknown as Partial<StylePackageManifest>),
      ),
    ) as unknown as typeof fetch;

    await expect(loadStylePackage(BASE_URL)).rejects.toThrow(/Malformed style-package manifest/);
  });

  test('throws on a malformed assets.fonts entry (missing src)', async () => {
    global.fetch = mock(() =>
      Promise.resolve(
        manifestResponse({
          ...baseManifest(),
          assets: { fonts: [{ family: 'Liquid Sans' }] },
        } as unknown as Partial<StylePackageManifest>),
      ),
    ) as unknown as typeof fetch;

    await expect(loadStylePackage(BASE_URL)).rejects.toThrow(/Malformed style-package manifest/);
  });

  test('a superseded call resolves with status "superseded" and does not overwrite a later call\'s state', async () => {
    let resolveFirst!: (response: Response) => void;
    const firstResponse = new Promise<Response>((resolve) => {
      resolveFirst = resolve;
    });
    let callCount = 0;
    global.fetch = mock(() => {
      callCount += 1;
      if (callCount === 1) return firstResponse;
      return Promise.resolve(manifestResponse(baseManifest({ name: 'second-brand', styleBundle: './second.css' })));
    }) as unknown as typeof fetch;

    const firstCall = loadStylePackage(BASE_URL);
    const secondCall = await loadStylePackage(BASE_URL);

    expect(secondCall.status).toBe('loaded');
    expect(getActiveStylePackage()?.manifest.name).toBe('second-brand');

    resolveFirst(manifestResponse(baseManifest({ name: 'first-brand', styleBundle: './first.css' })));
    const firstResult = await firstCall;

    expect(firstResult.status).toBe('superseded');
    expect(activeLinks()).toHaveLength(1);
    expect(getActiveStylePackage()?.manifest.name).toBe('second-brand');
  });
});

describe('setThemeMode', () => {
  afterEach(() => {
    document.documentElement.removeAttribute('data-mf-theme');
  });

  test('sets data-mf-theme on the document root by default', () => {
    setThemeMode('dark');
    expect(document.documentElement.getAttribute('data-mf-theme')).toBe('dark');
  });

  test('sets data-mf-theme on an explicit target element without touching the document root', () => {
    const subtree = document.createElement('div');
    document.body.appendChild(subtree);

    setThemeMode('inverse', subtree);

    expect(subtree.getAttribute('data-mf-theme')).toBe('inverse');
    expect(document.documentElement.getAttribute('data-mf-theme')).toBeNull();

    subtree.remove();
  });

  test('supports all three CONTRACT.md data-mf-theme values', () => {
    for (const mode of ['light', 'dark', 'inverse'] as const) {
      setThemeMode(mode);
      expect(document.documentElement.getAttribute('data-mf-theme')).toBe(mode);
    }
  });
});
