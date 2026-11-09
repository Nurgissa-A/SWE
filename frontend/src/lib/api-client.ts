'use client';

import createClient, { type Middleware } from 'openapi-fetch';
import type { paths } from './api-types';

const baseUrl = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:3000';

// The API returns a JWT instead of an httpOnly session cookie, so keep it in memory until the backend supports cookies.
let authToken: string | null = null;

export function setAuthToken(token: string | null): void {
  authToken = token?.trim() || null;
}

export function getAuthToken(): string | null {
  return authToken;
}

const authorizationMiddleware: Middleware = {
  onRequest({ request }) {
    if (!authToken) {
      return request;
    }

    const headers = new Headers(request.headers);
    headers.set('Authorization', `Bearer ${authToken}`);

    return new Request(request, { headers });
  },
};

export const authApi = createClient<paths>({ baseUrl });
export const eventsApi = createClient<paths>({ baseUrl });

authApi.use(authorizationMiddleware);
eventsApi.use(authorizationMiddleware);
