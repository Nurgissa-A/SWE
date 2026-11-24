'use client';

import Link from 'next/link';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { OrderSummary } from '@/lib/types';
import { RequireSignIn } from '@/components/layout/RequireSignIn';
import { Badge } from '@/components/ui/Badge';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Spinner } from '@/components/ui/Spinner';

export const ORDER_TONE: Record<string, 'default' | 'success' | 'error' | 'muted'> = {
  completed: 'success', pending: 'default', refunded: 'muted', cancelled: 'muted', failed: 'error', expired: 'muted',
};

export default function MyOrdersPage() {
  return <RequireSignIn><MyOrders /></RequireSignIn>;
}

function MyOrders() {
  const { t, money, dateTime } = useI18n();
  const { data, error, isLoading, mutate } = useApi<OrderSummary[]>('my-orders', () => call(api.GET('/api/orders')));
  const now = Date.now();
  const upcoming = (data ?? []).filter((o) => new Date(o.startsAt).getTime() >= now - 6 * 3600_000);
  const past = (data ?? []).filter((o) => new Date(o.startsAt).getTime() < now - 6 * 3600_000);

  const list = (rows: OrderSummary[]) => (
    <ul className="grid gap-3">
      {rows.map((o) => (
        <li key={o.id}>
          <Link href={`/orders/${o.id}`} className="grid gap-3 rounded-card border border-border bg-surface p-4 hover:border-primary sm:grid-cols-[1fr_auto] sm:items-center">
            <div>
              <p className="font-semibold text-text">{o.eventTitle}</p>
              <p className="mt-1 text-sm text-text-muted">{dateTime(o.startsAt)} · {o.number} · {t('{n} tickets', { n: o.tickets })}</p>
            </div>
            <div className="flex items-center gap-3"><span className="font-semibold">{o.totalKzt ? money(o.totalKzt) : t('Free')}</span><Badge tone={ORDER_TONE[o.status]}>{o.status}</Badge></div>
          </Link>
        </li>
      ))}
    </ul>
  );

  return (
    <main className="mx-auto grid max-w-4xl gap-8 px-4 py-8 sm:px-6 lg:py-12">
      <div>
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">{t('Your plans')}</p>
        <h1 className="mt-2 text-3xl font-bold text-text">{t('My tickets')}</h1>
      </div>
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} /> : (data ?? []).length === 0 ? (
        <div className="rounded-card border border-dashed border-border p-8 text-center">
          <p className="font-semibold text-text">{t('No tickets yet')}</p>
          <p className="mt-1 text-sm text-text-muted">{t('When you register or buy tickets, they appear here.')}</p>
          <Link href="/events" className="mt-4 inline-flex min-h-11 items-center rounded-full bg-primary px-5 font-semibold text-white">{t('Explore events')}</Link>
        </div>
      ) : (
        <>
          <section className="grid gap-3"><h2 className="font-sans text-lg font-bold tracking-normal">{t('Upcoming')}</h2>{upcoming.length ? list(upcoming) : <p className="text-sm text-text-muted">{t('Nothing coming up.')}</p>}</section>
          {past.length ? <section className="grid gap-3"><h2 className="font-sans text-lg font-bold tracking-normal">{t('Past')}</h2>{list(past)}</section> : null}
        </>
      )}
    </main>
  );
}
