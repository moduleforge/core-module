// Minimal semver-range satisfaction check, purpose-built for
// `theme-loader.ts`'s one job: does a style package's declared
// `targetContractVersion` range (e.g. `"^1.0.0"`) admit the runtime
// `MF_TOKEN_CONTRACT_VERSION`? `tokens/STYLE-PACKAGE-CONTRACT.md` only ever
// expects the caret/tilde/comparator range forms below in practice — pulling
// in a full `semver`-package dependency for that narrow, first-party check
// is not warranted (per node-design-standards' "reach for the standard
// library first; add a dependency only when the task genuinely warrants
// it"). Not exported from the package root — internal to the loader.

interface SemverVersion {
  major: number;
  minor: number;
  patch: number;
}

const VERSION_RE = /^(\d+)\.(\d+)\.(\d+)/;

/**
 * Defensive cap on a range string's length, checked before any parsing/
 * iteration. No legitimate `targetContractVersion` range (a handful of
 * comparator clauses) approaches this length; this only guards against an
 * oversized/misbehaving manifest field being iterated in full.
 */
const MAX_RANGE_LENGTH = 256;

function parseVersion(input: string): SemverVersion {
  const match = VERSION_RE.exec(input.trim());
  if (!match) {
    throw new Error(`Not a valid semver version: "${input}"`);
  }
  return { major: Number(match[1]), minor: Number(match[2]), patch: Number(match[3]) };
}

function compareVersions(a: SemverVersion, b: SemverVersion): number {
  if (a.major !== b.major) return a.major - b.major;
  if (a.minor !== b.minor) return a.minor - b.minor;
  return a.patch - b.patch;
}

type ComparatorOp = '>=' | '<=' | '>' | '<' | '=';

interface Comparator {
  op: ComparatorOp;
  version: SemverVersion;
}

const COMPARATOR_RE = /^(>=|<=|>|<|=)?\s*(\d+\.\d+\.\d+\S*)$/;

/** `^1.2.3 := >=1.2.3 <2.0.0`; `^0.2.3 := >=0.2.3 <0.3.0`; `^0.0.3 := >=0.0.3 <0.0.4` (standard caret semantics — a leading `0` major/minor narrows the allowed range). */
function caretRangeToComparators(version: SemverVersion): Comparator[] {
  let upper: SemverVersion;
  if (version.major > 0) {
    upper = { major: version.major + 1, minor: 0, patch: 0 };
  } else if (version.minor > 0) {
    upper = { major: 0, minor: version.minor + 1, patch: 0 };
  } else {
    upper = { major: 0, minor: 0, patch: version.patch + 1 };
  }
  return [
    { op: '>=', version },
    { op: '<', version: upper },
  ];
}

/** `~1.2.3 := >=1.2.3 <1.3.0` — allows patch-level changes only. */
function tildeRangeToComparators(version: SemverVersion): Comparator[] {
  const upper: SemverVersion = { major: version.major, minor: version.minor + 1, patch: 0 };
  return [
    { op: '>=', version },
    { op: '<', version: upper },
  ];
}

function parseComparatorClause(clause: string): Comparator[] {
  const trimmed = clause.trim();
  if (trimmed === '' || trimmed === '*') return [];
  if (trimmed.startsWith('^')) return caretRangeToComparators(parseVersion(trimmed.slice(1)));
  if (trimmed.startsWith('~')) return tildeRangeToComparators(parseVersion(trimmed.slice(1)));
  const match = COMPARATOR_RE.exec(trimmed);
  if (!match) {
    throw new Error(`Unsupported semver range clause: "${clause}"`);
  }
  const op = (match[1] ?? '=') as ComparatorOp;
  return [{ op, version: parseVersion(match[2]) }];
}

/**
 * Whether `version` satisfies `range`. Supports exact versions (bare
 * `"1.2.3"` is an exact-match clause, matching npm's bare-range semantics),
 * `^`/`~` shorthand, and `>=`/`<=`/`>`/`<`/`=` comparators, space-separated
 * as an AND set (no `||` OR-set support — not a form
 * `STYLE-PACKAGE-CONTRACT.md`'s `targetContractVersion` field uses).
 *
 * Throws if `range` or `version` cannot be parsed; callers that want an
 * incompatibility signal rather than a thrown error on a malformed range
 * should catch and treat that as "not satisfied" (this is exactly what
 * `theme-loader.ts`'s `isContractCompatible` does).
 */
export function satisfiesRange(version: string, range: string): boolean {
  if (range.length > MAX_RANGE_LENGTH) {
    throw new Error(`Semver range exceeds the ${MAX_RANGE_LENGTH}-character defensive cap: "${range.slice(0, 40)}…"`);
  }
  const target = parseVersion(version);
  const comparators = range
    .split(/\s+/)
    .filter(Boolean)
    .flatMap(parseComparatorClause);

  return comparators.every(({ op, version: bound }) => {
    const cmp = compareVersions(target, bound);
    switch (op) {
      case '>=':
        return cmp >= 0;
      case '<=':
        return cmp <= 0;
      case '>':
        return cmp > 0;
      case '<':
        return cmp < 0;
      case '=':
        return cmp === 0;
    }
  });
}
