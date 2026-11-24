'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent, OrderSummary } from '@/lib/types';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Panel } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';

const TONE: Record<string, 'default' | 'success' | 'muted' | 'error'> = { completed: 'success', refunded: 'muted', cancelled: 'muted' };

/** Orders with refund (paid) and cancel (free) actions; every action is audited (SRS §4.9). */
export function OrdersPanel({ event }: { event: ApiEvent }) {
  const { t, money, dateTime } = useI18n();
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<'' | 'completed' | 'refunded' | 'cancelled'>('');
  const { data, error, isLoading, mutate } = useApi<OrderSummary[]>(`event-orders:${event.id}:${q}:${status}`,
    () => call(api.GET('/api/events/{eventId}/orders', { params: { path: { eventId: event.id }, query: { q, status } } })));
  const [refunding, setRefunding] = useState<string | null>(null);
  const [reason, setReason] = useState('');
  const [actionError, setActionError] = useState('');
  const [busy, setBusy] = useState(false);

  async function refund(order: OrderSummary) {
    setBusy(true); setActionError('');
    try {
      if (order.totalKzt > 0) await call(api.POST('/api/orders/{orderId}/refund', { params: { path: { orderId: order.id } }, body: { reason } }));
      else await call(api.POST('/api/orders/{orderId}/cancel', { params: { path: { orderId: order.id } } }));
      setRefunding(null); setReason('');
      await mutate();
    } catch (e) { setActionError(getApiErrorMessage(e)); } finally { setBusy(false); }
  }

  return (
    <Panel title="Orders">
      <div className="mb-4 flex flex-wrap gap-2">
        <label className="min-w-60 flex-1"><span className="sr-only">{t('Search orders')}</span>
          <input type="search" value={q} onChange={(e) => setQ(e.currentTarget.value)} placeholder={t('Order number, name or e-mail')} className="min-h-11 w-full rounded-input border border-border bg-surface px-4 text-sm" />
        </label>
        <select aria-label={t('Filter by status')} value={status} onChange={(e) => setStatus(e.currentTarget.value as typeof status)} className="min-h-11 rounded-input border border-border bg-surface px-3 text-sm">
          <option value="">{t('All statuses')}</option><option value="completed">{t('Completed')}</option><option value="refunded">{t('Refunded')}</option><option value="cancelled">{t('Cancelled')}</option>
        </select>
      </div>
      <p className="mb-3 text-sm text-text-muted">{t('Refund policy')}: {t(event.refundPolicy ?? '')}</p>
      {actionError ? <ErrorMessage message={actionError} className="mb-3" /> : null}
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (data ?? []).length === 0 ? <p className="text-sm text-text-muted">{t('No orders yet.')}</p> : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[40rem] text-left text-sm">
            <thead className="text-xs uppercase tracking-wide text-text-muted"><tr><th className="py-2">{t('Order')}</th><th>{t('Buyer')}</th><th>{t('Tickets')}</th><th>{t('Total')}</th><th>{t('Status')}</th><th><span className="sr-only">{t('Actions')}</span></th></tr></thead>
            <tbody>
              {(data ?? []).map((o) => (
                <tr key={o.id} className="border-t border-border align-top">
                  <td className="py-3"><Link href={`/orders/${o.id}`} className="font-semibold text-primary">{o.number}</Link><div className="text-xs text-text-muted">{dateTime(o.createdAt, event.timezone, false)}</div></td>
                  <td className="py-3">{o.buyerName}<div className="text-xs text-text-muted">{o.buyerEmail}</div></td>
                  <td className="py-3">{o.tickets}</td>
                  <td className="py-3">{o.totalKzt ? money(o.totalKzt) : t('Free')}</td>
                  <td className="py-3"><Badge tone={TONE[o.status] ?? 'default'}>{o.status}</Badge></td>
                  <td className="py-3 text-right">
                    {o.status === 'completed' ? (refunding === o.id ? (
                      <div className="grid gap-2 text-left">
                        {o.totalKzt > 0 ? <input aria-label={t('Reason for the refund')} value={reason} onChange={(e) => setReason(e.currentTarget.value)} placeholder={t('Reason for the refund')} className="min-h-9 rounded-input border border-border px-2" /> : <p className="text-xs">{t('Cancel this free registration?')}</p>}
                        <div className="flex gap-1"><Button size="sm" variant="danger" loading={busy} disabled={o.totalKzt > 0 && !reason.trim()} onClick={() => void refund(o)}>{t(o.totalKzt > 0 ? 'Refund' : 'Cancel registration')}</Button><Button size="sm" variant="quiet" onClick={() => setRefunding(null)}>{t('Back')}</Button></div>
                      </div>
                    ) : <Button size="sm" variant="secondary" onClick={() => { setRefunding(o.id); setActionError(''); }}>{t(o.totalKzt > 0 ? 'Refund…' : 'Cancel…')}</Button>) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <p className="mt-3 text-xs text-text-muted">{t('Refunds are full and simulated; they are not possible once a ticket in the order was used.')}</p>
    </Panel>
  );
}
