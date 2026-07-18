import { describe, expect, test } from 'bun:test';
import { render, screen } from '@testing-library/react';
import { ErrorBanner } from './ErrorBanner';
import type { ApiError } from './lib/api-types';

describe('ErrorBanner', () => {
  test('renders nothing when error is undefined', () => {
    const { container } = render(<ErrorBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  test('renders nothing when error is null', () => {
    const { container } = render(<ErrorBanner error={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  test('renders a plain string error as the description with no title', () => {
    render(<ErrorBanner error="Something went wrong." />);
    expect(screen.getByRole('alert')).toHaveTextContent('Something went wrong.');
    expect(screen.queryByText('Something went wrong.')?.closest('[data-slot="alert-title"]')).toBeNull();
  });

  test('renders an ApiError-like value with message as the description, no title', () => {
    const apiError: ApiError = { code: 'forbidden', message: 'You do not have access.' };
    render(<ErrorBanner error={apiError} />);
    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('You do not have access.');
    expect(alert.querySelector('[data-slot="alert-title"]')).toBeNull();
  });

  test('renders an explicit title/description pair with both texts', () => {
    render(<ErrorBanner error={{ title: 'Request failed', description: 'Please try again.' }} />);
    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Request failed');
    expect(alert).toHaveTextContent('Please try again.');
  });

  test('renders the destructive Alert role="alert" root', () => {
    render(<ErrorBanner error="Something went wrong." />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  test('renders an AlertCircle icon as the first child of the alert', () => {
    render(<ErrorBanner error="Something went wrong." />);
    const alert = screen.getByRole('alert');
    const icon = alert.querySelector('svg');
    expect(icon).toBeInTheDocument();
    expect(alert.firstElementChild).toBe(icon);
  });
});
