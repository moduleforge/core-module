import { useState, type CSSProperties } from 'react';
import { CheckCircle2 } from 'lucide-react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Badge } from './ui/badge';

export interface ProfileData {
  email: string;
  given_name: string;
  family_name: string;
  is_admin: boolean;
  is_email_verified: boolean;
  created_at?: string;
}

export interface ProfileEditorProps {
  initial: ProfileData;
  onSave: (patch: Pick<ProfileData, 'given_name' | 'family_name'>) => Promise<void>;
  readOnly?: boolean;
}

export function ProfileEditor({ initial, onSave, readOnly = false }: ProfileEditorProps) {
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
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Something went wrong. Please try again.');
      }
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    // Adopts core-gui's own `container` utility (tokens/CONTRACT.md's "Tailwind container
    // integration") — the same idiom app-mftodo's TaskListContainer.tsx / TaskDetailContainer.tsx
    // / TaskEditorContainer.tsx already consume — instead of the ad hoc `max-w-xl` this used to
    // hardcode (followup AkGw). `container`'s width comes from `--mf-max-content-width`, which
    // defaults to 80rem — far wider than this page's established 36rem measure — so the width is
    // pinned locally via a scoped inline override of that custom property, following the same
    // per-component `--mf-x` override mechanism demonstrated in TokenScoping.stories.tsx
    // (`FallbackChain`), rather than touching the package-wide default. This is exactly the
    // per-page narrow-width need CONTRACT.md's "Known limitation" section under Spacing and
    // container width flags as accepted-and-open: a per-component override, not a global one.
    <div
      className="container"
      style={{ ['--mf-max-content-width' as string]: '36rem' } as CSSProperties}
    >
      <div className="mb-6">
        <h1 className="text-2xl font-semibold">Profile</h1>
        <p className="text-sm text-muted-foreground mt-1">Manage your account details</p>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>{initial.given_name} {initial.family_name}</CardTitle>
              <CardDescription>{initial.email}</CardDescription>
            </div>
            {initial.is_admin && (
              <Badge>Admin</Badge>
            )}
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {error && (
              <div className="flex items-center gap-2 rounded-lg border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {error}
              </div>
            )}
            {success && (
              <div className="flex items-center gap-2 rounded-lg border border-success/50 bg-success/10 px-3 py-2 text-sm text-success">
                <CheckCircle2 className="size-4" />
                Profile updated successfully.
              </div>
            )}
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={initial.email}
                disabled
                className="opacity-60"
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="given-name">First name</Label>
                <Input
                  id="given-name"
                  type="text"
                  value={givenName}
                  onChange={(e) => setGivenName(e.target.value)}
                  disabled={readOnly}
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="family-name">Last name</Label>
                <Input
                  id="family-name"
                  type="text"
                  value={familyName}
                  onChange={(e) => setFamilyName(e.target.value)}
                  disabled={readOnly}
                  required
                />
              </div>
            </div>
            {!readOnly && (
              <div className="flex justify-end">
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? 'Saving...' : 'Save changes'}
                </Button>
              </div>
            )}
          </form>
        </CardContent>
      </Card>

      {initial.created_at && (
        <div className="mt-4 text-xs text-muted-foreground">
          Account created: {new Date(initial.created_at).toLocaleDateString()}
        </div>
      )}
    </div>
  );
}
