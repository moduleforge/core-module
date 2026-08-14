import { describe, expect, test } from 'bun:test';
import { render } from '@testing-library/react';
import { Card } from './card';

describe('Card', () => {
  test('carries data-mf-component="card" on its root element', () => {
    const { getByTestId } = render(<Card data-testid="card">Content</Card>);
    expect(getByTestId('card')).toHaveAttribute('data-mf-component', 'card');
  });
});
