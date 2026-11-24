'use client';

import { useEffect, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { DashboardSidebar } from '@/components/layout/DashboardSidebar';
import { LoadingScreen } from '@/components/layout/LoadingScreen';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { useI18n } from '@/lib/i18n';

type DashboardLayoutProps = { children: ReactNode };

export default function DashboardLayout({ children }: DashboardLayoutProps) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, token, isLoading, error, logout, ready } = useAuth();
  const { t } = useI18n();

  useEffect(() => {
    if (ready && !token) router.replace(`/login?next=${encodeURIComponent(pathname)}`);
  }, [ready, token, pathname, router]);

  // Attendees see the organizer onboarding wherever they enter the dashboard.
  const needsOnboarding = Boolean(user && user.role !== 'organizer' && pathname !== '/dashboard/events');
  useEffect(() => {
    if (needsOnboarding) router.replace('/dashboard/events');
  }, [needsOnboarding, router]);

  if (!token || isLoading) return <LoadingScreen label={token ? 'Loading your account' : 'Taking you to sign in'} />;

  if (error || !user) {
    return (
      <main className="mx-auto max-w-2xl px-4 py-16 sm:px-6">
        <ErrorMessage message="We could not load your account. Sign in again to continue." />
        <div className="mt-5 flex flex-wrap gap-3">
          <Button onClick={() => void logout()}>{t('Sign out')}</Button>
          <Link href="/login" className="inline-flex min-h-11 items-center rounded-full border border-border px-5 text-sm font-semibold text-text">{t('Sign in')}</Link>
        </div>
      </main>
    );
  }

  const isOrganizer = user.role === 'organizer';
  if (needsOnboarding) return <LoadingScreen label="Loading your workspace" />;

  return (
    <div className="mx-auto grid max-w-7xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-8 lg:px-8 lg:py-10">
      <DashboardSidebar isOrganizer={isOrganizer} />
      <main className="min-w-0">{children}</main>
    </div>
  );
}
