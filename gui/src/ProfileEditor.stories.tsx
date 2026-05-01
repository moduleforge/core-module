import type { Story } from '@ladle/react';
import { ProfileEditor, type ProfileData, type ProfileEditorProps } from './ProfileEditor';

const basePerson: ProfileData = {
  email: 'alex@example.com',
  given_name: 'Alex',
  family_name: 'Rivera',
  is_admin: false,
  is_email_verified: true,
  created_at: '2025-11-14T10:22:00Z',
};

async function succeed() {
  await new Promise((r) => setTimeout(r, 300));
}

async function fail() {
  await new Promise((r) => setTimeout(r, 300));
  throw new Error('Profile update failed: simulated server error.');
}

export const Default: Story<ProfileEditorProps> = (args) => <ProfileEditor {...args} />;
Default.args = { initial: basePerson, onSave: succeed, readOnly: false };

export const Admin: Story<ProfileEditorProps> = (args) => <ProfileEditor {...args} />;
Admin.args = {
  initial: { ...basePerson, is_admin: true, given_name: 'Dana', family_name: 'Okafor' },
  onSave: succeed,
};

export const ReadOnly: Story<ProfileEditorProps> = (args) => <ProfileEditor {...args} />;
ReadOnly.args = { initial: basePerson, onSave: succeed, readOnly: true };

export const ServerError: Story<ProfileEditorProps> = (args) => <ProfileEditor {...args} />;
ServerError.args = { initial: basePerson, onSave: fail };

export const NoCreatedAt: Story<ProfileEditorProps> = (args) => <ProfileEditor {...args} />;
NoCreatedAt.args = { initial: { ...basePerson, created_at: undefined }, onSave: succeed };
