'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useApi } from '@/hooks/useApi';
import { api, authApi, downloadFile } from '@/lib/api-client';
import { call, callEmpty, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { AdminRow, AdminUser } from '@/lib/types';
import { RequireSignIn } from '@/components/layout/RequireSignIn';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Panel, Stat } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';
import { Tabs } from '@/components/ui/Tabs';
import { TextInput } from '@/components/ui/TextInput';

const TABS = [
  { id: 'overview', label: 'Overview' }, { id: 'events', label: 'Events' }, { id: 'users', label: 'Users' }, { id: 'orders', label: 'Orders' },
  { id: 'payments', label: 'Payments' }, { id: 'activations', label: 'Activations' }, { id: 'promo', label: 'Promo activity' },
  { id: 'reports', label: 'Reports' }, { id: 'settings', label: 'Settings' },
];

export default function AdminPage() {
  return <RequireSignIn><AdminConsole /></RequireSignIn>;
}

function AdminConsole() {
  const { t } = useI18n();
  const { user } = useAuth();
  const router = useRouter();
  const [tab, setTab] = useState('overview');
  useEffect(() => { if (user && user.role !== 'platform_admin') router.replace('/'); }, [user, router]);
  if (user?.role !== 'platform_admin') return null;
  return (
    <main className="mx-auto grid max-w-7xl gap-6 px-4 py-8 sm:px-6 lg:px-8 lg:py-12">
      <div>
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">BiletFlow</p>
        <h1 className="mt-2 text-3xl font-bold text-text">{t('Platform administration')}</h1>
        <p className="mt-2 text-text-muted">{t('Escalated and organizer support cases are in')} <Link href="/support" className="font-semibold text-primary">{t('Support')}</Link>.</p>
      </div>
      <Tabs tabs={TABS} active={tab} onChange={setTab} label="Administration" />
      {tab === 'overview' ? <Overview /> : null}
      {tab === 'events' ? <Events /> : null}
      {tab === 'users' ? <Users /> : null}
      {tab === 'orders' ? <Orders /> : null}
      {tab === 'payments' ? <Payments /> : null}
      {tab === 'activations' ? <Activations /> : null}
      {tab === 'promo' ? <Promo /> : null}
      {tab === 'reports' ? <Reports /> : null}
      {tab === 'settings' ? <Settings /> : null}
    </main>
  );
}

const str = (v: unknown) => (v == null ? '' : String(v));
const num = (v: unknown) => Number(v ?? 0);

function useRows(key: string, path: string, query: Record<string, string> = {}) {
  return useApi<AdminRow[]>(`admin:${key}:${JSON.stringify(query)}`, async () => {
    // Admin list routes share one shape; the typed client is used with a cast for brevity.
    const get = api.GET as unknown as (p: string, o: object) => Promise<{ data?: AdminRow[]; error?: unknown; response: Response }>;
    return call(get(path, { params: { query } }));
  });
}

function Search({ value, onChange, placeholder }: { value: string; onChange: (v: string) => void; placeholder: string }) {
  const { t } = useI18n();
  return <input type="search" aria-label={t(placeholder)} placeholder={t(placeholder)} value={value} onChange={(e) => onChange(e.currentTarget.value)} className="mb-4 min-h-11 w-full max-w-md rounded-input border border-border bg-surface px-4 text-sm" />;
}

function Overview() {
  const { money, number } = useI18n();
  const { data, error, isLoading } = useApi<AdminRow>('admin:summary', () => call(api.GET('/api/admin/summary')) as Promise<AdminRow>);
  if (isLoading) return <Spinner />;
  if (error || !data) return <ErrorMessage message={getApiErrorMessage(error)} />;
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Stat label="Published events" value={number(num(data.publishedEvents))} />
      <Stat label="Completed orders" value={number(num(data.completedOrders))} />
      <Stat label="Ticket volume (simulated)" value={money(num(data.ticketVolumeKzt))} />
      <Stat label="Refunds (simulated)" value={money(num(data.refundsKzt))} />
      <Stat label="Failed payments, 7 days" value={number(num(data.failedPayments7d))} />
      <Stat label="Suspended events" value={number(num(data.suspendedEvents))} />
      <Stat label="Open reports" value={number(num(data.openReports))} accent={num(data.openReports) > 0} />
      <Stat label="Open BiletFlow cases" value={number(num(data.openCases))} accent={num(data.openCases) > 0} />
    </div>
  );
}

