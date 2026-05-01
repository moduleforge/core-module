import { useState } from 'react';
import type { Story } from '@ladle/react';
import {
  CorporationForm,
  type CorporationData,
  type CorporationFormProps,
} from './CorporationForm';

type ControlledArgs = Pick<CorporationFormProps, 'isSubmitting' | 'readOnly'> & {
  initial: CorporationData;
  withSubmit: boolean;
};

function Controlled({ initial, withSubmit, isSubmitting, readOnly }: ControlledArgs) {
  const [value, setValue] = useState<CorporationData>(initial);
  return (
    <CorporationForm
      value={value}
      onChange={setValue}
      onSubmit={withSubmit ? () => console.log('submit', value) : undefined}
      isSubmitting={isSubmitting}
      readOnly={readOnly}
    />
  );
}

export const Default: Story<ControlledArgs> = (args) => <Controlled {...args} />;
Default.args = {
  initial: { legal_name: 'Acme Corp.', jurisdiction: 'Delaware, USA' },
  withSubmit: true,
  isSubmitting: false,
  readOnly: false,
};

export const Empty: Story<ControlledArgs> = (args) => <Controlled {...args} />;
Empty.args = {
  initial: { legal_name: '' },
  withSubmit: true,
  isSubmitting: false,
  readOnly: false,
};

export const NoJurisdiction: Story<ControlledArgs> = (args) => <Controlled {...args} />;
NoJurisdiction.args = {
  initial: { legal_name: 'Acme Corp.' },
  withSubmit: true,
  isSubmitting: false,
  readOnly: false,
};

export const ReadOnly: Story<ControlledArgs> = (args) => <Controlled {...args} />;
ReadOnly.args = {
  initial: { legal_name: 'Acme Corp.', jurisdiction: 'Delaware, USA' },
  withSubmit: false,
  isSubmitting: false,
  readOnly: true,
};
