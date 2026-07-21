import * as React from 'react';
import { loadStylePackage, type LoadStylePackageOptions, type LoadStylePackageResult } from './theme-loader';

/** `useStylePackage`'s lifecycle status. `'idle'` covers both "not yet started" and `source` being `null`/`undefined` (no package configured — mod-core defaults render, per the loader's non-required-by-default contract). */
export type UseStylePackageStatus = 'idle' | 'loading' | 'loaded' | 'skipped-strict-mismatch' | 'error';

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
 * hook does nothing (status stays `'idle'`) and mod-core's baked defaults
 * render, exactly as if the hook were never called.
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

  React.useEffect(() => {
    if (!source) {
      setState({ status: 'idle', result: null, error: null });
      return;
    }

    let cancelled = false;
    setState({ status: 'loading', result: null, error: null });

    loadStylePackage(source, optionsRef.current)
      .then((result) => {
        if (cancelled) return;
        setState({
          status: result.status === 'loaded' ? 'loaded' : 'skipped-strict-mismatch',
          result,
          error: null,
        });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState({ status: 'error', result: null, error: err instanceof Error ? err : new Error(String(err)) });
      });

    return () => {
      cancelled = true;
    };
    // `optionsRef` intentionally excluded from the dependency array — see the doc-comment above.
  }, [source]);

  return state;
}
