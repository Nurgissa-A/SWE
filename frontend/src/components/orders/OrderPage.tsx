'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { useApi } from '@/hooks/useApi';
import { api, downloadFile } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { Order } from '@/lib/types';
import { RequireSignIn } from '@/components/layout/RequireSignIn';
import { NewCaseForm } from '@/components/support/NewCaseForm';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { QRCode } from '@/components/ui/QRCode';
import { Spinner } from '@/components/ui/Spinner';

const TONE: Record<string, 'default' | 'success' | 'error' | 'muted'> = { completed: 'success', pending: 'default', refunded: 'muted', cancelled: 'muted', failed: 'error', expired: 'muted' };

export function OrderPage({ id, justBought }: { id: string; justBought: boolean }) {
  return <RequireSignIn><OrderDetails id={id} justBought={justBought} /></RequireSignIn>;
}

function OrderDetails({ id, justBought }: { id: string; justBought: boolean }) {
  const { t, money, dateTime } = useI18n();
  const { user } = useAuth();
  const { data: o, error, isLoading, mutate } = useApi<Order>(`order:${id}`, () => call(api.GET('/api/orders/{orderId}', { params: { path: { orderId: id } } })));
  const [support, setSupport] = useState(false);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState('');

  if (isLoading) return <main className="mx-auto max-w-4xl px-4 py-16"><Spinner /></main>;
  if (error || !o) return <main className="mx-auto max-w-4xl px-4 py-16"><ErrorMessage message={getApiErrorMessage(error)} /></main>;
  const tz = o.event.timezone;
  const isBuyer = o.event.organizerId !== user?.id;

  async function pdf(ticketId?: string) {
    setActionError('');
    try {
      await downloadFile(ticketId ? `/api/orders/${o!.id}/tickets/${ticketId}/pdf` : `/api/orders/${o!.id}/tickets.pdf`, `biletflow-${o!.number}${ticketId ? '-' + ticketId.slice(0, 8) : ''}.pdf`);
    } catch (e) { setActionError(getApiErrorMessage(e)); }
  }

  async function cancelRegistration() {
    if (!window.confirm(t('Cancel this registration? Your tickets will stop working.'))) return;
    setBusy(true); setActionError('');
    try { await mutate(await call(api.POST('/api/orders/{orderId}/cancel', { params: { path: { orderId: o!.id } } })), { revalidate: false }); }
    catch (e) { setActionError(getApiErrorMessage(e)); } finally { setBusy(false); }
  }

  return (
    <main className="mx-auto grid max-w-4xl gap-6 px-4 py-8 sm:px-6 lg:py-12">
      <Link href={isBuyer ? '/orders' : `/dashboard/events/${o.event.id}?tab=orders`} className="font-semibold text-primary hover:text-primary-hover">← {t(isBuyer ? 'My tickets' : 'Back to orders')}</Link>
      {justBought && o.status === 'completed' ? (
        <p role="status" className="rounded-card border border-success/30 bg-success/10 p-4 font-semibold text-success">
          {t('You are going! Your tickets were also sent to {email}.', { email: o.buyerEmail })}
        </p>
      ) : null}
      <section className="rounded-card bg-navy p-6 text-surface">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-sm text-mist">{t('Order')} {o.number}</p>
            <h1 className="mt-1 text-2xl font-bold text-surface">{o.event.title}</h1>
            <p className="mt-2 text-mist">{dateTime(o.event.startsAt, tz)}{o.event.venueName ? ` · ${o.event.venueName}` : ''}{o.event.city ? `, ${o.event.city}` : ''}</p>
          </div>
          <Badge tone={TONE[o.status]} onDark>{o.status}</Badge>
        </div>
        {o.event.status === 'cancelled' ? <p className="mt-4 rounded-input bg-[#4a2230] p-3 text-sm text-[#ffb4a8]">{t('This event was cancelled.')}</p> : null}
      </section>

      {actionError ? <ErrorMessage message={actionError} /> : null}

      {o.tickets.length ? (
        <section className="grid gap-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-sans text-lg font-bold tracking-normal">{t('Tickets')}</h2>
            {o.status === 'completed' && isBuyer ? <Button variant="secondary" onClick={() => void pdf()}>{t('Download all (PDF)')}</Button> : null}
          </div>
          <ul className="grid gap-4 sm:grid-cols-2">
            {o.tickets.map((tk) => (
              <li key={tk.id} className="grid gap-3 rounded-card border border-border bg-surface p-5">
                <div className="flex items-start justify-between gap-3">
                  <div><p className="font-semibold">{tk.ticketType}</p><p className="text-sm text-text-muted">{tk.attendeeName}</p></div>
                  <Badge tone={tk.status === 'valid' ? 'success' : tk.status === 'checked_in' ? 'default' : 'muted'}>{tk.status}</Badge>
                </div>
                {tk.qrPayload ? <div className="grid place-items-center"><QRCode value={tk.qrPayload} label={t('Admission QR code for ticket {code}', { code: tk.code })} /></div> : null}
                <p className="text-center font-display text-lg font-bold tracking-wider">{tk.code}</p>
                {tk.checkedInAt ? <p className="text-center text-sm text-text-muted">{t('Checked in {time}', { time: dateTime(tk.checkedInAt, tz, false) })}</p> : null}
                {tk.qrPayload ? <Button size="sm" variant="quiet" onClick={() => void pdf(tk.id)}>{t('Download this ticket (PDF)')}</Button> : null}
              </li>
            ))}
          </ul>
          {isBuyer && o.status === 'completed' ? <p className="text-sm text-text-muted">{t('Show the QR code on your phone or print the PDF. Both are the same ticket and admit one person once.')}</p> : null}
        </section>
      ) : null}

      <section className="grid gap-2 rounded-card border border-border bg-surface p-5">
        <h2 className="font-sans text-lg font-bold tracking-normal">{t('Summary')}</h2>
        <dl className="grid gap-1 text-sm">
          {o.items.map((it) => <div key={it.ticketTypeId} className="flex justify-between"><dt>{it.quantity} × {it.name}</dt><dd>{money(it.quantity * it.unitPriceKzt)}</dd></div>)}
          {o.discountKzt ? <div className="flex justify-between text-success"><dt>{t('Discount')}{o.promoCode ? ` (${o.promoCode})` : ''}</dt><dd>−{money(o.discountKzt)}</dd></div> : null}
          <div className="mt-1 flex justify-between border-t border-border pt-2 text-base font-bold"><dt>{t('Total')}</dt><dd>{money(o.totalKzt)}</dd></div>
        </dl>
        {o.totalKzt > 0 ? <p className="text-xs text-text-muted">{t('Demo payment: no real money was charged.')}</p> : null}
        <p className="mt-2 text-sm text-text-muted">{t('Refund policy')}: {t(o.event.refundPolicy)}</p>
      </section>

      {isBuyer ? (
        <div className="flex flex-wrap gap-2">
          {o.canCancel ? <Button variant="danger" loading={busy} onClick={() => void cancelRegistration()}>{t('Cancel my registration')}</Button> : null}
          <Button variant="secondary" onClick={() => setSupport((v) => !v)}>{t('Get help with this order')}</Button>
          <Link href={`/events/${o.event.id}`} className="inline-flex min-h-11 items-center rounded-full px-5 text-sm font-semibold text-primary">{t('Event page')}</Link>
        </div>
      ) : null}
      {support ? <NewCaseForm orderId={o.id} onDone={() => setSupport(false)} /> : null}
    </main>
  );
}
