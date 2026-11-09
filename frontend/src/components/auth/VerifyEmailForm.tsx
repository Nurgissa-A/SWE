'use client';

import { useState, type FormEvent } from 'react';
import Link from 'next/link';
import { verifyEmailToken } from '@/lib/auth';
import { getApiErrorMessage } from '@/lib/api-errors';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { TextInput } from '@/components/ui/TextInput';

type VerifyEmailFormProps = { initialToken: string };

export function VerifyEmailForm({ initialToken }: VerifyEmailFormProps) {
  const [token, setToken] = useState(initialToken);
  const [error, setError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [verified, setVerified] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    if (!token.trim()) {
      setError('Paste the verification token from your email.');
      return;
    }

    setError('');
    setIsSubmitting(true);
    try {
      await verifyEmailToken(token.trim());
      setVerified(true);
    } catch (requestError) {
      setError(getApiErrorMessage(requestError));
    } finally {
      setIsSubmitting(false);
    }
  }

  if (verified) {
    return (
      <div className="grid gap-4 text-center" role="status" aria-live="polite">
        <span className="mx-auto grid size-12 place-items-center rounded-full bg-success/10 text-lg font-bold text-success" aria-hidden="true">✓</span>
        <h2 className="text-xl font-bold text-text">Email verified</h2>
        <p className="text-sm leading-6 text-text-muted">Your BiletFlow account is ready. Sign in to continue.</p>
        <Link href="/login" className="font-semibold text-primary hover:text-primary-hover">Continue to sign in</Link>
      </div>
    );
  }

  return (
    <Form onSubmit={handleSubmit} className="grid gap-5">
      {error ? <ErrorMessage message={error} /> : null}
      <TextInput
        label="Verification token"
        type="text"
        autoComplete="one-time-code"
        required
        maxLength={256}
        value={token}
        onChange={(event) => setToken(event.currentTarget.value)}
        helpText="The token is in the verification link sent to your email."
      />
      <Button type="submit" size="lg" loading={isSubmitting} className="w-full">Verify email</Button>
      <p className="text-center text-sm text-text-muted">Already verified? <Link href="/login" className="font-semibold text-primary hover:text-primary-hover">Sign in</Link></p>
    </Form>
  );
}
