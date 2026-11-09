import type { Metadata } from 'next';
import { ResetPasswordForm } from '@/components/auth/ResetPasswordForm';
import { AuthShell } from '@/components/layout/AuthShell';

export const metadata: Metadata = { title: 'Choose a new password' };

type ResetPasswordPageProps = {
  searchParams: Promise<{ token?: string | string[] }>;
};

export default async function ResetPasswordPage({ searchParams }: ResetPasswordPageProps) {
  const params = await searchParams;
  const token = Array.isArray(params.token) ? params.token[0] : params.token;

  return (
    <AuthShell eyebrow="Account recovery" title="Choose a new password" description="Use a new password to get back to your BiletFlow account.">
      <ResetPasswordForm token={token} />
    </AuthShell>
  );
}
