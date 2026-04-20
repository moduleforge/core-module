import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';

export interface NaturalPersonData {
  given_name: string;
  family_name: string;
}

export interface NaturalPersonFormProps {
  value: NaturalPersonData;
  onChange: (value: NaturalPersonData) => void;
  onSubmit?: () => void;
  isSubmitting?: boolean;
  readOnly?: boolean;
}

export function NaturalPersonForm({
  value,
  onChange,
  onSubmit,
  isSubmitting = false,
  readOnly = false,
}: NaturalPersonFormProps) {
  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    onSubmit?.();
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="np-given-name">First name</Label>
          <Input
            id="np-given-name"
            type="text"
            value={value.given_name}
            onChange={(e) => onChange({ ...value, given_name: e.target.value })}
            disabled={readOnly}
            required
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="np-family-name">Last name</Label>
          <Input
            id="np-family-name"
            type="text"
            value={value.family_name}
            onChange={(e) => onChange({ ...value, family_name: e.target.value })}
            disabled={readOnly}
            required
          />
        </div>
      </div>
      {!readOnly && onSubmit && (
        <div className="flex justify-end">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Saving...' : 'Save changes'}
          </Button>
        </div>
      )}
    </form>
  );
}
