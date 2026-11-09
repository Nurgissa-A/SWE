import type { Metadata } from 'next';
import { ForgotPasswordForm } from '@/components/auth/ForgotPasswordForm';
import { AuthShell } from '@/components/layout/AuthShell';

export const metadata: Metadata = { title: 'Forgot password' };

export default function ForgotPasswordPage() {
  return (
    <AuthShell eyebrow="Account recovery" title="Reset your password" description="Enter the email address on your account and we&apos;ll send a secure reset link if it exists.">
      <ForgotPasswordForm />
    </AuthShell>
  );
}
