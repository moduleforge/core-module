import { useState } from 'react';
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
    <div className="p-6 max-w-xl">
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
              <div className="flex items-center gap-2 rounded-lg border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">
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
