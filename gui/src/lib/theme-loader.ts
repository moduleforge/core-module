// Runtime theme-loader utility (Phase 3 task 002, `gui-design-tokens` plan).
//
// Lets an app shell load an independently-published **style package** — a
// brand's compiled `--mf-*` override bundle + brand-asset manifest, per
// `../../tokens/STYLE-PACKAGE-CONTRACT.md` — at runtime, without rebuilding
// the app or `mod-core`, by injecting a versioned `<link rel="stylesheet">`
// into `<head>`. This is the v1 loading mechanism (plan-branch
// `runtime-theming.md`); JSON-manifest `setProperty()` instant-switching and
// Module Federation are explicitly out of scope here (future upgrade path
// only).
//
// The loader is framework-agnostic — no Next.js (or any framework) coupling.
// `./use-style-package.ts` provides a thin, optional React hook wrapper for
// callers that want it; it is never required.
//
// Default/fallback case: with no style package loaded, mod-core's compiled
// `--mf-x-default` set already renders a complete, working UI (per
// `tokens/CONTRACT.md`'s fallback-chaining contract) — this loader is purely
// additive and is never required for a working default UI.
//
// FOUC/caching: the bundle URL is versioned with the package's own semver
// (`?mf-style-version=<version>`), giving callers an immutable-cacheable,
// cache-busting-on-swap URL. `swapStyleLink` appends the new `<link>` before
// removing any previous one, so a brand swap never has a zero-stylesheet
// frame in `<head>`; it does not block on the new stylesheet's network load
// completing before returning (that additional guarantee — gating first
// paint on the `<link>`'s `load` event — is a documented future refinement,
// not built now: v1 accepts a brief FOUC on initial load / swap, per the
// task's explicit "do not over-engineer" allowance). The recommended
// injection timing for an app shell is to call `loadStylePackage` as early
// as possible in `<head>` (e.g. a root layout/server component that resolves
// the active style package before the first client paint, or a blocking
// `<script>`/effect gate on the returned promise if the app wants to avoid
// any flash at the cost of a slower first paint) — the loader itself does
// not gate rendering; that policy choice belongs to the app shell.

import { MF_TOKEN_CONTRACT_VERSION } from './token-contract-version';
import { satisfiesRange } from './semver-range';

// ─── Manifest / asset types ─────────────────────────────────────────────────
// Mirrors the JSON Schema in `../../tokens/STYLE-PACKAGE-CONTRACT.md` exactly
// ("Brand-asset manifest" section).

/** Per-mode logo variant, keyed to align with `data-mf-theme` light/dark. */
export interface StylePackageLogoVariants {
  light?: string;
  dark?: string;
}

/** An `@font-face` descriptor for a family the override bundle points a `--mf-font-*` lever at. */
export interface StylePackageFontDescriptor {
  family: string;
  /** Font file URLs, relative to the manifest, in `@font-face` `src` order. */
  src: string[];
  weight?: string;
  style?: 'normal' | 'italic' | 'oblique';
  display?: 'auto' | 'block' | 'swap' | 'fallback' | 'optional';
}

/** Brand assets (logos/fonts) a style package supplies alongside its token overrides. */
export interface StylePackageAssets {
  /** Logo roles ("mark", "wordmark", ... — brand-defined) mapped to a single URL or a light/dark variant map. */
  logos?: Record<string, string | StylePackageLogoVariants>;
  /** `@font-face` descriptors for families the override bundle names. */
  fonts?: StylePackageFontDescriptor[];
}

/** The `style-package.json` brand-asset manifest — the loader's fetch entry point. */
export interface StylePackageManifest {
  /** Manifest schema version; the loader only understands `1`. */
  formatVersion: number;
  /** Style-package identifier, e.g. `"liquid-labs"`. */
  name: string;
  /** The style package's own semver, independent of the token contract. */
  version: string;
  /** Semver RANGE of the mod-core token contract this package targets, e.g. `"^1.0.0"`. */
  targetContractVersion: string;
  /** URL of the compiled sparse `--mf-*` override bundle, relative to the manifest. */
  styleBundle: string;
  assets?: StylePackageAssets;
}

