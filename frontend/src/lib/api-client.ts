'use client';

import createClient, { type Middleware } from 'openapi-fetch';
import type { paths } from './api-types';
import { currentLocale } from './i18n';

const baseUrl = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:3000';

// The API returns a JWT bearer token (no cookie session yet). It is kept in memory and mirrored
// to sessionStorage so a page reload keeps you signed in for this browser tab only; closing the
// tab signs you out. Tokens expire after the identity service's access-token lifetime.
const STORAGE_KEY = 'biletflow.token';
let authToken: string | null = null;

function readStored(): string | null {
  try {
    return typeof window === 'undefined' ? null : window.sessionStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

export function setAuthToken(token: string | null): void {
  authToken = token?.trim() || null;
  try {
    if (authToken) window.sessionStorage.setItem(STORAGE_KEY, authToken);
    else window.sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Storage can be unavailable (private mode); the in-memory token still works.
  }
}

export function getAuthToken(): string | null {
  if (!authToken) authToken = readStored();
  return authToken;
}

const headersMiddleware: Middleware = {
  onRequest({ request }) {
    const headers = new Headers(request.headers);
    headers.set('X-Locale', currentLocale());
    const token = getAuthToken();
    if (token) headers.set('Authorization', `Bearer ${token}`);
    return new Request(request, { headers });
  },
};

export const authApi = createClient<paths>({ baseUrl });
export const eventsApi = createClient<paths>({ baseUrl });
/** Client for every Go business route (orders, check-in, support, admin, …). */
export const api = eventsApi;

authApi.use(headersMiddleware);
eventsApi.use(headersMiddleware);

/** Download a protected file (PDF tickets, CSV exports) with the bearer token. */
export async function downloadFile(path: string, filename: string): Promise<void> {
  const headers: Record<string, string> = { 'X-Locale': currentLocale() };
  const token = getAuthToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await fetch(path, { headers });
  if (!response.ok) {
    const { toApiRequestError } = await import('./api-errors');
    throw toApiRequestError(await response.json().catch(() => undefined), response.status);
  }
  const url = URL.createObjectURL(await response.blob());
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}
