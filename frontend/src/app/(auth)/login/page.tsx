import type { Metadata } from 'next';
import { AuthShell } from '@/components/layout/AuthShell';
import { LoginForm } from '@/components/auth/LoginForm';

export const metadata: Metadata = { title: 'Sign in' };

type LoginPageProps = { searchParams: Promise<{ next?: string }> };

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const params = await searchParams;
  return (
    <AuthShell eyebrow="Welcome back" title="Sign in to BiletFlow" description="Pick up where you left off and keep the good plans coming.">
      <LoginForm returnTo={params.next} />
    </AuthShell>
  );
}
