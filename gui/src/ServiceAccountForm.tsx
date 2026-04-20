import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';

export interface ServiceAccountData {
  label: string;
}

export interface ServiceAccountFormProps {
  value: ServiceAccountData;
  onChange: (value: ServiceAccountData) => void;
  onSubmit?: () => void;
  isSubmitting?: boolean;
  readOnly?: boolean;
}

export function ServiceAccountForm({
  value,
  onChange,
  onSubmit,
  isSubmitting = false,
  readOnly = false,
}: ServiceAccountFormProps) {
  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    onSubmit?.();
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="sa-label">Label</Label>
        <Input
          id="sa-label"
          type="text"
          value={value.label}
          onChange={(e) => onChange({ ...value, label: e.target.value })}
          disabled={readOnly}
          required
          placeholder="e.g. CI pipeline, Billing service"
        />
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
