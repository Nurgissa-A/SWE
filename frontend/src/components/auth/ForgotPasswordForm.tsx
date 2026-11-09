'use client';

import Link from 'next/link';
import { useState, type FormEvent } from 'react';
import { requestPasswordReset } from '@/lib/auth';
import { getApiErrorMessage } from '@/lib/api-errors';
import { validateEmail } from '@/lib/validation';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { TextInput } from '@/components/ui/TextInput';

export function ForgotPasswordForm() {
  const [email, setEmail] = useState('');
  const [emailError, setEmailError] = useState('');
  const [formError, setFormError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    const validationError = validateEmail(email);
    setEmailError(validationError ?? '');
    setFormError('');
    if (validationError) return;

    setIsSubmitting(true);
    try {
      await requestPasswordReset(email.trim());
      setSubmitted(true);
    } catch (error) {
      setFormError(getApiErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Form onSubmit={handleSubmit} className="grid gap-5">
      {formError ? <ErrorMessage message={formError} /> : null}
      {submitted ? (
        <p role="status" className="rounded-input border border-success/20 bg-success/5 px-4 py-3 text-sm leading-6 text-text">
          If an account matches that email, a password reset link is on its way.
        </p>
      ) : (
        <>
          <TextInput
            label="Email address"
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(event) => setEmail(event.currentTarget.value)}
            error={emailError}
          />
          <Button type="submit" size="lg" loading={isSubmitting} className="w-full">Send reset link</Button>
        </>
      )}
      <p className="text-center text-sm text-text-muted">
        Remembered your password? <Link href="/login" className="font-semibold text-primary hover:text-primary-hover">Sign in</Link>
      </p>
    </Form>
  );
}
