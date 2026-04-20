# Phase 6, Task 1 — Extract ProfileEditor

## Context
Current profile logic in `users-module/gui/src/app/profile/page.tsx` is a client component mixing data loading (`useAuth`), form state, and API submission. Extract the form rendering + local state into a reusable presentational component.

## Acceptance
File `core-module/gui/src/ProfileEditor.tsx`:

```tsx
import { useState } from 'react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Badge } from './ui/badge';
import { Alert, AlertDescription } from './ui/alert';

export interface ProfileData {
  email: string;
  given_name: string;
  family_name: string;
  is_admin: boolean;
  is_email_verified: boolean;
}

export interface ProfileEditorProps {
  initial: ProfileData;
  onSave: (next: Pick<ProfileData, 'given_name' | 'family_name'>) => Promise<void>;
  readOnly?: boolean;
}

export function ProfileEditor({ initial, onSave, readOnly }: ProfileEditorProps) {
  const [givenName, setGivenName] = useState(initial.given_name);
  const [familyName, setFamilyName] = useState(initial.family_name);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSuccess(false);
    setIsSubmitting(true);
    try {
      await onSave({ given_name: givenName, family_name: familyName });
      setSuccess(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      {/* ... identical layout to current profile/page.tsx ... */}
    </form>
  );
}
```

- The component is purely presentational — no `useAuth`, no direct `fetch` / `api` import.
- Export from `core-module/gui/src/index.ts`.

## How to verify
- `npm run build` in core-module/gui succeeds.
- `npm run typecheck` clean.
- Visual smoke (Phase 6.8) confirms rendering parity.

## Notes
- Keep the component unopinionated about server response shape — the `onSave` return type is `Promise<void>`. If the API returns an updated profile, the parent page handles refresh.
