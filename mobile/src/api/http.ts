import { ApiError } from './errors';
import { loadSession } from './session';
import { REQUEST_TIMEOUT_MS } from './config';

const NO_INTERNET =
  'Could not reach the BiletFlow server. Check your internet connection (and that the backend is running) and try again.';

// Sends one request to the backend and returns the JSON response.
// Errors come back as ApiError, using the backend's { error: { code, message } } format.
export async function request<T>(
  baseUrl: string,
  path: string,
  options: { method?: string; body?: unknown; auth?: boolean } = {}
): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';
  if (options.auth) {
    const session = await loadSession();
    if (session) headers.Authorization = `Bearer ${session.token}`;
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);

  let response: Response;
  try {
    response = await fetch(baseUrl + path, {
      method: options.method ?? 'GET',
      headers,
      body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
      signal: controller.signal,
    });
  } catch {
    throw new ApiError('network', NO_INTERNET);
  } finally {
    clearTimeout(timer);
  }

  const text = await response.text();
  let data: any = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = null;
  }

  if (!response.ok) {
    if (response.status === 401) {
      throw new ApiError('unauthorized', data?.error?.message ?? 'Please sign in again.');
    }
    throw new ApiError(
      data?.error?.code ?? `http_${response.status}`,
      data?.error?.message ?? `The server returned an error (${response.status}). Please try again.`
    );
  }

  return data as T;
}