const SUPPORTED_MANIFEST_FORMAT_VERSION = 1;
const MANIFEST_FILENAME = 'style-package.json';
const STYLE_LINK_MARKER = 'data-mf-style-package';
const CACHE_BUST_PARAM = 'mf-style-version';
/**
 * Defensive cap on a fetched manifest's raw response body, applied before
 * `JSON.parse`. No legitimate `style-package.json` (four short string
 * fields + a small assets block) approaches this size; this only guards
 * against an oversized/misbehaving response being parsed in full.
 */
const MAX_MANIFEST_BODY_LENGTH = 16 * 1024;

// ─── Public option / result types ───────────────────────────────────────────

/** A developer-facing signal that a loaded package's `targetContractVersion` does not admit the runtime `MF_TOKEN_CONTRACT_VERSION`. */
export interface ContractMismatchInfo {
  packageName: string;
  packageVersion: string;
  /** The package's declared range, e.g. `"^1.0.0"`. */
  targetContractVersion: string;
  /** The runtime `MF_TOKEN_CONTRACT_VERSION` the range was checked against. */
  runtimeContractVersion: string;
}

export interface LoadStylePackageOptions {
  /**
   * Base location the manifest's relative `styleBundle`/asset URLs resolve
   * against. Required when `source` is a pre-fetched manifest object (there
   * is no fetch to derive it from); ignored when `source` is a base-URL
   * string (the loader derives it from the fetched manifest's own URL).
   */
  baseUrl?: string;
  /** `Document` to inject into; defaults to the global `document` (browser). */
  doc?: Document;
  /**
   * Opt-in "Optional strict mode" from `STYLE-PACKAGE-CONTRACT.md`'s mismatch
   * policy: on a MAJOR contract mismatch, refuse to load and fall through to
   * mod-core's baked defaults instead of loading with a warning. Off by
   * default — the default policy always loads (graceful fallback makes every
   * combination render; see the contract doc's version-bump table).
   */
  strict?: boolean;
  /**
   * Called instead of the default `console.warn` on a MAJOR contract
   * mismatch — e.g. to route the signal to a production telemetry/log hook.
   * Called whether or not `strict` is set (strict mode still reports the
   * mismatch it acted on).
   *
   * The `info` passed here carries manifest-supplied strings
   * (`packageName`/`packageVersion`/`targetContractVersion`) that are not
   * validated for length or content beyond "is a string" — a production
   * implementation that persists/logs these fields should bound and/or
   * escape them first, the same way it would any other untrusted input.
   */
  onContractMismatch?: (info: ContractMismatchInfo) => void;
  /**
   * Skips the same-origin check the loader otherwise applies to every
   * resolved `styleBundle`/asset URL (see `STYLE-PACKAGE-CONTRACT.md`'s
   * "Security posture"). **Off by default**: a style package's assets must
   * resolve to the same origin as its own manifest, since an absolute URL
   * in an independently-hosted (and possibly compromised or spoofed)
   * manifest could otherwise silently point the injected stylesheet or
   * brand assets at an arbitrary third-party origin. Set this to `true`
   * only for a known-legitimate cross-origin hosting setup (e.g. serving
   * the compiled bundle from a CDN origin distinct from the manifest's).
   */
  allowCrossOriginAssets?: boolean;
}

