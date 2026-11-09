'use client';

import { useState, type FormEvent } from 'react';
import Link from 'next/link';
import { registerAccount } from '@/lib/auth';
import { ApiRequestError, getApiErrorMessage } from '@/lib/api-errors';
import { validateEmail, validatePassword } from '@/lib/validation';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { TextInput } from '@/components/ui/TextInput';

export function RegisterForm() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [registered, setRegistered] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    const nextErrors: Record<string, string> = {};
    const emailError = validateEmail(email);
    const passwordError = validatePassword(password);
    if (emailError) nextErrors.email = emailError;
    if (passwordError) nextErrors.password = passwordError;
    setFieldErrors(nextErrors);
    setFormError('');
    if (Object.keys(nextErrors).length > 0) return;

    setIsSubmitting(true);
    try {
      await registerAccount({ email: email.trim(), password });
      setRegistered(true);
    } catch (error) {
      setFormError(error instanceof ApiRequestError && error.status === 409
        ? 'An account with this email is already registered. Sign in or check the address you used.'
        : getApiErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  if (registered) {
    return (
      <div className="grid gap-4 text-center" role="status" aria-live="polite">
        <span className="mx-auto grid size-12 place-items-center rounded-full bg-success/10 text-lg font-bold text-success" aria-hidden="true">✓</span>
        <h2 className="text-xl font-bold text-text">Check your inbox</h2>
        <p className="text-sm leading-6 text-text-muted">We sent a verification link to <strong className="break-all text-text">{email}</strong>. Open it to verify your account, then come back to sign in.</p>
        <Link href="/verify-email" className="font-semibold text-primary hover:text-primary-hover">Enter a verification token</Link>
        <Link href="/login" className="text-sm font-semibold text-text-muted hover:text-text">Already verified? Sign in</Link>
      </div>
    );
  }

  return (
    <Form onSubmit={handleSubmit} className="grid gap-5">
      {formError ? <ErrorMessage message={formError} /> : null}
      <TextInput
        label="Email address"
        type="email"
        autoComplete="email"
        required
        value={email}
        onChange={(event) => setEmail(event.currentTarget.value)}
        error={fieldErrors.email}
      />
      <TextInput
        label="Create a password"
        type="password"
        autoComplete="new-password"
        required
        minLength={8}
        helpText="Use at least 8 characters."
        value={password}
        onChange={(event) => setPassword(event.currentTarget.value)}
        error={fieldErrors.password}
      />
      <Button type="submit" size="lg" loading={isSubmitting} className="w-full">Create account</Button>
      <p className="text-center text-sm text-text-muted">Already have an account? <Link href="/login" className="font-semibold text-primary hover:text-primary-hover">Sign in</Link></p>
    </Form>
  );
}
