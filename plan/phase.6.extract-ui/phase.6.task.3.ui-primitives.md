# Phase 6, Task 3 — shadcn primitives

## Context
Components depend on shadcn/ui primitives. Bundle them in core-module/gui so consumer apps don't need a matching shadcn setup (user confirmed re-export approach).

## Acceptance
Copy from `users-module/gui/src/components/ui/` into `core-module/gui/src/ui/`:
- `button.tsx`
- `input.tsx`
- `label.tsx`
- `card.tsx`
- `badge.tsx`
- `alert.tsx`
- Any others actually used by ProfileEditor / entity forms (run `grep -l "from '@/components/ui/"` in the migrated components to verify).

Each primitive:
- Byte-identical content to the source.
- Imports adjusted: `@/lib/utils` → `../lib/utils` (or similar) if utilities are needed; copy the utility file(s) too if required.

Barrel `core-module/gui/src/ui/index.ts`:
```ts
export { Button, buttonVariants } from './button';
export { Input } from './input';
export { Label } from './label';
export { Card, CardContent, CardDescription, CardHeader, CardTitle } from './card';
export { Badge } from './badge';
export { Alert, AlertDescription, AlertTitle } from './alert';
```

Re-export from `core-module/gui/src/index.ts`:
```ts
export * from './ui';
export * from './ProfileEditor';
export * from './NaturalPersonForm';
export * from './CorporationForm';
export * from './ServiceAccountForm';
```

Also port `users-module/gui/src/lib/utils.ts` → `core-module/gui/src/lib/utils.ts` if the primitives depend on `cn()` (almost certainly yes).

## How to verify
- `npm run build` succeeds.
- `dist/index.js` references Button, Input, etc.
- `npm run typecheck` clean.

## Notes
- Don't duplicate primitives during consumer migration. In Phase 6.6, users-module/gui will switch its own imports from `@/components/ui/button` → `@moduleforge/core-gui` and delete the local copies.
- If a primitive uses Radix `@radix-ui/*`, add those as runtime `dependencies` in core-module/gui/package.json.
