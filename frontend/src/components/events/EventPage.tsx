'use client';

import { useState } from 'react';
import Link from 'next/link';
import useSWR from 'swr';
import { useAuth } from '@/hooks/useAuth';
import { api } from '@/lib/api-client';
import { call, callEmpty, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent } from '@/lib/types';
import { EventPreview } from './EventPreview';
import { TicketPanel } from '@/components/checkout/TicketPanel';
import { NewCaseForm } from '@/components/support/NewCaseForm';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Spinner } from '@/components/ui/Spinner';
import { TextArea } from '@/components/ui/TextArea';

export function EventPage({ id, accessKey, promo }: { id: string; accessKey?: string; promo?: string }) {
  const { t } = useI18n();
  const { user, token, isLoading: userLoading } = useAuth();
  const { data: event, error, isLoading, mutate } = useSWR<ApiEvent>(['event', id, accessKey ?? '', token ?? ''],
    () => call(api.GET('/api/events/{eventId}', { params: { path: { eventId: id }, query: accessKey ? { key: accessKey } : {} } })),
    { revalidateOnFocus: false, shouldRetryOnError: false });
  const [panel, setPanel] = useState<'none' | 'support' | 'report'>('none');

  if (isLoading || userLoading) return <main className="mx-auto grid min-h-[50vh] max-w-7xl place-items-center px-4"><Spinner label="Loading event details" /></main>;
  if (error || !event) {
    const status = (error as { status?: number } | undefined)?.status;
    return (
      <main className="mx-auto max-w-3xl px-4 py-16 sm:px-6">
        {status === 404 || !error ? (
          <div className="rounded-card border border-border bg-surface p-8 text-center">
            <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">{t('Event not found')}</p>
            <h1 className="mt-3 text-3xl font-bold text-text">{t('This event may have moved.')}</h1>
            <p className="mt-3 text-text-muted">{t('Try browsing the latest events instead.')}</p>
            <Link href="/events" className="mt-6 inline-flex min-h-11 items-center rounded-full bg-primary px-5 font-semibold text-white hover:bg-primary-hover">{t('Explore events')}</Link>
          </div>
        ) : <ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} />}
      </main>
    );
  }

  const isOwner = event.organizerId === user?.id;
  return (
    <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8 lg:py-12">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <Link href="/events" className="inline-flex font-semibold text-primary hover:text-primary-hover">← {t('Back to events')}</Link>
        {isOwner ? <Link href={`/dashboard/events/${event.id}`} className="inline-flex min-h-10 items-center rounded-full border border-border px-4 text-sm font-semibold text-text hover:border-primary">{t('Manage this event')}</Link> : null}
      </div>
      <EventPreview
        event={event}
        aside={<TicketPanel event={event} accessKey={accessKey} initialPromo={promo} />}
        footer={user && !isOwner ? (
          <div className="grid gap-4">
            <div className="flex flex-wrap gap-3 text-sm">
              <button type="button" onClick={() => setPanel(panel === 'support' ? 'none' : 'support')} className="font-semibold text-primary hover:text-primary-hover">{t('Ask the organizer a question')}</button>
              <span aria-hidden="true" className="text-border">|</span>
              <button type="button" onClick={() => setPanel(panel === 'report' ? 'none' : 'report')} className="font-semibold text-text-muted hover:text-error">{t('Report this event')}</button>
            </div>
            {panel === 'support' ? <NewCaseForm eventId={event.id} defaultCategory="event_information" onDone={() => setPanel('none')} /> : null}
            {panel === 'report' ? <ReportForm eventId={event.id!} onDone={() => setPanel('none')} /> : null}
          </div>
        ) : null}
      />
    </main>
  );
}

function ReportForm({ eventId, onDone }: { eventId: string; onDone: () => void }) {
  const { t } = useI18n();
  const [reason, setReason] = useState('');
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  if (sent) return <p role="status" className="text-sm font-semibold text-success">{t('Thank you. BiletFlow moderators will review this event.')}</p>;
  return (
    <Form onSubmit={async () => {
      setBusy(true); setError('');
      try { await callEmpty(api.POST('/api/events/{eventId}/report', { params: { path: { eventId } }, body: { reason } })); setSent(true); }
      catch (e) { setError(getApiErrorMessage(e)); } finally { setBusy(false); }
    }} className="grid gap-3 rounded-card border border-border bg-background p-4">
      <TextArea label="What is wrong with this event?" required value={reason} onChange={(e) => setReason(e.currentTarget.value)} />
      {error ? <ErrorMessage message={error} /> : null}
      <div className="flex gap-2"><Button type="submit" variant="danger" loading={busy}>{t('Send report')}</Button><Button variant="quiet" onClick={onDone}>{t('Cancel')}</Button></div>
    </Form>
  );
}
