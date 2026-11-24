'use client';

import { useState } from 'react';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent, TimelineEntry } from '@/lib/types';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Panel } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';

const ACTION_LABEL: Record<string, string> = {
  'event.created': 'Event created', 'event.updated': 'Event details changed', 'event.published': 'Event published', 'event.unpublished': 'Event unpublished',
  'event.duplicated': 'Created as a copy', 'event.cancelled': 'Event cancelled', 'event.deleted': 'Draft deleted',
  'ticket_type.created': 'Ticket type added', 'ticket_type.updated': 'Ticket type changed', 'ticket_type.deleted': 'Ticket type removed',
  'order.paid': 'Paid order', 'order.completed': 'Free registration', 'order.payment_failed': 'Payment failed', 'order.refunded': 'Order refunded',
  'order.cancelled': 'Registration cancelled', 'activation.terms_accepted': 'Paid-sales terms accepted', 'activation.fee_paid': 'Activation fee paid',
  'activation.activated': 'Paid sales activated', 'campaign.created': 'Promo campaign created', 'campaign.updated': 'Promo campaign changed',
  'staff.assigned': 'Event Admin added', 'staff.removed': 'Event Admin removed', 'checkin.undone': 'Check-in undone',
  'moderation.suspended': 'Suspended by BiletFlow', 'moderation.unsuspended': 'Suspension lifted', 'moderation.paid_sales_suspend': 'Paid sales paused by BiletFlow',
  'moderation.paid_sales_resume': 'Paid sales resumed by BiletFlow',
};

const TYPES = [['', 'Everything'], ['event', 'Event changes'], ['ticket_type', 'Ticket types'], ['order', 'Orders and refunds'], ['activation', 'Paid sales'],
  ['campaign', 'Promo campaigns'], ['staff', 'Door staff'], ['checkin', 'Check-in'], ['moderation', 'Moderation']] as const;

/** Append-only history of the event (SRS §4.16); entries cannot be edited or deleted. */
export function TimelinePanel({ event }: { event: ApiEvent }) {
  const { t, dateTime } = useI18n();
  const [type, setType] = useState('');
  const { data, error, isLoading } = useApi<TimelineEntry[]>(`timeline:${event.id}:${type}`,
    () => call(api.GET('/api/events/{eventId}/timeline', { params: { path: { eventId: event.id }, query: type ? { type } : {} } })));
  return (
    <Panel title="Activity history" action={
      <select aria-label={t('Filter history')} value={type} onChange={(e) => setType(e.currentTarget.value)} className="min-h-10 rounded-input border border-border bg-surface px-3 text-sm">
        {TYPES.map(([v, l]) => <option key={v} value={v}>{t(l)}</option>)}
      </select>}>
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (data ?? []).length === 0 ? <p className="text-sm text-text-muted">{t('Nothing recorded yet.')}</p> : (
        <ol className="relative grid gap-4 border-l border-border pl-5">
          {(data ?? []).map((entry) => (
            <li key={entry.id} className="relative">
              <span aria-hidden="true" className="absolute -left-[1.6rem] top-1.5 size-2.5 rounded-full bg-primary" />
              <p className="text-sm font-semibold text-text">{ACTION_LABEL[entry.action] ? t(ACTION_LABEL[entry.action]) : entry.description}</p>
              {ACTION_LABEL[entry.action] && t(entry.description) !== t(ACTION_LABEL[entry.action]) ? <p className="text-sm text-text-muted">{t(entry.description)}</p> : null}
              <p className="text-xs text-text-muted">{dateTime(entry.createdAt, event.timezone, false)}{entry.actor ? ` · ${entry.actor}` : ''}</p>
            </li>
          ))}
        </ol>
      )}
      <p className="mt-4 text-xs text-text-muted">{t('This history is permanent: entries cannot be changed or deleted.')}</p>
    </Panel>
  );
}