/**
 * Result of `loadStylePackage`. `status` is `'loaded'` for every case that
 * injects the bundle (the default policy, including a non-strict MAJOR
 * mismatch — loaded with a warning); `'skipped-strict-mismatch'` only when
 * `options.strict` refused a MAJOR-mismatched package, leaving mod-core's
 * defaults active; and `'superseded'` when a later `loadStylePackage` call
 * for the same `doc` had already started before this call's async work
 * (fetch/resolution) completed — this call's outcome is discarded without
 * touching the DOM or `getActiveStylePackage`'s tracked state, leaving the
 * later call's result in effect. `assets`/`bundleUrl`/`linkElement` are
 * populated only in the `'loaded'` case.
 */
export interface LoadStylePackageResult {
  status: 'loaded' | 'skipped-strict-mismatch' | 'superseded';
  /** The manifest exactly as fetched/provided (relative URLs unresolved). */
  manifest: StylePackageManifest;
  /** `manifest.assets` with every relative URL resolved to absolute, ready for the app shell to render directly. `undefined` when the manifest declares no assets, or the load was skipped. */
  assets?: StylePackageAssets;
  /** Absolute, cache-busted URL of the injected `<link>`'s `href`. Undefined when skipped. */
  bundleUrl?: string;
  /** The `<link>` element the loader injected. Undefined when skipped. */
  linkElement?: HTMLLinkElement;
  /** Whether `manifest.targetContractVersion` admits `MF_TOKEN_CONTRACT_VERSION`. */
  contractCompatible: boolean;
}

// Tracks the last load/unload outcome per `Document`, for `getActiveStylePackage`'s
// accessor use — the alternative the task doc allows to returning the result
// from `loadStylePackage` directly. A `WeakMap` (rather than a module-level
// singleton) supports more than one `Document` in the same JS realm (e.g.
// concurrent tests, or a multi-frame host), without leaking references once a
// `Document` is discarded.
const activeStylePackages = new WeakMap<Document, LoadStylePackageResult | null>();

// Monotonic sequencing guard against overlapping concurrent `loadStylePackage`
// calls for the same `doc`: an earlier call whose fetch/resolution resolves
// later than a subsequently-started call could otherwise overwrite the later
// call's DOM/state. Each call captures its own token at start; only the call
// holding the latest token for its `doc` is allowed to perform the DOM swap /
// update `activeStylePackages`. A per-`Document` (rather than a single global)
// latest-token map lets independent `doc`s load concurrently without
// superseding each other.
let loadSequenceCounter = 0;
const latestLoadSequenceByDoc = new WeakMap<Document, number>();

// ─── Manifest fetch + validation ────────────────────────────────────────────

function ensureTrailingSlash(url: string): string {
  return url.endsWith('/') ? url : `${url}/`;
}

function assertManifestShape(manifest: unknown, source: string): asserts manifest is StylePackageManifest {
  if (manifest === null || typeof manifest !== 'object') {
    throw new Error(`Malformed style-package manifest (${source}): expected a JSON object.`);
  }
  const candidate = manifest as Partial<StylePackageManifest>;
  if (
    typeof candidate.name !== 'string' ||
    typeof candidate.version !== 'string' ||
    typeof candidate.targetContractVersion !== 'string' ||
    typeof candidate.styleBundle !== 'string'
  ) {
    throw new Error(
      `Malformed style-package manifest (${source}): missing one or more required fields ` +
        `(name, version, targetContractVersion, styleBundle).`,
    );
  }
  if (candidate.formatVersion !== SUPPORTED_MANIFEST_FORMAT_VERSION) {
    throw new Error(
      `Unsupported style-package manifest formatVersion ${String(candidate.formatVersion)} (${source}); ` +
        `this loader understands formatVersion ${SUPPORTED_MANIFEST_FORMAT_VERSION} only.`,
    );
  }
  assertAssetsShape(candidate.assets, source);
}

/** Type guard for a single `assets.logos` value: a bare URL string, or a `{ light?, dark? }` variant map of URL strings. */
function isValidLogoValue(value: unknown): boolean {
  if (typeof value === 'string') return true;
  if (value === null || typeof value !== 'object') return false;
  const variants = value as { light?: unknown; dark?: unknown };
  return (
    (variants.light === undefined || typeof variants.light === 'string') &&
    (variants.dark === undefined || typeof variants.dark === 'string')
  );
}

