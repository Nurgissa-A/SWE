import type { Metadata } from 'next';
import { AuthShell } from '@/components/layout/AuthShell';
import { VerifyEmailForm } from '@/components/auth/VerifyEmailForm';

export const metadata: Metadata = { title: 'Verify your email' };

type VerifyEmailPageProps = { searchParams: Promise<{ token?: string | string[] }> };

export default async function VerifyEmailPage({ searchParams }: VerifyEmailPageProps) {
  const params = await searchParams;
  const token = Array.isArray(params.token) ? params.token[0] ?? '' : params.token ?? '';
  return (
    <AuthShell eyebrow="One quick step" title="Verify your email" description="Paste the token from your verification email to finish setting up your account.">
      <VerifyEmailForm initialToken={token} />
    </AuthShell>
  );
}
