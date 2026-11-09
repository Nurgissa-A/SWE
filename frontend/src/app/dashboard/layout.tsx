'use client';

import { useEffect, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { DashboardSidebar } from '@/components/layout/DashboardSidebar';
import { LoadingScreen } from '@/components/layout/LoadingScreen';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';

type DashboardLayoutProps = { children: ReactNode };

export default function DashboardLayout({ children }: DashboardLayoutProps) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, token, isLoading, error, logout } = useAuth();

  useEffect(() => {
    if (!token) router.replace(`/login?next=${encodeURIComponent(pathname)}`);
  }, [token, pathname, router]);

  if (!token || isLoading) return <LoadingScreen label={token ? 'Loading your account' : 'Taking you to sign in'} />;

  if (error || !user) {
    return (
      <main className="mx-auto max-w-2xl px-4 py-16 sm:px-6">
        <ErrorMessage message="We could not load your account. Sign in again to continue." />
        <div className="mt-5 flex flex-wrap gap-3">
          <Button onClick={() => void logout()}>Sign out</Button>
          <Link href="/login" className="inline-flex min-h-11 items-center rounded-full border border-border px-5 text-sm font-semibold text-text">Sign in</Link>
        </div>
      </main>
    );
  }

  const isOrganizer = user.role === 'organizer';
  if (!isOrganizer && pathname !== '/dashboard/events') {
    return (
      <main className="mx-auto max-w-2xl px-4 py-16 sm:px-6">
        <ErrorMessage message="Organizer tools are available after you create an organizer profile." />
        <Link href="/dashboard/events" className="mt-5 inline-flex font-semibold text-primary hover:text-primary-hover">Set up an organizer profile</Link>
      </main>
    );
  }

  return (
    <div className="mx-auto grid max-w-7xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-8 lg:px-8 lg:py-10">
      <DashboardSidebar isOrganizer={isOrganizer} />
      <main className="min-w-0">{children}</main>
    </div>
  );
}