/** Type guard for a single `assets.fonts` entry: a string `family` and a `string[]` `src`. */
function isValidFontDescriptor(font: unknown): boolean {
  if (font === null || typeof font !== 'object') return false;
  const candidate = font as { family?: unknown; src?: unknown };
  return (
    typeof candidate.family === 'string' &&
    Array.isArray(candidate.src) &&
    candidate.src.every((src) => typeof src === 'string')
  );
}

/**
 * Validates the optional `assets` sub-shape (logos/fonts) against
 * `STYLE-PACKAGE-CONTRACT.md`'s schema. `assertManifestShape` only checked
 * the four required top-level string fields and `formatVersion`; without
 * this, a malformed `assets` block (e.g. a logo value that is neither a
 * string nor a `{light,dark}` object, or a font entry missing `src`) would
 * reach `resolveAssets` and throw a raw, uncaught `TypeError` instead of the
 * documented "Malformed style-package manifest" error.
 */
function assertAssetsShape(assets: unknown, source: string): void {
  if (assets === undefined) return;
  if (assets === null || typeof assets !== 'object') {
    throw new Error(`Malformed style-package manifest (${source}): "assets" must be an object when present.`);
  }
  const candidate = assets as { logos?: unknown; fonts?: unknown };
  if (candidate.logos !== undefined) {
    if (candidate.logos === null || typeof candidate.logos !== 'object') {
      throw new Error(`Malformed style-package manifest (${source}): "assets.logos" must be an object when present.`);
    }
    for (const [role, value] of Object.entries(candidate.logos as Record<string, unknown>)) {
      if (!isValidLogoValue(value)) {
        throw new Error(
          `Malformed style-package manifest (${source}): "assets.logos.${role}" must be a string URL or a ` +
            `{ light?, dark? } object of string URLs.`,
        );
      }
    }
  }
  if (candidate.fonts !== undefined) {
    if (!Array.isArray(candidate.fonts)) {
      throw new Error(`Malformed style-package manifest (${source}): "assets.fonts" must be an array when present.`);
    }
    candidate.fonts.forEach((font, index) => {
      if (!isValidFontDescriptor(font)) {
        throw new Error(
          `Malformed style-package manifest (${source}): "assets.fonts[${index}]" must have a string "family" ` +
            `and a string[] "src".`,
        );
      }
    });
  }
}

async function fetchManifest(baseUrl: string): Promise<{ manifest: StylePackageManifest; manifestUrl: string }> {
  const manifestUrl = new URL(MANIFEST_FILENAME, ensureTrailingSlash(baseUrl)).toString();
  let response: Response;
  try {
    response = await fetch(manifestUrl);
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Network request failed';
    throw new Error(`Failed to fetch style-package manifest at ${manifestUrl}: ${message}`);
  }
  if (!response.ok) {
    throw new Error(`Failed to fetch style-package manifest at ${manifestUrl}: HTTP ${response.status}`);
  }
  const body = await response.text();
  if (body.length > MAX_MANIFEST_BODY_LENGTH) {
    throw new Error(
      `Malformed style-package manifest (${manifestUrl}): response body exceeds the ` +
        `${MAX_MANIFEST_BODY_LENGTH}-byte defensive cap for a style-package manifest.`,
    );
  }
  let manifest: unknown;
  try {
    manifest = JSON.parse(body);
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Invalid JSON';
    throw new Error(`Malformed style-package manifest (${manifestUrl}): failed to parse JSON body: ${message}`);
  }
  assertManifestShape(manifest, manifestUrl);
  return { manifest, manifestUrl };
}