function Events() {
  const { t, dateTime } = useI18n();
  const [q, setQ] = useState('');
  const { data, error, isLoading, mutate } = useRows('events', '/api/admin/events', { q });
  const [acting, setActing] = useState<string | null>(null);
  const [reason, setReason] = useState('');
  const [actionError, setActionError] = useState('');
  async function run(fn: () => Promise<void>) {
    setActionError('');
    try { await fn(); setActing(null); setReason(''); await mutate(); } catch (e) { setActionError(getApiErrorMessage(e)); }
  }
  return (
    <Panel title="Events">
      <Search value={q} onChange={setQ} placeholder="Title, city or organizer e-mail" />
      {actionError ? <ErrorMessage message={actionError} className="mb-3" /> : null}
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (
        <ul className="grid gap-2">
          {(data ?? []).map((e) => {
            const id = str(e.id);
            return (
              <li key={id} className="grid gap-2 rounded-card border border-border p-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="min-w-0">
                    <Link href={`/events/${id}`} className="font-semibold text-primary">{str(e.title)}</Link>
                    <p className="text-sm text-text-muted">{dateTime(str(e.startsAt))} · {str(e.city)} · {str(e.organizer)} · {t('{n} sold', { n: num(e.sold) })}</p>
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    <Badge tone={e.status === 'published' ? 'success' : 'muted'}>{str(e.status)}</Badge>
                    {e.suspended ? <Badge tone="error">{t('Suspended')}</Badge> : null}
                    {e.paidSalesStatus !== 'inactive' ? <Badge>{t('Paid sales: {s}', { s: t(str(e.paidSalesStatus)) })}</Badge> : null}
                    {num(e.openReports) ? <Badge tone="error">{t('{n} reports', { n: num(e.openReports) })}</Badge> : null}
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  {e.suspended
                    ? <Button size="sm" variant="secondary" onClick={() => void run(() => callEmpty(api.POST('/api/admin/events/{eventId}/unsuspend', { params: { path: { eventId: id } } })))}>{t('Lift suspension')}</Button>
                    : <Button size="sm" variant="secondary" onClick={() => setActing(acting === id ? null : id)}>{t('Suspend…')}</Button>}
                  {e.paidSalesStatus === 'active' ? <Button size="sm" variant="quiet" onClick={() => void run(() => callEmpty(api.POST('/api/admin/events/{eventId}/paid-sales', { params: { path: { eventId: id } }, body: { action: 'suspend' } })))}>{t('Pause paid sales')}</Button> : null}
                  {e.paidSalesStatus === 'suspended' ? <Button size="sm" variant="quiet" onClick={() => void run(() => callEmpty(api.POST('/api/admin/events/{eventId}/paid-sales', { params: { path: { eventId: id } }, body: { action: 'resume' } })))}>{t('Resume paid sales')}</Button> : null}
                </div>
                {acting === id ? (
                  <Form onSubmit={() => run(() => callEmpty(api.POST('/api/admin/events/{eventId}/suspend', { params: { path: { eventId: id } }, body: { reason } })))} className="flex flex-wrap items-end gap-2">
                    <div className="min-w-64 flex-1"><TextInput label="Reason sent to the organizer" required value={reason} onChange={(ev) => setReason(ev.currentTarget.value)} /></div>
                    <Button type="submit" variant="danger" disabled={!reason.trim()}>{t('Suspend event')}</Button>
                  </Form>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </Panel>
  );
}

function Users() {
  const { t, date } = useI18n();
  const [q, setQ] = useState('');
  const { data, error, isLoading, mutate } = useApi<AdminUser[]>(`admin:users:${q}`, () => call((authApi as any).GET('/api/admin/users', { params: { query: { q } } })));
  const [actionError, setActionError] = useState('');
  return (
    <Panel title="Users">
      <Search value={q} onChange={setQ} placeholder="E-mail or name" />
      <p className="mb-3 text-xs text-text-muted">{t('Suspending an account signs it out everywhere at once. Administrator roles are managed by operators, not here.')}</p>
      {actionError ? <ErrorMessage message={actionError} className="mb-3" /> : null}
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (
        <div className="overflow-x-auto"><table className="w-full min-w-[36rem] text-left text-sm">
          <thead className="text-xs uppercase text-text-muted"><tr><th className="py-2">{t('Account')}</th><th>{t('Role')}</th><th>{t('Joined')}</th><th>{t('Status')}</th><th><span className="sr-only">{t('Actions')}</span></th></tr></thead>
          <tbody>{(data ?? []).map((u) => (
            <tr key={u.id} className="border-t border-border">
              <td className="py-2">{u.displayName || '—'}<div className="text-xs text-text-muted">{u.email}</div></td>
              <td>{t(u.role.replace('_', ' ').replace(/^./, (c: string) => c.toUpperCase()))}</td>
              <td>{date(u.createdAt)}</td>
              <td><Badge tone={u.isActive ? 'success' : 'error'}>{u.isActive ? 'active' : 'suspended'}</Badge></td>
              <td className="text-right">{u.role !== 'platform_admin' ? (
                <Button size="sm" variant={u.isActive ? 'quiet' : 'secondary'} onClick={() => void call((authApi as any).PATCH('/api/admin/users/{userId}', { params: { path: { userId: u.id } }, body: { isActive: !u.isActive } }))
                  .then(() => mutate()).catch((e) => setActionError(getApiErrorMessage(e)))}>{t(u.isActive ? 'Suspend' : 'Reactivate')}</Button>
              ) : null}</td>
            </tr>
          ))}</tbody>
        </table></div>
      )}
    </Panel>
  );
}

function Orders() {
  const { t, money, date } = useI18n();
  const [q, setQ] = useState('');
  const { data, error, isLoading } = useRows('orders', '/api/admin/orders', { q });
  return (
    <Panel title="Orders" action={<Button size="sm" variant="secondary" onClick={() => void downloadFile('/api/admin/export/orders.csv', 'biletflow-orders.csv')}>{t('Export CSV')}</Button>}>
      <Search value={q} onChange={setQ} placeholder="Order number, buyer or event" />
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (
        <div className="overflow-x-auto"><table className="w-full min-w-[40rem] text-left text-sm">
          <thead className="text-xs uppercase text-text-muted"><tr><th className="py-2">{t('Order')}</th><th>{t('Event')}</th><th>{t('Buyer')}</th><th>{t('Total')}</th><th>{t('Status')}</th></tr></thead>
          <tbody>{(data ?? []).map((o) => (
            <tr key={str(o.id)} className="border-t border-border">
              <td className="py-2"><Link href={`/orders/${str(o.id)}`} className="font-semibold text-primary">{str(o.number)}</Link><div className="text-xs text-text-muted">{date(str(o.createdAt))}</div></td>
              <td>{str(o.eventTitle)}</td><td>{str(o.buyerName)}<div className="text-xs text-text-muted">{str(o.buyerEmail)}</div></td>
              <td>{money(num(o.totalKzt))}</td><td><Badge tone={o.status === 'completed' ? 'success' : 'muted'}>{str(o.status)}</Badge></td>
            </tr>
          ))}</tbody>
        </table></div>
      )}
    </Panel>
  );
}

function Payments() {
  const { t, money, dateTime } = useI18n();
  const [status, setStatus] = useState('');
  const { data, error, isLoading } = useRows('payments', '/api/admin/payments', status ? { status } : {});
  return (
    <Panel title="Payments and refunds" action={<div className="flex gap-2">
      <select aria-label={t('Filter by status')} value={status} onChange={(e) => setStatus(e.currentTarget.value)} className="min-h-9 rounded-input border border-border bg-surface px-3 text-sm">
        <option value="">{t('All payments')}</option><option value="succeeded">{t('Succeeded')}</option><option value="failed">{t('Failed')}</option>
      </select>
      <Button size="sm" variant="secondary" onClick={() => void downloadFile('/api/admin/export/payments.csv', 'biletflow-payments.csv')}>{t('Export CSV')}</Button></div>}>
      <p className="mb-3 text-xs text-text-muted">{t('Every payment is simulated; only the last four card digits are stored.')}</p>
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (
        <div className="overflow-x-auto"><table className="w-full min-w-[44rem] text-left text-sm">
          <thead className="text-xs uppercase text-text-muted"><tr><th className="py-2">{t('Time')}</th><th>{t('Kind')}</th><th>{t('Event')}</th><th>{t('Amount')}</th><th>{t('Fee')}</th><th>{t('Refunded')}</th><th>{t('Status')}</th></tr></thead>
          <tbody>{(data ?? []).map((p) => (
            <tr key={str(p.id)} className="border-t border-border">
              <td className="py-2">{dateTime(str(p.createdAt), undefined, false)}<div className="text-xs text-text-muted">{str(p.providerRef)} · •••• {str(p.cardLast4)}</div></td>
              <td>{t(p.kind === 'ticket' ? 'Tickets' : 'Activation fee')}{p.orderNumber ? <div className="text-xs text-text-muted">{str(p.orderNumber)}</div> : null}</td>
              <td>{str(p.eventTitle)}</td><td>{money(num(p.amountKzt))}</td><td>{money(num(p.feeKzt))}</td><td>{num(p.refundedKzt) ? money(num(p.refundedKzt)) : '—'}</td>
              <td><Badge tone={p.status === 'succeeded' ? 'success' : 'error'}>{str(p.status)}</Badge>{p.failureReason ? <div className="text-xs text-text-muted">{t(str(p.failureReason).replace(/_/g, ' ').toLowerCase().replace(/^./, (c) => c.toUpperCase()))}</div> : null}</td>
            </tr>
          ))}</tbody>
        </table></div>
      )}
    </Panel>
  );
}

function Activations() {
  const { t, money, date } = useI18n();
  const { data, error, isLoading } = useRows('activations', '/api/admin/activations');
  return (
    <Panel title="Paid-sales activations">
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (data ?? []).length === 0 ? <p className="text-sm text-text-muted">{t('No activations yet.')}</p> : (
        <div className="overflow-x-auto"><table className="w-full min-w-[44rem] text-left text-sm">
          <thead className="text-xs uppercase text-text-muted"><tr><th className="py-2">{t('Event')}</th><th>{t('Legal name')}</th><th>{t('Payout')}</th><th>{t('Fee')}</th><th>{t('Terms')}</th><th>{t('Status')}</th></tr></thead>
          <tbody>{(data ?? []).map((a) => (
            <tr key={str(a.eventId)} className="border-t border-border">
              <td className="py-2">{str(a.title)}<div className="text-xs text-text-muted">{str(a.organizer)}</div></td>
              <td>{str(a.legalName) || '—'}</td><td>{str(a.ibanMasked) || '—'}</td>
              <td>{a.feePaidAt ? `${money(num(a.feeKzt))} · ${date(str(a.feePaidAt))}` : '—'}</td>
              <td>{a.termsAcceptedAt ? date(str(a.termsAcceptedAt)) : '—'}</td>
              <td><Badge tone={a.paidSalesStatus === 'active' ? 'success' : a.paidSalesStatus === 'suspended' ? 'error' : 'muted'}>{str(a.paidSalesStatus)}</Badge></td>
            </tr>
          ))}</tbody>
        </table></div>
      )}
    </Panel>
  );
}

function Promo() {
  const { t, money } = useI18n();
  const { data, error, isLoading } = useRows('campaigns', '/api/admin/campaigns');
  return (
    <Panel title="Promo activity">
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (
        <div className="overflow-x-auto"><table className="w-full min-w-[40rem] text-left text-sm">
          <thead className="text-xs uppercase text-text-muted"><tr><th className="py-2">{t('Code')}</th><th>{t('Event')}</th><th>{t('Discount')}</th><th>{t('Redemptions')}</th><th>{t('Visits')}</th><th>{t('Discounts given')}</th></tr></thead>
          <tbody>{(data ?? []).map((c) => (
            <tr key={str(c.id)} className="border-t border-border">
              <td className="py-2 font-display">{str(c.code)} <Badge tone={c.status === 'active' ? 'success' : 'muted'}>{str(c.status)}</Badge></td>
              <td>{str(c.eventTitle)}</td>
              <td>{c.discountType === 'percent' ? `${num(c.discountValue)}%` : money(num(c.discountValue))}</td>
              <td>{num(c.redemptions)}{c.maxRedemptions ? ` / ${num(c.maxRedemptions)}` : ''}</td><td>{num(c.visits)}</td><td>{money(num(c.discountKzt))}</td>
            </tr>
          ))}</tbody>
        </table></div>
      )}
    </Panel>
  );
}

function Reports() {
  const { t, date } = useI18n();
  const { data, error, isLoading, mutate } = useRows('reports', '/api/admin/reports');
  const [actionError, setActionError] = useState('');
  const set = (id: string, status: 'actioned' | 'dismissed') => callEmpty(api.PATCH('/api/admin/reports/{reportId}', { params: { path: { reportId: id } }, body: { status } }))
    .then(() => mutate()).catch((e) => setActionError(getApiErrorMessage(e)));
  return (
    <Panel title="Reports">
      {actionError ? <ErrorMessage message={actionError} className="mb-3" /> : null}
      {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (data ?? []).length === 0 ? <p className="text-sm text-text-muted">{t('No reports.')}</p> : (
        <ul className="grid gap-2">
          {(data ?? []).map((r) => (
            <li key={str(r.id)} className="grid gap-2 rounded-card border border-border p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <Link href={`/events/${str(r.eventId)}`} className="font-semibold text-primary">{str(r.eventTitle)}</Link>
                <Badge tone={r.status === 'open' ? 'error' : 'muted'}>{str(r.status)}</Badge>
              </div>
              <p className="text-sm">{str(r.reason)}</p>
              <p className="text-xs text-text-muted">{str(r.reporter)} · {date(str(r.createdAt))}</p>
              {r.status === 'open' ? <div className="flex gap-2">
                <Button size="sm" variant="secondary" onClick={() => void set(str(r.id), 'actioned')}>{t('Mark as actioned')}</Button>
                <Button size="sm" variant="quiet" onClick={() => void set(str(r.id), 'dismissed')}>{t('Dismiss')}</Button>
              </div> : null}
            </li>
          ))}
        </ul>
      )}
      <p className="mt-3 text-xs text-text-muted">{t('To act on a report, suspend the event on the Events tab.')}</p>
    </Panel>
  );
}

function Settings() {
  const { t } = useI18n();
  const { data, error, isLoading, mutate } = useApi<Record<string, number>>('admin:settings', () => call(api.GET('/api/admin/settings')) as Promise<Record<string, number>>);
  const [values, setValues] = useState<Record<string, string>>({});
  const [message, setMessage] = useState('');
  const [formError, setFormError] = useState('');
  useEffect(() => { if (data) setValues(Object.fromEntries(Object.entries(data).map(([k, v]) => [k, String(v)]))); }, [data]);
  if (isLoading) return <Spinner />;
  if (error) return <ErrorMessage message={getApiErrorMessage(error)} />;
  const fields = [['activation_fee_kzt', 'Activation fee per event (KZT)'], ['processing_fee_percent', 'Processing fee (%)'], ['hold_minutes', 'Checkout hold (minutes)']];
  return (
    <Panel title="Platform settings">
      <Form onSubmit={async () => {
        setMessage(''); setFormError('');
        try {
          await mutate(await call(api.PUT('/api/admin/settings', { body: Object.fromEntries(Object.entries(values).map(([k, v]) => [k, Number(v)])) })) as Record<string, number>, { revalidate: false });
          setMessage('Saved');
        } catch (e) { setFormError(getApiErrorMessage(e)); }
      }} className="grid max-w-xl gap-4">
        {fields.map(([key, label]) => <TextInput key={key} label={label} type="number" step="any" value={values[key] ?? ''} onChange={(e) => setValues({ ...values, [key]: e.currentTarget.value })} />)}
        {formError ? <ErrorMessage message={formError} /> : null}
        <div className="flex items-center gap-3"><Button type="submit">{t('Save settings')}</Button>{message ? <span role="status" className="text-sm font-semibold text-success">{t(message)}</span> : null}</div>
        <p className="text-xs text-text-muted">{t('Changes apply to new activations and orders; every change is recorded in the audit log.')}</p>
      </Form>
    </Panel>
  );
}
