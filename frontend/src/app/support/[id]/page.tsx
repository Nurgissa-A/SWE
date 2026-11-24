'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { SupportCase } from '@/lib/types';
import { CATEGORY_LABEL, type Category } from '@/components/support/NewCaseForm';
import { RequireSignIn } from '@/components/layout/RequireSignIn';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Spinner } from '@/components/ui/Spinner';
import { TextArea } from '@/components/ui/TextArea';

const STATUSES = ['open', 'in_progress', 'waiting_for_customer', 'resolved'] as const;

export default function SupportCasePage() {
  return <RequireSignIn><CaseThread /></RequireSignIn>;
}

function CaseThread() {
  const { id } = useParams<{ id: string }>();
  const { t, dateTime } = useI18n();
  const { user } = useAuth();
  const { data: c, error, isLoading, mutate } = useApi<SupportCase>(`case:${id}`, () => call(api.GET('/api/support/cases/{caseId}', { params: { path: { caseId: id } } })), { refreshInterval: 10_000 });
  const [body, setBody] = useState('');
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState('');

  async function update(change: { status?: (typeof STATUSES)[number]; assignToMe?: boolean; escalate?: boolean }) {
    setActionError('');
    try { await mutate(await call(api.PATCH('/api/support/cases/{caseId}', { params: { path: { caseId: id } }, body: change })), { revalidate: false }); }
    catch (e) { setActionError(getApiErrorMessage(e)); }
  }

  if (isLoading) return <main className="mx-auto max-w-3xl px-4 py-16"><Spinner /></main>;
  if (error || !c) return <main className="mx-auto max-w-3xl px-4 py-16"><ErrorMessage message={getApiErrorMessage(error)} /></main>;
  const isRequester = c.requesterId === user?.id;

  return (
    <main className="mx-auto grid max-w-3xl gap-6 px-4 py-8 sm:px-6 lg:py-12">
      <Link href="/support" className="font-semibold text-primary hover:text-primary-hover">← {t('All cases')}</Link>
      <div className="rounded-card bg-navy p-6 text-surface">
        <p className="text-sm text-mist">#{c.number} · {t(CATEGORY_LABEL[c.category as Category] ?? c.category ?? '')}</p>
        <h1 className="mt-2 text-2xl font-bold text-surface">{c.subject}</h1>
        <div className="mt-3 flex flex-wrap gap-2">
          <Badge onDark tone={c.status === 'resolved' ? 'success' : 'default'}>{c.status ?? 'open'}</Badge>
          {c.escalated ? <Badge onDark tone="error">{t('Escalated')}</Badge> : null}
        </div>
        <dl className="mt-4 grid gap-1 text-sm text-mist">
          {c.eventTitle ? <div>{t('Event')}: <span className="text-white">{c.eventTitle}</span></div> : null}
          {c.orderNumber ? <div>{t('Order')}: <Link className="text-white underline" href={`/orders/${c.orderId}`}>{c.orderNumber}</Link></div> : null}
          {c.ticketCode ? <div>{t('Ticket')}: <span className="text-white">{c.ticketCode}</span></div> : null}
          {!isRequester ? <div>{t('From')}: <span className="text-white">{c.requester}</span></div> : null}
          {c.assignee ? <div>{t('Assigned to')}: <span className="text-white">{c.assignee}</span></div> : null}
        </dl>
      </div>
      {c.canManage || c.canEscalate || (isRequester && c.status !== 'resolved') ? (
        <div className="flex flex-wrap items-center gap-2">
          {c.canManage ? (
            <label className="flex items-center gap-2 text-sm font-semibold">
              {t('Status')}
              <select value={c.status} onChange={(e) => void update({ status: e.currentTarget.value as (typeof STATUSES)[number] })} className="min-h-10 rounded-full border border-border bg-surface px-3">
                {STATUSES.map((s) => <option key={s} value={s}>{t(s.replace(/_/g, ' ').replace(/^./, (x) => x.toUpperCase()))}</option>)}
              </select>
            </label>
          ) : null}
          {c.canManage && c.assignedTo !== user?.id ? <Button size="sm" variant="secondary" onClick={() => void update({ assignToMe: true })}>{t('Assign to me')}</Button> : null}
          {c.canEscalate ? <Button size="sm" variant="secondary" onClick={() => void update({ escalate: true })}>{t('Escalate to BiletFlow')}</Button> : null}
          {isRequester && !c.canManage && c.status !== 'resolved' ? <Button size="sm" variant="secondary" onClick={() => void update({ status: 'resolved' })}>{t('Mark as resolved')}</Button> : null}
        </div>
      ) : null}
      {actionError ? <ErrorMessage message={actionError} /> : null}
      <ol className="grid gap-3" aria-label={t('Messages')}>
        {(c.messages ?? []).map((m) => (
          <li key={m.id} className={`max-w-[85%] rounded-card p-4 ${m.authorId === user?.id ? 'ml-auto bg-secondary' : 'border border-border bg-surface'}`}>
            <p className="text-xs font-semibold text-text-muted">{m.authorId === user?.id ? t('You') : `${m.author} · ${t(m.role === 'biletflow' ? 'BiletFlow' : m.role === 'organizer' ? 'Organizer' : 'Attendee')}`} · {dateTime(m.createdAt, undefined, false)}</p>
            <p className="mt-1 whitespace-pre-line text-sm text-text">{m.body}</p>
          </li>
        ))}
      </ol>
      <Form onSubmit={async () => {
        if (!body.trim()) return;
        setBusy(true); setActionError('');
        try { await mutate(await call(api.POST('/api/support/cases/{caseId}/messages', { params: { path: { caseId: id } }, body: { body } })), { revalidate: false }); setBody(''); }
        catch (e) { setActionError(getApiErrorMessage(e)); } finally { setBusy(false); }
      }} className="grid gap-3">
        <TextArea label="Your reply" value={body} onChange={(e) => setBody(e.currentTarget.value)} maxLength={5000} />
        <div><Button type="submit" loading={busy} disabled={!body.trim()}>{t('Send reply')}</Button></div>
      </Form>
    </main>
  );
}