/**
 * Enforces the same-origin asset-pinning policy: `resolvedUrl` must share
 * `manifestUrl`'s origin unless the caller opted out via
 * `options.allowCrossOriginAssets`. Without this, an absolute URL in a
 * manifest (compromised, spoofed, or simply misconfigured) silently wins
 * over `manifestUrl`'s origin, letting the loader inject a `<link>`/asset
 * pointed at an arbitrary third-party origin.
 */
function assertSameOrigin(resolvedUrl: string, manifestUrl: string, context: string): void {
  const resolvedOrigin = new URL(resolvedUrl).origin;
  const manifestOrigin = new URL(manifestUrl).origin;
  if (resolvedOrigin !== manifestOrigin) {
    throw new Error(
      `Malformed style-package manifest (${manifestUrl}): "${context}" resolves to origin "${resolvedOrigin}", ` +
        `which does not match the manifest's own origin "${manifestOrigin}". A style package's assets must be ` +
        `same-origin with its manifest by default, since an independently-hosted/untrusted manifest could ` +
        `otherwise point the injected stylesheet or brand assets at an arbitrary third-party origin. Pass ` +
        `{ allowCrossOriginAssets: true } to loadStylePackage to permit intentional cross-origin hosting ` +
        `(e.g. a CDN origin distinct from the manifest's own).`,
    );
  }
}

function resolveAssetUrl(
  value: string,
  manifestUrl: string,
  allowCrossOriginAssets: boolean,
  context: string,
): string {
  const resolved = new URL(value, manifestUrl).toString();
  if (!allowCrossOriginAssets) {
    assertSameOrigin(resolved, manifestUrl, context);
  }
  return resolved;
}

function resolveAssets(
  assets: StylePackageAssets | undefined,
  manifestUrl: string,
  allowCrossOriginAssets: boolean,
): StylePackageAssets | undefined {
  if (!assets) return undefined;
  const resolved: StylePackageAssets = {};
  if (assets.logos) {
    resolved.logos = Object.fromEntries(
      Object.entries(assets.logos).map(([role, value]) => [
        role,
        typeof value === 'string'
          ? resolveAssetUrl(value, manifestUrl, allowCrossOriginAssets, `assets.logos.${role}`)
          : {
              light: value.light
                ? resolveAssetUrl(value.light, manifestUrl, allowCrossOriginAssets, `assets.logos.${role}.light`)
                : undefined,
              dark: value.dark
                ? resolveAssetUrl(value.dark, manifestUrl, allowCrossOriginAssets, `assets.logos.${role}.dark`)
                : undefined,
            },
      ]),
    );
  }
  if (assets.fonts) {
    resolved.fonts = assets.fonts.map((font, fontIndex) => ({
      ...font,
      src: font.src.map((src, srcIndex) =>
        resolveAssetUrl(src, manifestUrl, allowCrossOriginAssets, `assets.fonts[${fontIndex}].src[${srcIndex}]`),
      ),
    }));
  }
  return resolved;
}

// ─── Contract-version mismatch policy ───────────────────────────────────────
// Implements `STYLE-PACKAGE-CONTRACT.md`'s "Token-contract versioning" /
// "Mismatch policy" exactly: load, always, by default; warn (don't block) on
// a MAJOR mismatch; optional strict mode refuses to load on MAJOR mismatch.

function isContractCompatible(targetContractVersion: string): boolean {
  try {
    return satisfiesRange(MF_TOKEN_CONTRACT_VERSION, targetContractVersion);
  } catch {
    // An unparseable range is itself a mismatch signal, not a crash to
    // propagate — surfaces through the same warning path as a real mismatch.
    return false;
  }
}

