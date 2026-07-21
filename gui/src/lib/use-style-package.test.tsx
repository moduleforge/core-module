import { afterEach, describe, expect, mock, test } from 'bun:test';
import { renderHook, waitFor } from '@testing-library/react';
import { useStylePackage } from './use-style-package';
import { unloadStylePackage, type StylePackageManifest } from './theme-loader';

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
    targetContractVersion: '^1.0.0',
    styleBundle: './style.css',
    ...overrides,
  };
}

describe('useStylePackage', () => {
  afterEach(() => {
    global.fetch = originalFetch;
    unloadStylePackage();
  });

  test('stays idle when source is null/undefined', () => {
    const { result } = renderHook(() => useStylePackage(null));
    expect(result.current.status).toBe('idle');
    expect(result.current.result).toBeNull();
  });

  test('transitions loading -> loaded and returns the loadStylePackage result', async () => {
    global.fetch = mock(() => Promise.resolve(manifestResponse(baseManifest()))) as unknown as typeof fetch;

    const { result } = renderHook(() => useStylePackage(BASE_URL));

    expect(result.current.status).toBe('loading');

    await waitFor(() => expect(result.current.status).toBe('loaded'));

    expect(result.current.result?.manifest.name).toBe('liquid-labs');
    expect(result.current.error).toBeNull();
  });

  test('transitions to error when the fetch/parse fails', async () => {
    global.fetch = mock(() =>
      Promise.resolve(new Response('not json', { status: 200 })),
    ) as unknown as typeof fetch;

    const { result } = renderHook(() => useStylePackage(BASE_URL));

    await waitFor(() => expect(result.current.status).toBe('error'));

    expect(result.current.error).toBeInstanceOf(Error);
  });

  test('transitions to skipped-strict-mismatch under strict mode with a MAJOR mismatch', async () => {
    global.fetch = mock(() =>
      Promise.resolve(manifestResponse(baseManifest({ targetContractVersion: '^2.0.0' }))),
    ) as unknown as typeof fetch;

    const { result } = renderHook(() => useStylePackage(BASE_URL, { strict: true }));

    await waitFor(() => expect(result.current.status).toBe('skipped-strict-mismatch'));

    expect(result.current.result?.status).toBe('skipped-strict-mismatch');
  });
});
