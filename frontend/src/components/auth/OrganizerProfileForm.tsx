'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { authApi } from '@/lib/api-client';
import { toApiRequestError, getApiErrorMessage } from '@/lib/api-errors';
import { validateEmail } from '@/lib/validation';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { TextInput } from '@/components/ui/TextInput';

export function OrganizerProfileForm() {
  const router = useRouter();
  const { refreshUser } = useAuth();
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [simulationLabel, setSimulationLabel] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    const errors: Record<string, string> = {};
    const emailError = validateEmail(email);
    if (emailError) errors.contactEmail = emailError;
    if (phone && !/^[+0-9 ()-]{1,32}$/.test(phone)) errors.contactPhone = 'Use a phone number with digits, spaces, parentheses, +, or hyphens.';
    if (simulationLabel && (!simulationLabel.startsWith('demo:') || simulationLabel.length > 100)) {
      errors.payoutAccountPlaceholder = 'Use a simulation label starting with demo: (100 characters maximum).';
    }
    setFieldErrors(errors);
    setFormError('');
    if (Object.keys(errors).length > 0) return;

    setIsSubmitting(true);
    try {
      const { error, response } = await authApi.POST('/api/organizer-profile', {
        body: {
          contactEmail: email.trim(),
          contactPhone: phone.trim() || null,
          payoutAccountPlaceholder: simulationLabel.trim() || null,
        },
      });
      const statusCode = response.status;
      if (error) throw toApiRequestError(error, statusCode);
      await refreshUser();
      router.replace('/dashboard/events');
    } catch (error) {
      setFormError(getApiErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Form onSubmit={handleSubmit} className="grid gap-5">
      {formError ? <ErrorMessage message={formError} /> : null}
      <TextInput label="Organizer contact email" type="email" autoComplete="email" required value={email} onChange={(event) => setEmail(event.currentTarget.value)} error={fieldErrors.contactEmail} />
      <TextInput label="Contact phone" type="tel" autoComplete="tel" maxLength={32} value={phone} onChange={(event) => setPhone(event.currentTarget.value)} error={fieldErrors.contactPhone} />
      <TextInput label="Payout simulation label" type="text" maxLength={100} value={simulationLabel} onChange={(event) => setSimulationLabel(event.currentTarget.value)} error={fieldErrors.payoutAccountPlaceholder} helpText="Optional demo-only label. Never enter real payout or banking information; the API accepts simulation labels prefixed with demo:." />
      <Button type="submit" size="lg" loading={isSubmitting}>Create organizer profile</Button>
    </Form>
  );
}
