import { describe, expect, test } from 'bun:test';
import { render, screen } from '@testing-library/react';
import { Button } from './button';

describe('Button', () => {
  test('carries data-mf-component="button" on its root element', () => {
    render(<Button>Click me</Button>);
    expect(screen.getByRole('button', { name: 'Click me' })).toHaveAttribute(
      'data-mf-component',
      'button',
    );
  });
});
