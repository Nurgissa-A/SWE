'use client';

import { useState } from 'react';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { AnalyticsReport, ApiEvent, TicketType } from '@/lib/types';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Panel, Stat } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';

/** Event dashboard computed from BiletFlow records only (SRS §4.15), with next steps. */
export function AnalyticsPanel({ event, onGo }: { event: ApiEvent; onGo: (tab: string) => void }) {
  const { t, money, number } = useI18n();
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [typeId, setTypeId] = useState('');
  const query = { ...(from ? { from } : {}), ...(to ? { to } : {}), ...(typeId ? { ticketTypeId: typeId } : {}) };
  const { data: r, error, isLoading } = useApi<AnalyticsReport>(`analytics:${event.id}:${from}:${to}:${typeId}`,
    () => call(api.GET('/api/events/{eventId}/analytics', { params: { path: { eventId: event.id }, query } })));
  const { data: types } = useApi<TicketType[]>(`manage-types:${event.id}`,
    () => call(api.GET('/api/events/{eventId}/ticket-types', { params: { path: { eventId: event.id } } })));
  const [view, setView] = useState<'chart' | 'table'>('chart');

  const steps = [
    { done: Boolean(event.venueName && event.city), label: 'Add the venue and city', tab: 'details' },
    { done: (types ?? []).length > 0, label: 'Add ticket types', tab: 'tickets' },
    { done: event.status !== 'draft', label: 'Publish the event', tab: 'overview' },
  ];
  const needsPaid = (types ?? []).some((x) => x.priceKzt > 0) && event.paidSalesStatus !== 'active';

  if (isLoading) return <Spinner />;
  if (error || !r) return <ErrorMessage message={getApiErrorMessage(error)} />;
  const s = r.summary;
  const max = Math.max(1, ...r.daily.map((d) => d.tickets));

  return (
    <div className="grid gap-6">
      {event.status === 'draft' || needsPaid ? (
        <Panel title="Next steps">
          <ol className="grid gap-2 text-sm">
            {steps.map((step) => (
              <li key={step.label} className="flex items-center gap-2">
                <span aria-hidden="true" className={`grid size-5 place-items-center rounded-full text-[0.65rem] ${step.done ? 'bg-success text-white' : 'border border-border'}`}>{step.done ? '✓' : ''}</span>
                {step.done ? <span className="text-text-muted line-through">{t(step.label)}</span> : <button type="button" onClick={() => onGo(step.tab)} className="font-semibold text-primary">{t(step.label)}</button>}
              </li>
            ))}
            {needsPaid ? <li><button type="button" onClick={() => onGo('paid')} className="font-semibold text-primary">{t('Activate paid sales to sell paid tickets')}</button></li> : null}
          </ol>
        </Panel>
      ) : null}
      <div className="flex flex-wrap items-end gap-3 text-sm">
        <label className="grid gap-1">{t('From')}<input type="date" value={from} onChange={(e) => setFrom(e.currentTarget.value)} className="min-h-10 rounded-input border border-border bg-surface px-3" /></label>
        <label className="grid gap-1">{t('To')}<input type="date" value={to} onChange={(e) => setTo(e.currentTarget.value)} className="min-h-10 rounded-input border border-border bg-surface px-3" /></label>
        <label className="grid gap-1">{t('Ticket type')}
          <select value={typeId} onChange={(e) => setTypeId(e.currentTarget.value)} className="min-h-10 rounded-input border border-border bg-surface px-3">
            <option value="">{t('All ticket types')}</option>{(types ?? []).map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
          </select>
        </label>
        {from || to || typeId ? <Button size="sm" variant="quiet" onClick={() => { setFrom(''); setTo(''); setTypeId(''); }}>{t('Clear filters')}</Button> : null}
        <p className="text-xs text-text-muted">{t('Sales dates use the payment day in {tz}.', { tz: r.timezone })}</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Stat label="Tickets sold" value={<>{number(s.sold)}<span className="text-base text-mist"> / {number(s.sold + s.remaining)}</span></>} hint={t('{p}% sold', { p: s.percentSold })} />
        <Stat label="Net sales" value={money(s.netKzt)} hint={t('Gross {g} − discounts − refunds', { g: money(s.grossKzt) })} />
        <Stat label="Checked in" value={number(s.checkedIn)} hint={t('{p}% attendance', { p: s.attendanceRate })} accent />
        <Stat label="Estimated payout" value={money(s.estimatedPayoutKzt)} hint={t('after {fee} processing fee (simulated)', { fee: money(s.feesKzt) })} />
      </div>
      <Panel title="Sales over time" action={
        <div role="group" aria-label={t('View')} className="flex rounded-full border border-border p-0.5 text-sm">
          {(['chart', 'table'] as const).map((v) => <button key={v} type="button" aria-pressed={view === v} onClick={() => setView(v)} className={`rounded-full px-3 py-1 ${view === v ? 'bg-navy text-white' : ''}`}>{t(v === 'chart' ? 'Chart' : 'Table')}</button>)}
        </div>}>
        {r.daily.length === 0 ? <p className="text-sm text-text-muted">{t('No sales in this period.')}</p> : view === 'chart' ? (
          <div role="group" aria-label={t('Tickets sold per day')} className="flex h-48 items-end gap-1.5 overflow-x-auto pb-6">
            {r.daily.map((d) => (
              <div key={d.day} role="img" aria-label={t('{day}: {n} tickets, {amount}', { day: d.day, n: d.tickets, amount: money(d.revenueKzt) })} className="relative flex h-full min-w-8 flex-1 flex-col justify-end">
                <div className="rounded-t-md bg-primary" style={{ height: `${(d.tickets / max) * 100}%` }} />
                <span className="absolute -bottom-5 left-1/2 -translate-x-1/2 whitespace-nowrap text-[0.65rem] text-text-muted">{d.day.slice(5)}</span>
              </div>
            ))}
          </div>
        ) : (
          <table className="w-full text-sm"><thead className="text-left text-xs uppercase text-text-muted"><tr><th>{t('Day')}</th><th>{t('Tickets')}</th><th>{t('Revenue')}</th></tr></thead>
            <tbody>{r.daily.map((d) => <tr key={d.day} className="border-t border-border"><td className="py-2">{d.day}</td><td>{d.tickets}</td><td>{money(d.revenueKzt)}</td></tr>)}</tbody></table>
        )}
      </Panel>
      <div className="grid gap-6 lg:grid-cols-2">
        <Panel title="By ticket type">
          <table className="w-full text-sm"><thead className="text-left text-xs uppercase text-text-muted"><tr><th>{t('Type')}</th><th>{t('Sold')}</th><th>{t('Refunded')}</th><th>{t('Checked in')}</th><th>{t('Revenue')}</th></tr></thead>
            <tbody>{r.byTicketType.map((x) => <tr key={x.id} className="border-t border-border"><td className="py-2">{x.name}</td><td>{x.sold}/{x.quantity}</td><td>{x.refunded}</td><td>{x.checkedIn}</td><td>{money(x.revenueKzt)}</td></tr>)}</tbody></table>
        </Panel>
        <Panel title="Promo campaigns">
          {r.campaigns.length === 0 ? <p className="text-sm text-text-muted">{t('No campaigns yet.')}</p> : (
            <table className="w-full text-sm"><thead className="text-left text-xs uppercase text-text-muted"><tr><th>{t('Code')}</th><th>{t('Visits')}</th><th>{t('Orders')}</th><th>{t('Revenue')}</th><th>{t('Discount')}</th></tr></thead>
              <tbody>{r.campaigns.map((c) => <tr key={c.id} className="border-t border-border"><td className="py-2 font-display">{c.code}</td><td>{c.visits}</td><td>{c.orders}</td><td>{money(c.revenueKzt)}</td><td>{money(c.discountKzt)}</td></tr>)}</tbody></table>
          )}
        </Panel>
      </div>
      <p className="text-xs text-text-muted">{t('All figures come from BiletFlow orders and check-ins. Payments are simulated.')}</p>
    </div>
  );
}
