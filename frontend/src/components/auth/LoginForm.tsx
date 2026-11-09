'use client';

import { useState, type FormEvent } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { validateEmail } from '@/lib/validation';
import { getApiErrorMessage } from '@/lib/api-errors';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { TextInput } from '@/components/ui/TextInput';

type LoginFormProps = { returnTo?: string };

export function LoginForm({ returnTo }: LoginFormProps) {
  const router = useRouter();
  const { login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    const nextErrors: Record<string, string> = {};
    const emailError = validateEmail(email);
    if (emailError) nextErrors.email = emailError;
    if (!password) nextErrors.password = 'Enter your password.';
    setFieldErrors(nextErrors);
    setFormError('');
    if (Object.keys(nextErrors).length > 0) return;

    setIsSubmitting(true);
    try {
      const user = await login({ email: email.trim(), password });
      const safeReturnTo = returnTo?.startsWith('/') && !returnTo.startsWith('//') ? returnTo : undefined;
      const defaultPath = user.role === 'organizer' ? '/dashboard/events' : '/events';
      router.replace(safeReturnTo ?? defaultPath);
    } catch (error) {
      setFormError(getApiErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
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
      <div className="grid gap-2">
        <TextInput
          label="Password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(event) => setPassword(event.currentTarget.value)}
          error={fieldErrors.password}
        />
        <Link href="/forgot-password" className="justify-self-end text-sm font-semibold text-primary hover:text-primary-hover">Forgot password?</Link>
      </div>
      <Button type="submit" size="lg" loading={isSubmitting} className="w-full">Sign in</Button>
      <p className="text-center text-sm text-text-muted">
        New to BiletFlow? <Link href="/register" className="font-semibold text-primary hover:text-primary-hover">Create an account</Link>
      </p>
    </Form>
  );
}
