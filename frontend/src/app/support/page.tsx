'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { SupportCase } from '@/lib/types';
import { CATEGORY_LABEL, NewCaseForm, type Category } from '@/components/support/NewCaseForm';
import { RequireSignIn } from '@/components/layout/RequireSignIn';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Spinner } from '@/components/ui/Spinner';

export const STATUS_TONE: Record<string, 'default' | 'success' | 'muted' | 'error'> = { open: 'default', in_progress: 'default', waiting_for_customer: 'muted', resolved: 'success' };

export default function SupportPage() {
  return <RequireSignIn><SupportList /></RequireSignIn>;
}

function SupportList() {
  const { t, dateTime } = useI18n();
  const { user } = useAuth();
  const [status, setStatus] = useState('');
  const [creating, setCreating] = useState(false);
  const { data, error, isLoading, mutate } = useApi<SupportCase[]>(`cases:${status}`, () => call(api.GET('/api/support/cases', { params: { query: status ? { status } : {} } })), { refreshInterval: 20_000 });
  const mine = (data ?? []).filter((c) => c.requesterId === user?.id);
  const handling = (data ?? []).filter((c) => c.requesterId !== user?.id);

  const list = (rows: SupportCase[]) => (
    <ul className="grid gap-3">
      {rows.map((c) => (
        <li key={c.id}>
          <Link href={`/support/${c.id}`} className="flex flex-wrap items-center justify-between gap-3 rounded-card border border-border bg-surface p-4 hover:border-primary">
            <div className="min-w-0">
              <p className="font-semibold text-text">#{c.number} · {c.subject}</p>
              <p className="mt-1 text-sm text-text-muted">{t(CATEGORY_LABEL[c.category as Category] ?? c.category ?? '')}{c.eventTitle ? ` · ${c.eventTitle}` : ''} · {dateTime(c.updatedAt, undefined, false)}</p>
            </div>
            <div className="flex gap-2">{c.escalated ? <Badge tone="error">{t('Escalated')}</Badge> : null}<Badge tone={STATUS_TONE[c.status ?? 'open']}>{c.status ?? 'open'}</Badge></div>
          </Link>
        </li>
      ))}
    </ul>
  );

  return (
    <main className="mx-auto grid max-w-5xl gap-6 px-4 py-8 sm:px-6 lg:py-12">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">{t('Help centre')}</p>
          <h1 className="mt-2 text-3xl font-bold text-text">{t('Support')}</h1>
          <p className="mt-2 text-text-muted">{t('Questions about a ticket? Open a case from your order so we can see which one.')}</p>
        </div>
        <div className="flex gap-2">
          <select aria-label={t('Filter by status')} value={status} onChange={(e) => setStatus(e.currentTarget.value)} className="min-h-11 rounded-full border border-border bg-surface px-4 text-sm">
            <option value="">{t('All statuses')}</option>
            {['open', 'in_progress', 'waiting_for_customer', 'resolved'].map((s) => <option key={s} value={s}>{t(s.replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase()))}</option>)}
          </select>
          {user?.role === 'organizer' ? <Button variant="secondary" onClick={() => setCreating((v) => !v)}>{t('Contact BiletFlow')}</Button> : null}
        </div>
      </div>
      {creating ? <NewCaseForm kind="organizer" defaultCategory="activation" onDone={() => { setCreating(false); void mutate(); }} /> : null}
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} /> : (
        <>
          {handling.length ? <section className="grid gap-3"><h2 className="font-sans text-lg font-bold tracking-normal">{t(user?.role === 'platform_admin' ? 'Cases for BiletFlow' : 'Questions from your attendees')}</h2>{list(handling)}</section> : null}
          <section className="grid gap-3">
            <h2 className="font-sans text-lg font-bold tracking-normal">{t('My cases')}</h2>
            {mine.length ? list(mine) : <p className="rounded-card border border-dashed border-border p-6 text-sm text-text-muted">{t('You have no support cases. To ask about an order, open it from My tickets.')}</p>}
          </section>
        </>
      )}
    </main>
  );
}
