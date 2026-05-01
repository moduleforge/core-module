import { useState } from 'react';
import type { Story } from '@ladle/react';
import {
  ServiceAccountForm,
  type ServiceAccountData,
  type ServiceAccountFormProps,
} from './ServiceAccountForm';

type ControlledArgs = Pick<ServiceAccountFormProps, 'isSubmitting' | 'readOnly'> & {
  initial: ServiceAccountData;
  withSubmit: boolean;
};

function Controlled({ initial, withSubmit, isSubmitting, readOnly }: ControlledArgs) {
  const [value, setValue] = useState<ServiceAccountData>(initial);
  return (
    <ServiceAccountForm
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
  initial: { label: 'CI pipeline' },
  withSubmit: true,
  isSubmitting: false,
  readOnly: false,
};

export const Empty: Story<ControlledArgs> = (args) => <Controlled {...args} />;
Empty.args = {
  initial: { label: '' },
  withSubmit: true,
  isSubmitting: false,
  readOnly: false,
};

export const ReadOnly: Story<ControlledArgs> = (args) => <Controlled {...args} />;
ReadOnly.args = {
  initial: { label: 'Billing service' },
  withSubmit: false,
  isSubmitting: false,
  readOnly: true,
};
