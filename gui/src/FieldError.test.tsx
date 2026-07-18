import { describe, expect, test } from 'bun:test';
import { render, screen } from '@testing-library/react';
import { FieldError } from './FieldError';
import type { FieldErrorData } from './lib/api-types';

const sampleError: FieldErrorData = {
  field: 'email',
  code: 'invalid',
  message: 'Enter a valid email address.',
};

describe('FieldError', () => {
  test('renders nothing when error is undefined', () => {
    const { container } = render(<FieldError />);
    expect(container).toBeEmptyDOMElement();
  });

  test('renders nothing when error is null', () => {
    const { container } = render(<FieldError error={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  test('renders an alert with the error message when error is populated', () => {
    render(<FieldError error={sampleError} />);
    expect(screen.getByRole('alert')).toHaveTextContent(sampleError.message);
  });

  test('applies the id prop to the rendered element', () => {
    render(<FieldError error={sampleError} id="email-error" />);
    expect(screen.getByRole('alert')).toHaveAttribute('id', 'email-error');
  });
});
