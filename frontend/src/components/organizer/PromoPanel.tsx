'use client';

import { useState } from 'react';
import { useApi } from '@/hooks/useApi';
import { api, downloadFile } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent, Campaign, TicketType } from '@/lib/types';
import { zonedToUtc } from '@/lib/zoned-time';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { QRCode } from '@/components/ui/QRCode';
import { Select } from '@/components/ui/Select';
import { Panel } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';
import { TextInput } from '@/components/ui/TextInput';

/** Promo campaigns with server-calculated discounts and Campaign QR links (SRS §4.14). */
export function PromoPanel({ event }: { event: ApiEvent }) {
  const { t, money } = useI18n();
  const { data, error, isLoading, mutate } = useApi<Campaign[]>(`campaigns:${event.id}`,
    () => call(api.GET('/api/events/{eventId}/campaigns', { params: { path: { eventId: event.id } } })));
  const { data: types } = useApi<TicketType[]>(`manage-types:${event.id}`,
    () => call(api.GET('/api/events/{eventId}/ticket-types', { params: { path: { eventId: event.id } } })));
  const [creating, setCreating] = useState(false);
  const [showQr, setShowQr] = useState<string | null>(null);
  const [form, setForm] = useState({ name: '', code: '', discountType: 'percent' as 'percent' | 'fixed', discountValue: '10', maxRedemptions: '', validUntil: '', ticketTypeIds: [] as string[] });
  const [formError, setFormError] = useState('');
  const [busy, setBusy] = useState(false);
  const origin = typeof window === 'undefined' ? '' : window.location.origin;

  async function create() {
    setBusy(true); setFormError('');
    try {
      await call(api.POST('/api/events/{eventId}/campaigns', { params: { path: { eventId: event.id } }, body: {
        name: form.name, code: form.code, discountType: form.discountType, discountValue: Number(form.discountValue),
        maxRedemptions: form.maxRedemptions ? Number(form.maxRedemptions) : null,
        validUntil: form.validUntil ? zonedToUtc(form.validUntil, event.timezone || 'Asia/Almaty') : null,
        ticketTypeIds: form.ticketTypeIds,
      } }));
      setCreating(false);
      await mutate();
    } catch (e) { setFormError(getApiErrorMessage(e)); } finally { setBusy(false); }
  }

  async function toggle(c: Campaign) {
    await call(api.PATCH('/api/events/{eventId}/campaigns/{campaignId}', { params: { path: { eventId: event.id, campaignId: c.id } }, body: { status: c.status === 'active' ? 'disabled' : 'active' } }))
      .then(() => mutate()).catch((e) => setFormError(getApiErrorMessage(e)));
  }

  return (
    <Panel title="Promo codes" action={!creating ? <Button size="sm" onClick={() => setCreating(true)}>{t('New campaign')}</Button> : null}>
      <div className="grid gap-4">
        {creating ? (
          <Form onSubmit={create} className="grid gap-4 rounded-card border border-border bg-background p-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <TextInput label="Campaign name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.currentTarget.value })} placeholder="Instagram, Students…" />
              <TextInput label="Code" required value={form.code} onChange={(e) => setForm({ ...form, code: e.currentTarget.value.toUpperCase() })} helpText="3–30 letters, digits, - or _." />
              <Select label="Discount type" value={form.discountType} onChange={(e) => setForm({ ...form, discountType: e.currentTarget.value as 'percent' | 'fixed' })}>
                <option value="percent">{t('Percent off each ticket')}</option><option value="fixed">{t('Fixed amount off each ticket (KZT)')}</option>
              </Select>
              <TextInput label={form.discountType === 'percent' ? 'Percent' : 'Amount (KZT)'} type="number" min="1" max={form.discountType === 'percent' ? 100 : undefined} value={form.discountValue} onChange={(e) => setForm({ ...form, discountValue: e.currentTarget.value })} />
              <TextInput label="Redemption limit (optional)" type="number" min="1" value={form.maxRedemptions} onChange={(e) => setForm({ ...form, maxRedemptions: e.currentTarget.value })} />
              <TextInput label="Valid until (optional)" type="datetime-local" value={form.validUntil} onChange={(e) => setForm({ ...form, validUntil: e.currentTarget.value })} />
            </div>
            {(types ?? []).length > 1 ? (
              <fieldset className="grid gap-2 text-sm"><legend className="mb-1 font-semibold">{t('Applies to (none selected = all ticket types)')}</legend>
                {(types ?? []).map((tt) => (
                  <label key={tt.id} className="flex items-center gap-2"><input type="checkbox" checked={form.ticketTypeIds.includes(tt.id)}
                    onChange={(e) => setForm({ ...form, ticketTypeIds: e.currentTarget.checked ? [...form.ticketTypeIds, tt.id] : form.ticketTypeIds.filter((x) => x !== tt.id) })} />{tt.name}</label>
                ))}
              </fieldset>
            ) : null}
            {formError ? <ErrorMessage message={formError} /> : null}
            <div className="flex gap-2"><Button type="submit" loading={busy}>{t('Create campaign')}</Button><Button variant="quiet" onClick={() => setCreating(false)}>{t('Cancel')}</Button></div>
          </Form>
        ) : null}
        {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (data ?? []).length === 0 && !creating ? <p className="text-sm text-text-muted">{t('No campaigns yet. A campaign gives a promo code and a QR code that opens this event with the code applied.')}</p> : null}
        <ul className="grid gap-3">
          {(data ?? []).map((c) => {
            const link = `${origin}/c/${c.linkToken}`;
            return (
              <li key={c.id} className="grid gap-3 rounded-card border border-border p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="font-semibold">{c.name} · <span className="font-display">{c.code}</span> <Badge tone={c.status === 'active' ? 'success' : 'muted'}>{c.status}</Badge></p>
                    <p className="text-sm text-text-muted">{c.discountType === 'percent' ? t('{n}% off', { n: c.discountValue }) : t('{amount} off', { amount: money(c.discountValue) })}
                      {' · '}{c.maxRedemptions ? t('{used} of {max} used', { used: c.redemptionsCount, max: c.maxRedemptions }) : t('{used} used', { used: c.redemptionsCount })}</p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button size="sm" variant="secondary" onClick={() => setShowQr(showQr === c.id ? null : c.id)}>{t('Campaign QR')}</Button>
                    <Button size="sm" variant="quiet" onClick={() => void toggle(c)}>{t(c.status === 'active' ? 'Disable' : 'Enable')}</Button>
                  </div>
                </div>
                <dl className="grid grid-cols-2 gap-2 text-sm sm:grid-cols-4">
                  {[['Link visits', String(c.linkVisits)], ['Orders', String(c.results?.orders ?? 0)], ['Revenue', money(c.results?.revenueKzt ?? 0)], ['Discounts given', money(c.results?.discountKzt ?? 0)]].map(([label, value]) => (
                    <div key={label} className="rounded-input bg-background p-2"><dt className="text-xs text-text-muted">{t(label)}</dt><dd className="font-bold">{value}</dd></div>
                  ))}
                </dl>
                {showQr === c.id ? (
                  <div className="flex flex-wrap items-center gap-4 rounded-input bg-background p-3">
                    <QRCode value={link} size={160} label={t('Campaign QR code for {code}', { code: c.code })} />
                    <div className="grid gap-2 text-sm">
                      <p className="break-all text-text-muted">{link}</p>
                      <p className="text-xs text-text-muted">{t('This QR opens the event page with the code applied. It is not a ticket and is refused at the door.')}</p>
                      <Button size="sm" variant="secondary" onClick={() => void downloadFile(`/api/events/${event.id}/campaigns/${c.id}/qr.png`, `campaign-${c.code}.png`)}>{t('Download PNG')}</Button>
                    </div>
                  </div>
                ) : null}
              </li>
            );
          })}
        </ul>
      </div>
    </Panel>
  );
}
