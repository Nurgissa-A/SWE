'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Select } from '@/components/ui/Select';
import { TextArea } from '@/components/ui/TextArea';
import { TextInput } from '@/components/ui/TextInput';

export const CATEGORIES = ['ticket_delivery', 'payment', 'refund', 'event_information', 'check_in', 'account', 'activation', 'technical'] as const;
export type Category = (typeof CATEGORIES)[number];
export const CATEGORY_LABEL: Record<Category, string> = {
  ticket_delivery: 'Ticket delivery', payment: 'Payment', refund: 'Refund', event_information: 'Event information',
  check_in: 'Check-in', account: 'Account', activation: 'Paid-sales activation', technical: 'Technical problem',
};

type Props = { eventId?: string; orderId?: string; ticketId?: string; kind?: 'attendee' | 'organizer'; defaultCategory?: Category; onDone?: () => void };

/** Opens a support case; the event, order or ticket it is about is attached automatically (SRS §4.13). */
export function NewCaseForm({ eventId, orderId, ticketId, kind = 'attendee', defaultCategory = 'ticket_delivery', onDone }: Props) {
  const { t } = useI18n();
  const router = useRouter();
  const [category, setCategory] = useState<Category>(defaultCategory);
  const [subject, setSubject] = useState('');
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  return (
    <Form onSubmit={async () => {
      setBusy(true); setError('');
      try {
        const created = await call(api.POST('/api/support/cases', { body: { kind, category, subject, message, eventId: eventId ?? null, orderId: orderId ?? null, ticketId: ticketId ?? null } }));
        onDone?.();
        router.push(`/support/${created.id}`);
      } catch (e) { setError(getApiErrorMessage(e)); } finally { setBusy(false); }
    }} className="grid gap-4 rounded-card border border-border bg-background p-4 sm:p-5">
      <Select label="Topic" value={category} onChange={(e) => setCategory(e.currentTarget.value as Category)}>
        {CATEGORIES.filter((c) => kind === 'organizer' || c !== 'activation').map((c) => <option key={c} value={c}>{t(CATEGORY_LABEL[c])}</option>)}
      </Select>
      <TextInput label="Subject" required maxLength={150} value={subject} onChange={(e) => setSubject(e.currentTarget.value)} />
      <TextArea label="Message" required maxLength={5000} value={message} onChange={(e) => setMessage(e.currentTarget.value)} />
      {error ? <ErrorMessage message={error} /> : null}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" loading={busy}>{t('Send to support')}</Button>
        {onDone ? <Button variant="quiet" onClick={onDone}>{t('Cancel')}</Button> : null}
      </div>
    </Form>
  );
}
