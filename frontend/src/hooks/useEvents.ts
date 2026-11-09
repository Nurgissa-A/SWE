'use client';

import useSWR, { useSWRConfig } from 'swr';
import type { components, paths } from '@/lib/api-types';
import { eventsApi } from '@/lib/api-client';
import { ApiRequestError, toApiRequestError } from '@/lib/api-errors';
import { useAuth } from './useAuth';

export type ApiEvent = components['schemas']['Event'];
export type EventStatusFilter = NonNullable<paths['/api/events']['get']['parameters']['query']>['status'];

export function useEvents(status?: EventStatusFilter) {
  const { token, logout } = useAuth();
  const key = ['events', token ?? 'anonymous', status ?? 'all'] as const;

  return useSWR<ApiEvent[]>(key, async () => {
    const options = status ? { params: { query: { status } } } : undefined;
    const { data, error, response } = await eventsApi.GET('/api/events', options);
    const statusCode = response.status;
    if (error) throw toApiRequestError(error, statusCode);
    return data ?? [];
  }, {
    revalidateOnFocus: false,
    shouldRetryOnError: false,
    onError: (requestError) => {
      if (token && requestError instanceof ApiRequestError && requestError.status === 401) void logout();
    },
  });
}

export function useEvent(id: string | undefined) {
  const { token, logout } = useAuth();
  return useSWR<ApiEvent>(
    id ? ['event', id, token ?? 'anonymous'] : null,
    async () => {
      if (!id) throw new Error('An event ID is required.');
      const { data, error, response } = await eventsApi.GET('/api/events/{eventId}', {
        params: { path: { eventId: id } },
      });
      const statusCode = response.status;
      if (error) throw toApiRequestError(error, statusCode);
      if (!data) throw toApiRequestError(undefined, statusCode);
      return data;
    },
    {
      revalidateOnFocus: false,
      shouldRetryOnError: false,
      onError: (requestError) => {
        if (token && requestError instanceof ApiRequestError && requestError.status === 401) void logout();
      },
    },
  );
}

export function useRefreshEventLists() {
  const { mutate } = useSWRConfig();
  return () => mutate(
    (key: unknown) => Array.isArray(key) && key[0] === 'events',
    undefined,
    { revalidate: true },
  );
}