function reportContractMismatch(manifest: StylePackageManifest, options: LoadStylePackageOptions): void {
  const info: ContractMismatchInfo = {
    packageName: manifest.name,
    packageVersion: manifest.version,
    targetContractVersion: manifest.targetContractVersion,
    runtimeContractVersion: MF_TOKEN_CONTRACT_VERSION,
  };
  if (options.onContractMismatch) {
    options.onContractMismatch(info);
    return;
  }
  console.warn(
    `[mf-style-package] "${info.packageName}@${info.packageVersion}" targets token contract ` +
      `"${info.targetContractVersion}", which does not admit the runtime contract version ` +
      `"${info.runtimeContractVersion}". Every affected token gracefully falls back to mod-core's ` +
      `default (per tokens/CONTRACT.md's fallback-chaining contract); rebuild the style package ` +
      `against the current contract when convenient.`,
  );
}

// ─── DOM injection ───────────────────────────────────────────────────────────

function withCacheBustVersion(url: string, version: string): string {
  const versioned = new URL(url);
  versioned.searchParams.set(CACHE_BUST_PARAM, version);
  return versioned.toString();
}

/**
 * Injects a new style-package `<link rel="stylesheet">` and removes any
 * previously-injected one. Appends before removing so a swap never leaves a
 * paint frame with zero style-package link in `<head>` (see the module
 * doc-comment's FOUC note for what this does and does not guarantee).
 *
 * Prefers the previous link element already held in `activeStylePackages`
 * (an O(1) direct removal) over scanning `<head>`; the `querySelectorAll`
 * scan is kept only as a defensive fallback for when no tracked reference
 * exists (e.g. the first load for a `doc` the loader has not tracked before).
 */
function swapStyleLink(doc: Document, href: string): HTMLLinkElement {
  const link = doc.createElement('link');
  link.rel = 'stylesheet';
  link.href = href;
  link.setAttribute(STYLE_LINK_MARKER, 'true');
  doc.head.appendChild(link);

  const previousLink = activeStylePackages.get(doc)?.linkElement;
  if (previousLink) {
    previousLink.remove();
  } else {
    Array.from(doc.head.querySelectorAll(`link[${STYLE_LINK_MARKER}="true"]`))
      .filter((el) => el !== link)
      .forEach((el) => el.remove());
  }

  return link;
}

// ─── Public API ──────────────────────────────────────────────────────────────

/**
 * Loads (or swaps to) a style package. `source` is either the style
 * package's **base location** (a URL the loader fetches
 * `<base>/style-package.json` from, per `STYLE-PACKAGE-CONTRACT.md`'s
 * "Brand-asset manifest" section — the manifest is the loader's entry
 * point) or an already-fetched `StylePackageManifest` object (for a caller
 * that fetched it itself, e.g. server-side); `options.baseUrl` is then
 * required to resolve the manifest's relative URLs.
 *
 * Applies the token-contract mismatch policy from
 * `STYLE-PACKAGE-CONTRACT.md` before injecting: loads unconditionally by
 * default (warning on a MAJOR mismatch via `options.onContractMismatch` or
 * `console.warn`); with `options.strict`, refuses to load a MAJOR-mismatched
 * package and falls through to mod-core's defaults instead (removing any
 * previously-active style-package link).
 *
 * Does not itself touch `data-mf-theme` — that attribute encodes
 * light/dark/inverse *mode*, orthogonal to *which brand* is loaded (a brand
 * scopes its own color overrides under the same attribute's selectors; see
 * `tokens/CONTRACT.md`'s "Runtime brand selection" case). Callers drive mode
 * with `setThemeMode` below; this keeps the one unified scoping mechanism
 * doing both jobs without a second attribute.
 *
 * Guards against overlapping concurrent calls for the same `doc`: each call
 * captures a monotonic sequence token at start, and only performs its DOM
 * swap / updates `getActiveStylePackage`'s tracked state if no later call
 * has since started for that `doc` — otherwise it resolves with
 * `status: 'superseded'` and leaves the later call's outcome untouched. This
 * prevents an earlier call that resolves later (e.g. a slower network
 * response) from silently overwriting a subsequent call's result.
 */
