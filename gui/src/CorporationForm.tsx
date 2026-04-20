import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';

export interface CorporationData {
  legal_name: string;
  jurisdiction?: string;
}

export interface CorporationFormProps {
  value: CorporationData;
  onChange: (value: CorporationData) => void;
  onSubmit?: () => void;
  isSubmitting?: boolean;
  readOnly?: boolean;
}

export function CorporationForm({
  value,
  onChange,
  onSubmit,
  isSubmitting = false,
  readOnly = false,
}: CorporationFormProps) {
  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    onSubmit?.();
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="corp-legal-name">Legal name</Label>
        <Input
          id="corp-legal-name"
          type="text"
          value={value.legal_name}
          onChange={(e) => onChange({ ...value, legal_name: e.target.value })}
          disabled={readOnly}
          required
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="corp-jurisdiction">Jurisdiction</Label>
        <Input
          id="corp-jurisdiction"
          type="text"
          value={value.jurisdiction ?? ''}
          onChange={(e) =>
            onChange({ ...value, jurisdiction: e.target.value || undefined })
          }
          disabled={readOnly}
          placeholder="e.g. Delaware, USA"
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
