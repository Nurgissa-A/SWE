'use client';

import useSWR, { type SWRConfiguration } from 'swr';
import { useAuth } from './useAuth';
import { useI18n } from '@/lib/i18n';

/**
 * SWR wrapper keyed by the signed-in user and language (server text such as notification
 * titles depends on both). Pass null as key to skip.
 */
export function useApi<T>(key: string | null, fetcher: () => Promise<T>, config?: SWRConfiguration<T>) {
  const { token } = useAuth();
  const { locale } = useI18n();
  return useSWR<T>(key ? [key, token ?? 'anonymous', locale] : null, fetcher, { revalidateOnFocus: false, shouldRetryOnError: false, ...config });
}
