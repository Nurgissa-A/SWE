'use client';

import { useState, type ReactNode } from 'react';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent, Checklist, PaymentSetup } from '@/lib/types';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Panel, SimulationNotice } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';
import { TextInput } from '@/components/ui/TextInput';

/** Per-event paid-sales activation checklist (SRS §4.5). Free events never need this. */
export function ActivationPanel({ event, onChange }: { event: ApiEvent; onChange: () => void }) {
  const { t, money } = useI18n();
  const { data: c, error, isLoading, mutate } = useApi<Checklist>(`activation:${event.id}`,
    () => call(api.GET('/api/events/{eventId}/activation', { params: { path: { eventId: event.id } } })));
  const [card, setCard] = useState('4242 4242 4242 4242');
  const [busy, setBusy] = useState('');
  const [actionError, setActionError] = useState('');

  async function run(name: string, fn: () => Promise<Checklist>) {
    setBusy(name); setActionError('');
    try { await mutate(await fn(), { revalidate: false }); onChange(); }
    catch (e) { setActionError(getApiErrorMessage(e)); } finally { setBusy(''); }
  }

  if (isLoading) return <Spinner />;
  if (error || !c) return <ErrorMessage message={getApiErrorMessage(error)} />;
  if (c.paidSalesStatus === 'active') {
    return <Panel title="Paid sales"><p className="rounded-input bg-success/10 p-4 font-semibold text-success">{t('Paid sales are active for this event.')}</p><div className="mt-4"><SimulationNotice /></div></Panel>;
  }
  if (c.paidSalesStatus === 'suspended') {
    return <Panel title="Paid sales"><p className="rounded-input bg-error/10 p-4 font-semibold text-error">{t('BiletFlow paused paid sales for this event. Free registration stays open. Contact support for details.')}</p></Panel>;
  }

  return (
    <Panel title="Paid sales">
      <div className="grid gap-5">
        <p className="text-sm text-text-muted">{t('Free tickets work without this. To sell paid tickets for this event, complete each step. Activation applies to this event only.')}</p>
        <SimulationNotice />
        {actionError ? <ErrorMessage message={actionError} /> : null}
        <ol className="grid gap-3">
          <Step done={c.hasPaidTicketType} title="Add a paid ticket type">{t('Create a ticket type with a price on the Tickets tab.')}</Step>
          <Step done={c.identityVerified} title="Verify your identity (simulated)"><IdentityForm onDone={() => void mutate()} /></Step>
          <Step done={c.payoutAccount} title="Add a payout account (simulated)"><PayoutForm onDone={() => void mutate()} /></Step>
          <Step done={c.termsAccepted} title="Accept the paid-sales terms">
            <p className="text-sm text-text-muted">{t('You are responsible for running the event and for refunds under your refund policy. BiletFlow keeps a 2.5% processing fee from each paid order.')}</p>
            <Button size="sm" className="mt-2" loading={busy === 'terms'} onClick={() => void run('terms', () => call(api.POST('/api/events/{eventId}/activation/terms', { params: { path: { eventId: event.id } } })))}>{t('I accept')}</Button>
          </Step>
          <Step done={c.feePaid} title="Pay the activation fee">
            <p className="text-sm text-text-muted">{t('One-time, non-refundable fee for this event: {fee}.', { fee: money(c.feeKzt) })}</p>
            <Form onSubmit={() => run('fee', () => call(api.POST('/api/events/{eventId}/activation/fee', { params: { path: { eventId: event.id } }, body: { cardNumber: card } })))} className="mt-2 flex flex-wrap items-end gap-2">
              <div className="min-w-60 flex-1"><TextInput label="Card number" value={card} onChange={(e) => setCard(e.currentTarget.value)} /></div>
              <Button type="submit" loading={busy === 'fee'}>{t('Pay {amount}', { amount: money(c.feeKzt) })}</Button>
            </Form>
          </Step>
        </ol>
        <div className="flex flex-wrap items-center gap-3 border-t border-border pt-4">
          <Button size="lg" disabled={!c.ready} loading={busy === 'activate'} onClick={() => void run('activate', () => call(api.POST('/api/events/{eventId}/activation/activate', { params: { path: { eventId: event.id } } })))}>{t('Activate paid sales')}</Button>
          {!c.ready ? <p className="text-sm text-text-muted">{t('Complete every step first.')}</p> : null}
        </div>
      </div>
    </Panel>
  );
}

function Step({ done, title, children }: { done: boolean; title: string; children: ReactNode }) {
  const { t } = useI18n();
  return (
    <li className={`grid gap-2 rounded-card border p-4 ${done ? 'border-success/30 bg-success/5' : 'border-border'}`}>
      <p className="flex items-center gap-2 font-semibold">
        <span aria-hidden="true" className={`grid size-6 place-items-center rounded-full text-xs ${done ? 'bg-success text-white' : 'border border-border'}`}>{done ? '✓' : ''}</span>
        {t(title)} <span className="sr-only">{t(done ? 'Done' : 'Not done')}</span>
      </p>
      {!done ? <div className="pl-8">{children}</div> : null}
    </li>
  );
}

function IdentityForm({ onDone }: { onDone: () => void }) {
  const { t } = useI18n();
  const [legalName, setLegalName] = useState('');
  const [iin, setIin] = useState('');
  const [consent, setConsent] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  return (
    <Form onSubmit={async () => {
      setBusy(true); setError('');
      try { await call<PaymentSetup>(api.PUT('/api/organizer/payment-setup/identity', { body: { legalName, iin, consent } })); onDone(); }
      catch (e) { setError(getApiErrorMessage(e)); } finally { setBusy(false); }
    }} className="grid gap-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <TextInput label="Full legal name" required value={legalName} onChange={(e) => setLegalName(e.currentTarget.value)} />
        <TextInput label="IIN (12 digits)" required inputMode="numeric" maxLength={12} value={iin} onChange={(e) => setIin(e.currentTarget.value.replace(/\D/g, ''))} helpText="Checked for format only and never stored." />
      </div>
      <label className="flex items-start gap-2 text-sm"><input type="checkbox" checked={consent} onChange={(e) => setConsent(e.currentTarget.checked)} className="mt-1" />{t('I agree to a simulated identity check for this demo.')}</label>
      {error ? <ErrorMessage message={error} /> : null}
      <div><Button type="submit" size="sm" loading={busy}>{t('Verify')}</Button></div>
    </Form>
  );
}

function PayoutForm({ onDone }: { onDone: () => void }) {
  const { t } = useI18n();
  const [bankName, setBank] = useState('');
  const [accountHolder, setHolder] = useState('');
  const [iban, setIban] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  return (
    <Form onSubmit={async () => {
      setBusy(true); setError('');
      try { await call<PaymentSetup>(api.PUT('/api/organizer/payment-setup/payout', { body: { bankName, accountHolder, iban } })); onDone(); }
      catch (e) { setError(getApiErrorMessage(e)); } finally { setBusy(false); }
    }} className="grid gap-3">
      <div className="grid gap-3 sm:grid-cols-3">
        <TextInput label="Bank" required value={bankName} onChange={(e) => setBank(e.currentTarget.value)} />
        <TextInput label="Account holder" required value={accountHolder} onChange={(e) => setHolder(e.currentTarget.value)} />
        <TextInput label="IBAN" required value={iban} onChange={(e) => setIban(e.currentTarget.value.toUpperCase())} placeholder="KZ…" helpText="Stored masked." />
      </div>
      {error ? <ErrorMessage message={error} /> : null}
      <div><Button type="submit" size="sm" loading={busy}>{t('Save payout account')}</Button></div>
    </Form>
  );
}
