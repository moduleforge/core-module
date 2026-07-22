import * as React from 'react';
import {
  loadStylePackage,
  unloadStylePackage,
  type LoadStylePackageOptions,
  type LoadStylePackageResult,
} from './theme-loader';

/** `useStylePackage`'s lifecycle status. `'idle'` covers both "not yet started" and `source` being `null`/`undefined` (no package configured — mod-core defaults render, per the loader's non-required-by-default contract). `'superseded'` mirrors `loadStylePackage`'s result status (see `theme-loader.ts`). */
export type UseStylePackageStatus = 'idle' | 'loading' | 'loaded' | 'skipped-strict-mismatch' | 'superseded' | 'error';

/**
 * Derives a stable string key from `source` for use as an effect dependency.
 * `source` may be a freshly-constructed manifest object each render (not
 * just a string URL); keying the effect on this string instead of the raw
 * `source` reference avoids re-triggering the load/DOM-swap effect on every
 * render when the caller passes an object literal with the same logical
 * identity.
 */
function sourceCacheKey(source: string | LoadStylePackageResult['manifest'] | null | undefined): string {
  if (!source) return '';
  return typeof source === 'string' ? source : `${source.name}@${source.version}`;
}

export interface UseStylePackageResult {
  status: UseStylePackageStatus;
  /** The `loadStylePackage` result once resolved; `null` while idle/loading/errored. */
  result: LoadStylePackageResult | null;
  /** The thrown error once `status` is `'error'`; `null` otherwise. */
  error: Error | null;
}

/**
 * Thin, optional React wrapper around `loadStylePackage` for a component
 * that wants declarative "load this style package for as long as `source`
 * names one" behavior — e.g. an app shell driven by a config value. Every
 * capability here is reachable via `loadStylePackage`/`unloadStylePackage`
 * directly; a non-React app shell (or a caller that wants finer control over
 * injection timing — see `theme-loader.ts`'s FOUC guidance) should call
 * those instead. This hook stays deliberately optional, not a dependency of
 * the framework-agnostic loader.
 *
 * `source: null | undefined` is the "no style package configured" case: the
 * hook calls `unloadStylePackage` (removing any previously-injected
 * style-package `<link>`) and the status stays/settles at `'idle'` — mod-core's
 * baked defaults render, exactly as if the hook were never called.
 *
 * `options` is read only on mount / when `source` changes — it is not a
 * reactive dependency, since it commonly carries a fresh callback identity
 * per render. A caller that needs `options.strict`/`baseUrl` to vary at
 * runtime should key that into `source` (or re-mount) rather than relying on
 * `options` identity to retrigger the effect.
 */
export function useStylePackage(
  source: string | LoadStylePackageResult['manifest'] | null | undefined,
  options: LoadStylePackageOptions = {},
): UseStylePackageResult {
  const [state, setState] = React.useState<UseStylePackageResult>({
    status: 'idle',
    result: null,
    error: null,
  });
  const optionsRef = React.useRef(options);
  optionsRef.current = options;
  const sourceKey = sourceCacheKey(source);

  React.useEffect(() => {
    if (!source) {
      unloadStylePackage(optionsRef.current);
      setState({ status: 'idle', result: null, error: null });
      return;
    }

    let cancelled = false;
    setState({ status: 'loading', result: null, error: null });

    loadStylePackage(source, optionsRef.current)
      .then((result) => {
        if (cancelled) return;
        setState({ status: result.status, result, error: null });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState({ status: 'error', result: null, error: err instanceof Error ? err : new Error(String(err)) });
      });

    return () => {
      cancelled = true;
    };
    // `optionsRef` intentionally excluded from the dependency array — see the
    // doc-comment above. `source` is intentionally replaced by `sourceKey` (a
    // stable string derived from it) here too: `source` may be a
    // freshly-constructed manifest object each render, and depending on the
    // raw reference would re-trigger this effect (and the DOM swap) on every
    // render even when the logical style package named is unchanged.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sourceKey]);

  return state;
}
