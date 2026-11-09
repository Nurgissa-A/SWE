'use client';

import Link from 'next/link';
import { useState, type FormEvent } from 'react';
import { resetPassword } from '@/lib/auth';
import { getApiErrorMessage } from '@/lib/api-errors';
import { validatePassword } from '@/lib/validation';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { TextInput } from '@/components/ui/TextInput';

type ResetPasswordFormProps = { token?: string };

export function ResetPasswordForm({ token }: ResetPasswordFormProps) {
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [confirmationError, setConfirmationError] = useState('');
  const [formError, setFormError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    const nextPasswordError = validatePassword(password) ?? '';
    const nextConfirmationError = confirmation !== password ? 'Passwords must match.' : '';
    setPasswordError(nextPasswordError);
    setConfirmationError(nextConfirmationError);
    setFormError('');
    if (!token) {
      setFormError('This reset link is missing its token. Request a new password reset link.');
      return;
    }
    if (nextPasswordError || nextConfirmationError) return;

    setIsSubmitting(true);
    try {
      await resetPassword(token, password);
      setCompleted(true);
    } catch (error) {
      setFormError(getApiErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  if (completed) {
    return (
      <div role="status" className="grid gap-4">
        <p className="rounded-input border border-success/20 bg-success/5 px-4 py-3 text-sm leading-6 text-text">Your password has been updated. You can now sign in with your new password.</p>
        <Link href="/login" className="justify-self-center font-semibold text-primary hover:text-primary-hover">Sign in</Link>
      </div>
    );
  }

  return (
    <Form onSubmit={handleSubmit} className="grid gap-5">
      {formError ? <ErrorMessage message={formError} /> : null}
      {!token ? <ErrorMessage message="This password reset link is missing its token. Request a new one." /> : null}
      <TextInput
        label="New password"
        type="password"
        autoComplete="new-password"
        minLength={8}
        maxLength={1024}
        required
        value={password}
        onChange={(event) => setPassword(event.currentTarget.value)}
        error={passwordError}
        helpText="Use 8 to 1024 characters."
      />
      <TextInput
        label="Confirm new password"
        type="password"
        autoComplete="new-password"
        required
        value={confirmation}
        onChange={(event) => setConfirmation(event.currentTarget.value)}
        error={confirmationError}
      />
      <Button type="submit" size="lg" loading={isSubmitting} disabled={!token} className="w-full">Update password</Button>
      <p className="text-center text-sm text-text-muted">
        Need another link? <Link href="/forgot-password" className="font-semibold text-primary hover:text-primary-hover">Request a reset</Link>
      </p>
    </Form>
  );
}
