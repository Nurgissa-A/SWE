'use client';

import { useState } from 'react';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, callEmpty, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent, StaffMember } from '@/lib/types';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Panel } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';
import { TextInput } from '@/components/ui/TextInput';

/** Event Admins can check guests in for this event only, with the BiletFlow mobile app. */
export function StaffPanel({ event }: { event: ApiEvent }) {
  const { t } = useI18n();
  const { data, error, isLoading, mutate } = useApi<StaffMember[]>(`staff:${event.id}`, () => call(api.GET('/api/events/{eventId}/staff', { params: { path: { eventId: event.id } } })));
  const [email, setEmail] = useState('');
  const [canUndo, setCanUndo] = useState(true);
  const [formError, setFormError] = useState('');
  const [busy, setBusy] = useState(false);

  return (
    <Panel title="Door staff">
      <div className="grid gap-4">
        <p className="text-sm text-text-muted">{t('Door staff sign in to the BiletFlow check-in app with their own account and can only scan tickets for this event. You can always check guests in yourself.')}</p>
        <Form onSubmit={async () => {
          setBusy(true); setFormError('');
          try { await call(api.POST('/api/events/{eventId}/staff', { params: { path: { eventId: event.id } }, body: { email, canUndo } })); setEmail(''); await mutate(); }
          catch (e) { setFormError(getApiErrorMessage(e)); } finally { setBusy(false); }
        }} className="grid gap-3 rounded-card border border-border bg-background p-4">
          <div className="flex flex-wrap items-end gap-3">
            <div className="min-w-64 flex-1"><TextInput label="Their BiletFlow account e-mail" type="email" required value={email} onChange={(e) => setEmail(e.currentTarget.value)} /></div>
            <Button type="submit" loading={busy}>{t('Add Event Admin')}</Button>
          </div>
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={canUndo} onChange={(e) => setCanUndo(e.currentTarget.checked)} />{t('Allowed to undo a check-in')}</label>
          {formError ? <ErrorMessage message={formError} /> : null}
        </Form>
        {isLoading ? <Spinner /> : error ? <ErrorMessage message={getApiErrorMessage(error)} /> : (data ?? []).length === 0 ? <p className="text-sm text-text-muted">{t('No door staff yet.')}</p> : (
          <ul className="grid gap-2">
            {(data ?? []).map((m) => (
              <li key={m.id} className="flex flex-wrap items-center justify-between gap-3 rounded-card border border-border p-3">
                <div><p className="font-semibold">{m.displayName || m.email}</p><p className="text-sm text-text-muted">{m.email} · {t(m.canUndo ? 'Can undo check-ins' : 'Cannot undo check-ins')}</p></div>
                <Button size="sm" variant="quiet" onClick={() => void callEmpty(api.DELETE('/api/events/{eventId}/staff/{staffId}', { params: { path: { eventId: event.id, staffId: m.id } } })).then(() => mutate()).catch((e) => setFormError(getApiErrorMessage(e)))}>{t('Remove')}</Button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Panel>
  );
}
