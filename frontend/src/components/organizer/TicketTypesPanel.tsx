'use client';

import { useState } from 'react';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, callEmpty, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent, TicketType } from '@/lib/types';
import { utcToZoned, zonedToUtc } from '@/lib/zoned-time';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Panel } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';
import { TextInput } from '@/components/ui/TextInput';

type Draft = { name: string; description: string; priceKzt: string; quantity: string; maxPerOrder: string; salesStartAt: string; salesEndAt: string };
const empty: Draft = { name: '', description: '', priceKzt: '0', quantity: '100', maxPerOrder: '10', salesStartAt: '', salesEndAt: '' };

/** Free and paid ticket types with live counts (SRS §4.3). */
export function TicketTypesPanel({ event, onChange }: { event: ApiEvent; onChange: () => void }) {
  const { t, money } = useI18n();
  const tz = event.timezone || 'Asia/Almaty';
  const { data, error, isLoading, mutate } = useApi<TicketType[]>(`manage-types:${event.id}`,
    () => call(api.GET('/api/events/{eventId}/ticket-types', { params: { path: { eventId: event.id } } })));
  const [editing, setEditing] = useState<string | 'new' | null>(null);
  const [draft, setDraft] = useState<Draft>(empty);
  const [formError, setFormError] = useState('');
  const [busy, setBusy] = useState(false);
  const locked = event.status === 'cancelled';

  function edit(type?: TicketType) {
    setFormError('');
    setEditing(type ? type.id : 'new');
    setDraft(type ? {
      name: type.name, description: type.description, priceKzt: String(type.priceKzt), quantity: String(type.quantity),
      maxPerOrder: String(type.maxPerOrder), salesStartAt: utcToZoned(type.salesStartAt, tz), salesEndAt: utcToZoned(type.salesEndAt, tz),
    } : empty);
  }

  async function save() {
    setBusy(true); setFormError('');
    const body = {
      name: draft.name.trim(), description: draft.description.trim(), priceKzt: Number(draft.priceKzt || 0), quantity: Number(draft.quantity || 0),
      maxPerOrder: Number(draft.maxPerOrder || 10),
      salesStartAt: draft.salesStartAt ? zonedToUtc(draft.salesStartAt, tz) : null, salesEndAt: draft.salesEndAt ? zonedToUtc(draft.salesEndAt, tz) : null,
    };
    try {
      if (editing === 'new') await call(api.POST('/api/events/{eventId}/ticket-types', { params: { path: { eventId: event.id } }, body }));
      else if (editing) await call(api.PATCH('/api/events/{eventId}/ticket-types/{ticketTypeId}', { params: { path: { eventId: event.id, ticketTypeId: editing } }, body }));
      setEditing(null);
      await mutate();
      onChange();
    } catch (e) { setFormError(getApiErrorMessage(e)); } finally { setBusy(false); }
  }

  async function toggleHidden(type: TicketType) {
    try { await call(api.PATCH('/api/events/{eventId}/ticket-types/{ticketTypeId}', { params: { path: { eventId: event.id, ticketTypeId: type.id } }, body: { hidden: !type.hidden } })); await mutate(); }
    catch (e) { setFormError(getApiErrorMessage(e)); }
  }

  async function remove(type: TicketType) {
    if (!window.confirm(t('Delete the ticket type "{name}"?', { name: type.name }))) return;
    try { await callEmpty(api.DELETE('/api/events/{eventId}/ticket-types/{ticketTypeId}', { params: { path: { eventId: event.id, ticketTypeId: type.id } } })); await mutate(); onChange(); }
    catch (e) { setFormError(getApiErrorMessage(e)); }
  }

  const form = (
    <Form onSubmit={save} className="grid gap-4 rounded-card border border-border bg-background p-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <TextInput label="Name" required value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.currentTarget.value })} placeholder="Standard, VIP, Student…" />
        <TextInput label="Price (KZT)" type="number" min="0" step="1" value={draft.priceKzt} onChange={(e) => setDraft({ ...draft, priceKzt: e.currentTarget.value })} helpText="0 makes it a free ticket." />
        <TextInput label="Quantity" type="number" min="0" step="1" value={draft.quantity} onChange={(e) => setDraft({ ...draft, quantity: e.currentTarget.value })} />
        <TextInput label="Maximum per order" type="number" min="1" max="50" step="1" value={draft.maxPerOrder} onChange={(e) => setDraft({ ...draft, maxPerOrder: e.currentTarget.value })} />
        <TextInput label="Sales start (optional)" type="datetime-local" value={draft.salesStartAt} onChange={(e) => setDraft({ ...draft, salesStartAt: e.currentTarget.value })} />
        <TextInput label="Sales end (optional)" type="datetime-local" value={draft.salesEndAt} onChange={(e) => setDraft({ ...draft, salesEndAt: e.currentTarget.value })} />
      </div>
      <TextInput label="Description (optional)" value={draft.description} onChange={(e) => setDraft({ ...draft, description: e.currentTarget.value })} />
      {formError ? <ErrorMessage message={formError} /> : null}
      <div className="flex gap-2"><Button type="submit" loading={busy}>{t(editing === 'new' ? 'Add ticket type' : 'Save changes')}</Button><Button variant="quiet" onClick={() => setEditing(null)}>{t('Cancel')}</Button></div>
    </Form>
  );

  return (
    <Panel title="Ticket types" action={!locked && editing === null ? <Button size="sm" onClick={() => edit()}>{t('Add ticket type')}</Button> : null}>
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (
        <div className="grid gap-4">
          {editing === 'new' ? form : null}
          {(data ?? []).length === 0 && editing !== 'new' ? <p className="text-sm text-text-muted">{t('Add at least one ticket type before publishing. Free events use a ticket type with price 0.')}</p> : null}
          <ul className="grid gap-3">
            {(data ?? []).map((type) => (
              <li key={type.id} className="grid gap-3 rounded-card border border-border p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="font-semibold">{type.name} {type.hidden ? <Badge tone="muted">{t('Hidden')}</Badge> : null}</p>
                    <p className="text-sm text-text-muted">{type.priceKzt ? money(type.priceKzt) : t('Free')} · {t('max {n} per order', { n: type.maxPerOrder })}</p>
                  </div>
                  {!locked ? (
                    <div className="flex flex-wrap gap-2">
                      <Button size="sm" variant="secondary" onClick={() => edit(type)}>{t('Edit')}</Button>
                      <Button size="sm" variant="quiet" onClick={() => void toggleHidden(type)}>{t(type.hidden ? 'Show' : 'Hide')}</Button>
                      <Button size="sm" variant="quiet" onClick={() => void remove(type)}>{t('Delete')}</Button>
                    </div>
                  ) : null}
                </div>
                <dl className="grid grid-cols-2 gap-2 text-sm sm:grid-cols-5">
                  {[['Quantity', type.quantity], ['Available', type.available], ['Reserved', type.counts?.reserved ?? 0], ['Sold', type.counts?.sold ?? 0], ['Checked in', type.counts?.checkedIn ?? 0]].map(([label, value]) => (
                    <div key={label as string} className="rounded-input bg-background p-2"><dt className="text-xs text-text-muted">{t(label as string)}</dt><dd className="font-display font-bold">{value}</dd></div>
                  ))}
                </dl>
                {(type.counts?.refunded ?? 0) > 0 ? <p className="text-xs text-text-muted">{t('{n} refunded or cancelled', { n: type.counts?.refunded ?? 0 })}</p> : null}
                {editing === type.id ? form : null}
              </li>
            ))}
          </ul>
          {formError && editing === null ? <ErrorMessage message={formError} /> : null}
        </div>
      )}
    </Panel>
  );
}
