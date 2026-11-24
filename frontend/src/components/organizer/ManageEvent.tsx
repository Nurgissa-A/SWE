'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import useSWR from 'swr';
import { useAuth } from '@/hooks/useAuth';
import { useRefreshEventLists } from '@/hooks/useEvents';
import { api } from '@/lib/api-client';
import { call, callEmpty, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent } from '@/lib/types';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Spinner } from '@/components/ui/Spinner';
import { Tabs } from '@/components/ui/Tabs';
import { TextArea } from '@/components/ui/TextArea';
import { ActivationPanel } from './ActivationPanel';
import { AnalyticsPanel } from './AnalyticsPanel';
import { DetailsPanel } from './DetailsPanel';
import { OrdersPanel } from './OrdersPanel';
import { PromoPanel } from './PromoPanel';
import { StaffPanel } from './StaffPanel';
import { TicketTypesPanel } from './TicketTypesPanel';
import { TimelinePanel } from './TimelinePanel';

const TABS = [
  { id: 'overview', label: 'Overview' }, { id: 'details', label: 'Details' }, { id: 'tickets', label: 'Tickets' },
  { id: 'paid', label: 'Paid sales' }, { id: 'orders', label: 'Orders' }, { id: 'promo', label: 'Promo codes' },
  { id: 'staff', label: 'Door staff' }, { id: 'history', label: 'History' },
];

