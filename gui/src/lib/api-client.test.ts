import { afterEach, describe, expect, mock, test } from 'bun:test';
import { ApiRequestError, request } from './api-client';

const originalFetch = global.fetch;

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('request()', () => {
  afterEach(() => {
    global.fetch = originalFetch;
  });

  test('resolves to the parsed, typed body on a 200 JSON response', async () => {
    const payload = { id: '1', name: 'Acme Corp.' };
    global.fetch = mock(() => Promise.resolve(jsonResponse(200, payload))) as unknown as typeof fetch;

    const result = await request<typeof payload>('/api/widgets/1');

    expect(result).toEqual(payload);
  });

  test('resolves to undefined on a 204 empty response without parsing a body', async () => {
    global.fetch = mock(() =>
      Promise.resolve(new Response(null, { status: 204 }))
    ) as unknown as typeof fetch;

    const result = await request('/api/widgets/1', { method: 'DELETE' });

    expect(result).toBeUndefined();
  });

  test('throws a network_error ApiRequestError when fetch rejects', async () => {
    global.fetch = mock(() => Promise.reject(new Error('fetch failed'))) as unknown as typeof fetch;

    await expect(request('/api/widgets/1')).rejects.toBeInstanceOf(ApiRequestError);

    global.fetch = mock(() => Promise.reject(new Error('fetch failed'))) as unknown as typeof fetch;
    try {
      await request('/api/widgets/1');
      throw new Error('expected request() to throw');
    } catch (err) {
      const apiErr = err as ApiRequestError;
      expect(apiErr.code).toBe('network_error');
      expect(apiErr.status).toBe(0);
    }
  });

  test('throws an ApiRequestError with the envelope fields on a non-2xx response', async () => {
    const envelope = {
      error: {
        code: 'not_found',
        message: 'Widget not found.',
        details: [{ field: 'id', code: 'unknown_id', message: 'No widget with this id.' }],
      },
    };
    global.fetch = mock(() => Promise.resolve(jsonResponse(404, envelope))) as unknown as typeof fetch;

    try {
      await request('/api/widgets/missing');
      throw new Error('expected request() to throw');
    } catch (err) {
      const apiErr = err as ApiRequestError;
      expect(apiErr).toBeInstanceOf(ApiRequestError);
      expect(apiErr.code).toBe(envelope.error.code);
      expect(apiErr.message).toBe(envelope.error.message);
      expect(apiErr.status).toBe(404);
      expect(apiErr.details).toEqual(envelope.error.details);
    }
  });
});