export async function loadStylePackage(
  source: string | StylePackageManifest,
  options: LoadStylePackageOptions = {},
): Promise<LoadStylePackageResult> {
  const doc = options.doc ?? document;
  const sequence = ++loadSequenceCounter;
  latestLoadSequenceByDoc.set(doc, sequence);
  const isLatestCall = (): boolean => latestLoadSequenceByDoc.get(doc) === sequence;

  let manifest: StylePackageManifest;
  let manifestUrl: string;
  if (typeof source === 'string') {
    ({ manifest, manifestUrl } = await fetchManifest(source));
  } else {
    assertManifestShape(source, '<provided manifest>');
    if (!options.baseUrl) {
      throw new Error(
        'loadStylePackage: options.baseUrl is required when `source` is a pre-fetched manifest object ' +
          '(used to resolve its relative styleBundle/asset URLs).',
      );
    }
    manifest = source;
    manifestUrl = new URL(MANIFEST_FILENAME, ensureTrailingSlash(options.baseUrl)).toString();
  }

  const contractCompatible = isContractCompatible(manifest.targetContractVersion);
  if (!contractCompatible) {
    reportContractMismatch(manifest, options);
  }

  if (!contractCompatible && options.strict) {
    if (!isLatestCall()) {
      return { status: 'superseded', manifest, contractCompatible };
    }
    unloadStylePackage({ doc });
    const result: LoadStylePackageResult = { status: 'skipped-strict-mismatch', manifest, contractCompatible };
    activeStylePackages.set(doc, result);
    return result;
  }

  const allowCrossOriginAssets = options.allowCrossOriginAssets ?? false;
  const resolvedBundleUrl = new URL(manifest.styleBundle, manifestUrl).toString();
  if (!allowCrossOriginAssets) {
    assertSameOrigin(resolvedBundleUrl, manifestUrl, 'styleBundle');
  }
  const bundleUrl = withCacheBustVersion(resolvedBundleUrl, manifest.version);
  const assets = resolveAssets(manifest.assets, manifestUrl, allowCrossOriginAssets);

  if (!isLatestCall()) {
    return { status: 'superseded', manifest, contractCompatible };
  }

  const linkElement = swapStyleLink(doc, bundleUrl);

  const result: LoadStylePackageResult = {
    status: 'loaded',
    manifest,
    assets,
    bundleUrl,
    linkElement,
    contractCompatible,
  };
  activeStylePackages.set(doc, result);
  return result;
}

/**
 * Removes any currently-injected style-package `<link>`, returning the page
 * to mod-core's baked `--mf-x-default` set — the same "no package loaded"
 * default state as before any `loadStylePackage` call.
 */
export function unloadStylePackage(options: { doc?: Document } = {}): void {
  const doc = options.doc ?? document;
  doc.head.querySelectorAll(`link[${STYLE_LINK_MARKER}="true"]`).forEach((el) => el.remove());
  activeStylePackages.set(doc, null);
}

/**
 * Accessor for the last `loadStylePackage`/`unloadStylePackage` outcome for
 * `doc` — the alternative the task doc allows to holding onto
 * `loadStylePackage`'s return value directly (e.g. a component that mounts
 * after the app shell already loaded the package at startup).
 */
export function getActiveStylePackage(doc: Document = document): LoadStylePackageResult | null {
  return activeStylePackages.get(doc) ?? null;
}

/** The three `data-mf-theme` values from `tokens/CONTRACT.md`'s unified scoping attribute. */
export type MfThemeMode = 'light' | 'dark' | 'inverse';

/**
 * Sets/updates the unified `data-mf-theme` scoping attribute (`tokens/CONTRACT.md`)
 * on `target` (defaults to the document root). This is the *only* mechanism
 * the loader uses to drive light/dark/inverse mode — it never introduces a
 * second scoping attribute or class-based toggle.
 */
export function setThemeMode(mode: MfThemeMode, target: Element = document.documentElement): void {
  target.setAttribute('data-mf-theme', mode);
}