export function ManageEvent({ id, initialTab }: { id: string; initialTab?: string }) {
  const { t, dateBlock, dateTime } = useI18n();
  const { token } = useAuth();
  const router = useRouter();
  const refreshLists = useRefreshEventLists();
  const [tab, setTabState] = useState(TABS.some((x) => x.id === initialTab) ? initialTab! : 'overview');
  const { data: event, error, isLoading, mutate } = useSWR<ApiEvent>(['manage-event', id, token],
    () => call(api.GET('/api/events/{eventId}', { params: { path: { eventId: id } } })), { revalidateOnFocus: false, shouldRetryOnError: false });
  const [busy, setBusy] = useState('');
  const [actionError, setActionError] = useState('');
  const [cancelling, setCancelling] = useState(false);
  const [reason, setReason] = useState('');

  function setTab(next: string) {
    setTabState(next);
    window.history.replaceState(null, '', `?tab=${next}`);
  }

  async function act(name: string, run: () => Promise<ApiEvent | void>) {
    setBusy(name); setActionError('');
    try {
      const result = await run();
      if (result) await mutate(result, { revalidate: false }); else await mutate();
      await refreshLists();
    } catch (e) { setActionError(getApiErrorMessage(e)); } finally { setBusy(''); }
  }

  if (isLoading) return <Spinner label="Loading event details" />;
  if (error || !event) return <ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} />;

  const date = dateBlock(event.startsAt, event.timezone);
  const shareUrl = typeof window === 'undefined' ? '' : `${window.location.origin}/events/${event.id}${event.visibility === 'private' && event.accessToken ? `?key=${event.accessToken}` : ''}`;

  return (
    <section className="grid gap-6">
      <Link href="/dashboard/events" className="text-sm font-semibold text-primary hover:text-primary-hover">← {t('My events')}</Link>
      <div className="rounded-card bg-navy p-5 text-surface sm:p-7">
        <div className="flex flex-wrap items-start gap-5">
          {date ? (
            <div className="grid h-18 w-16 shrink-0 place-content-center rounded-2xl bg-primary text-center text-white">
              <span className="text-xs font-semibold tracking-[0.1em]">{date.month}</span>
              <span className="font-display text-2xl font-bold leading-none">{date.day}</span>
            </div>
          ) : null}
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap gap-2">
              <Badge onDark tone={event.status === 'published' ? 'success' : event.status === 'cancelled' ? 'error' : 'muted'}>{event.lifecycle ?? event.status}</Badge>
              {event.visibility !== 'public' ? <Badge onDark>{event.visibility}</Badge> : null}
              {event.paidSalesStatus === 'active' ? <Badge onDark tone="success">{t('Paid sales on')}</Badge> : null}
              {event.suspended ? <Badge onDark tone="error">{t('Suspended by BiletFlow')}</Badge> : null}
            </div>
            <h1 className="mt-3 text-2xl font-bold text-surface sm:text-3xl">{event.title}</h1>
            <p className="mt-2 text-mist">{dateTime(event.startsAt, event.timezone)}{event.venueName ? ` · ${event.venueName}` : ''}{event.city ? `, ${event.city}` : ''}</p>
          </div>
        </div>
        {event.suspended && event.suspensionReason ? <p className="mt-4 rounded-input bg-[#4a2230] p-3 text-sm text-[#ffb4a8]">{t('Reason')}: {event.suspensionReason}</p> : null}
        <div className="mt-5 flex flex-wrap gap-2">
          {event.status === 'draft' ? <Button loading={busy === 'publish'} onClick={() => void act('publish', () => call(api.POST('/api/events/{eventId}/publish', { params: { path: { eventId: id } } })))}>{t('Publish')}</Button> : null}
          {event.status === 'published' ? <Button variant="onDark" loading={busy === 'unpublish'} onClick={() => void act('unpublish', () => call(api.POST('/api/events/{eventId}/unpublish', { params: { path: { eventId: id } } })))}>{t('Unpublish')}</Button> : null}
          <Link href={`/events/${event.id}${event.visibility === 'private' && event.accessToken ? `?key=${event.accessToken}` : ''}`} className="inline-flex min-h-9 items-center rounded-full border border-navy-line px-3 text-sm font-semibold text-mist hover:bg-navy-raised hover:text-white">{t(event.status === 'draft' ? 'Preview' : 'Event page')}</Link>
          <Button variant="onDark" size="sm" loading={busy === 'duplicate'} onClick={() => void act('duplicate', async () => {
            const copy = await call(api.POST('/api/events/{eventId}/duplicate', { params: { path: { eventId: id } } }));
            router.push(`/dashboard/events/${copy.id}?tab=details`);
          })}>{t('Duplicate')}</Button>
          {event.status !== 'cancelled' && event.status !== 'draft' ? <Button variant="onDark" size="sm" onClick={() => setCancelling((v) => !v)}>{t('Cancel event')}</Button> : null}
          {event.status === 'draft' ? <Button variant="onDark" size="sm" loading={busy === 'delete'} onClick={() => {
            if (!window.confirm(t('Delete this draft? This cannot be undone.'))) return;
            void act('delete', async () => { await callEmpty(api.DELETE('/api/events/{eventId}', { params: { path: { eventId: id } } })); router.push('/dashboard/events'); });
          }}>{t('Delete draft')}</Button> : null}
        </div>
        {event.status === 'published' && shareUrl ? (
          <div className="mt-4 grid gap-1 text-sm">
            <label htmlFor="share-link" className="text-mist">{t('Share link')}</label>
            <input id="share-link" readOnly value={shareUrl} onFocus={(e) => e.currentTarget.select()} className="min-h-10 rounded-input border border-navy-line bg-navy-raised px-3 text-white" />
          </div>
        ) : null}
      </div>
      {actionError ? <ErrorMessage message={actionError} /> : null}
      {cancelling ? (
        <Form onSubmit={() => act('cancel', async () => {
          const result = await call(api.POST('/api/events/{eventId}/cancel', { params: { path: { eventId: id } }, body: { reason } }));
          setCancelling(false);
          return result;
        })} className="grid gap-3 rounded-card border border-error/30 bg-error/5 p-5">
          <p className="font-semibold text-error">{t('Cancelling refunds every paid order, cancels free registrations and notifies all attendees. This cannot be undone.')}</p>
          <TextArea label="Reason shown to attendees" required value={reason} onChange={(e) => setReason(e.currentTarget.value)} />
          <div className="flex gap-2"><Button type="submit" variant="danger" loading={busy === 'cancel'} disabled={!reason.trim()}>{t('Cancel event and refund')}</Button><Button variant="quiet" onClick={() => setCancelling(false)}>{t('Keep the event')}</Button></div>
        </Form>
      ) : null}
      <Tabs tabs={TABS} active={tab} onChange={setTab} label="Event management" />
      <div role="tabpanel">
        {tab === 'overview' ? <AnalyticsPanel event={event} onGo={setTab} /> : null}
        {tab === 'details' ? <DetailsPanel event={event} onSaved={(e) => void mutate(e, { revalidate: false })} /> : null}
        {tab === 'tickets' ? <TicketTypesPanel event={event} onChange={() => void mutate()} /> : null}
        {tab === 'paid' ? <ActivationPanel event={event} onChange={() => void mutate()} /> : null}
        {tab === 'orders' ? <OrdersPanel event={event} /> : null}
        {tab === 'promo' ? <PromoPanel event={event} /> : null}
        {tab === 'staff' ? <StaffPanel event={event} /> : null}
        {tab === 'history' ? <TimelinePanel event={event} /> : null}
      </div>
    </section>
  );
}
