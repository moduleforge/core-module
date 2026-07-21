import { describe, expect, test } from 'bun:test';
import { satisfiesRange } from './semver-range';

describe('satisfiesRange', () => {
  test('caret range admits any version with the same major (and, for major 0, matching minor)', () => {
    expect(satisfiesRange('1.0.0', '^1.0.0')).toBe(true);
    expect(satisfiesRange('1.4.2', '^1.0.0')).toBe(true);
    expect(satisfiesRange('1.99.99', '^1.0.0')).toBe(true);
    expect(satisfiesRange('2.0.0', '^1.0.0')).toBe(false);
    expect(satisfiesRange('0.9.9', '^1.0.0')).toBe(false);
  });

  test('caret range on a 0.x.y version narrows to matching minor (standard caret semantics)', () => {
    expect(satisfiesRange('0.2.3', '^0.2.0')).toBe(true);
    expect(satisfiesRange('0.3.0', '^0.2.0')).toBe(false);
  });

  test('tilde range admits patch-level changes only', () => {
    expect(satisfiesRange('1.2.3', '~1.2.0')).toBe(true);
    expect(satisfiesRange('1.2.99', '~1.2.0')).toBe(true);
    expect(satisfiesRange('1.3.0', '~1.2.0')).toBe(false);
  });

  test('a bare version range is an exact-match clause', () => {
    expect(satisfiesRange('1.0.0', '1.0.0')).toBe(true);
    expect(satisfiesRange('1.0.1', '1.0.0')).toBe(false);
  });

  test('comparator operators combine as an AND set across whitespace-separated clauses', () => {
    expect(satisfiesRange('1.5.0', '>=1.0.0 <2.0.0')).toBe(true);
    expect(satisfiesRange('2.0.0', '>=1.0.0 <2.0.0')).toBe(false);
    expect(satisfiesRange('0.9.9', '>=1.0.0 <2.0.0')).toBe(false);
  });

  test('"*" and "" ranges admit any version', () => {
    expect(satisfiesRange('1.0.0', '*')).toBe(true);
    expect(satisfiesRange('9.9.9', '')).toBe(true);
  });

  test('throws on an unparseable range clause', () => {
    expect(() => satisfiesRange('1.0.0', 'not-a-range')).toThrow();
  });

  test('throws on an unparseable target version', () => {
    expect(() => satisfiesRange('not-a-version', '^1.0.0')).toThrow();
  });
});
