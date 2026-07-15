import type { ApiErrorResponse, FieldErrorData } from './api-types';

// ─── Client error ───────────────────────────────────────────────────────────

/**
 * Thrown by `request()` for every failed call. Carries the reserved top-level
 * `code` (or the client-synthesized `network_error`), the HTTP `status` (or
 * `0` for a transport failure that never reached the server), and any
 * per-field `details` from the response envelope.
 *
 * A superset-compatible extension of mod-users' existing `ApiRequestError`
 * (adds `details`); the two established special cases it implements —
 * `network_error`/status 0 on a transport failure, and the 401-redirect /
 * 403-no-redirect split — are documented on `request()` below.
 */
export class ApiRequestError extends Error {
  code: string;
  status: number;
  details?: FieldErrorData[];

  constructor(code: string, message: string, status: number, details?: FieldErrorData[]) {
    super(message);
    this.name = 'ApiRequestError';
    this.code = code;
    this.status = status;
    this.details = details;
  }
}

// ─── Auth seam ──────────────────────────────────────────────────────────────
//
// mod-core/gui is a component library, not an app: hard-coding
// `localStorage`/`window.location` here would couple every consumer to a
// specific browser-storage and routing story. Instead `request()` defers
// "how do I get the current token" and "what happens on 401" to a small
// injectable seam (`ApiClientAuthHandler`) that a consuming app can override
// via `configureApiClient()`. The default implementation below is a
// browser-only convenience (matching mod-users' existing behavior: a
// `auth_token` key in `localStorage`, hard redirect to `/auth/login`) that is
// guarded to no-op outside a browser (SSR) so `tsc` and server-side
// rendering never break on `window`/`localStorage`.

export interface ApiClientAuthHandler {
  /** Returns the bearer token to attach to a request, or a falsy value if unauthenticated. */
  getToken: () => string | null | undefined;
  /** Called on a `401 unauthenticated` response (unless `skipAuthRedirect` is set): clear stored auth and route to login. */
  onUnauthenticated: () => void;
}

const defaultAuthHandler: ApiClientAuthHandler = {
  getToken: () => {
    if (typeof window === 'undefined') return null;
    return window.localStorage.getItem('auth_token');
  },
  onUnauthenticated: () => {
    if (typeof window === 'undefined') return;
    window.localStorage.removeItem('auth_token');
    window.location.href = '/auth/login';
  },
};

let authHandler: ApiClientAuthHandler = defaultAuthHandler;

/**
 * Overrides the token retrieval / unauthenticated handling `request()` uses.
 * Consuming apps call this once at startup (e.g. in an app-level provider)
 * to supply their own token storage and redirect behavior; any field omitted
 * keeps its current (default, browser-based) implementation.
 */
export function configureApiClient(handler: Partial<ApiClientAuthHandler>): void {
  authHandler = { ...authHandler, ...handler };
}

// ─── request() ──────────────────────────────────────────────────────────────

export interface RequestOptions extends RequestInit {
  /**
   * When true, a 401 response is surfaced to the caller as a thrown
   * `ApiRequestError` without clearing the stored token or redirecting.
   * Defaults to false: a 401 clears the token and redirects via the
   * configured auth handler.
   */
  skipAuthRedirect?: boolean;
}

/**
 * Typed fetch wrapper implementing the client contract from
 * docs/mf-standards/architecture/api-response-design.md
 * ("GUI-facing error-data contract" / "Client contract (ApiRequestError)"):
 *
 * - A transport failure (`fetch` rejects; no HTTP response reached) is
 *   synthesized as `ApiRequestError("network_error", <message>, 0)` — no
 *   envelope, never a field error.
 * - Any non-2xx HTTP response is parsed as the nested
 *   `{error: {code, message, details?}}` envelope and thrown as
 *   `ApiRequestError` carrying `error.code`, `error.message`, the real HTTP
 *   `status`, and `error.details`. An unparseable or missing body falls back
 *   to a generic code/message, still carrying the real status.
 * - Only `401` triggers the auth handler's `onUnauthenticated()` (token
 *   clear + redirect), and only when `skipAuthRedirect` is not set. `403`
 *   (including a masked not-found) never redirects — it is thrown like any
 *   other non-2xx error for inline handling.
 * - On success, the JSON body is parsed and returned typed via `T`; a `204`
 *   or otherwise empty body resolves to `undefined`.
 */
export async function request<T>(input: string | URL, options: RequestOptions = {}): Promise<T> {
  const { skipAuthRedirect = false, ...fetchOptions } = options;
  const token = authHandler.getToken();
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...fetchOptions.headers,
  };
  if (token) {
    (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
  }

  let response: Response;
  try {
    response = await fetch(input, { ...fetchOptions, headers });
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Network request failed';
    throw new ApiRequestError('network_error', message, 0);
  }

  if (!response.ok) {
    let code = 'unknown_error';
    let message = `Request failed with status ${response.status}`;
    let details: FieldErrorData[] | undefined;
    try {
      const body = (await response.json()) as Partial<ApiErrorResponse> | undefined;
      if (body?.error) {
        code = body.error.code ?? code;
        message = body.error.message ?? message;
        details = body.error.details;
      }
    } catch {
      // Body missing or unparseable — fall back to the generic code/message
      // above, which still carries the real HTTP status.
    }

    if (response.status === 401 && !skipAuthRedirect) {
      authHandler.onUnauthenticated();
    }

    throw new ApiRequestError(code, message, response.status, details);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  if (!text) {
    return undefined as T;
  }
  return JSON.parse(text) as T;
}
