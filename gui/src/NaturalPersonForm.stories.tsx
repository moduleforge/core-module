import { useState } from 'react';
import type { Story } from '@ladle/react';
import {
  NaturalPersonForm,
  type NaturalPersonData,
  type NaturalPersonFormProps,
} from './NaturalPersonForm';

type ControlledArgs = Pick<NaturalPersonFormProps, 'isSubmitting' | 'readOnly'> & {
  initial: NaturalPersonData;
  withSubmit: boolean;
};

function Controlled({ initial, withSubmit, isSubmitting, readOnly }: ControlledArgs) {
  const [value, setValue] = useState<NaturalPersonData>(initial);
  return (
    <NaturalPersonForm
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
  initial: { given_name: 'Alex', family_name: 'Rivera' },
  withSubmit: true,
  isSubmitting: false,
  readOnly: false,
};

export const Empty: Story<ControlledArgs> = (args) => <Controlled {...args} />;
Empty.args = {
  initial: { given_name: '', family_name: '' },
  withSubmit: true,
  isSubmitting: false,
  readOnly: false,
};

export const Submitting: Story<ControlledArgs> = (args) => <Controlled {...args} />;
Submitting.args = {
  initial: { given_name: 'Alex', family_name: 'Rivera' },
  withSubmit: true,
  isSubmitting: true,
  readOnly: false,
};

export const ReadOnly: Story<ControlledArgs> = (args) => <Controlled {...args} />;
ReadOnly.args = {
  initial: { given_name: 'Alex', family_name: 'Rivera' },
  withSubmit: false,
  isSubmitting: false,
  readOnly: true,
};
