'use client';

import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { Button } from '@/components/ui/Button';

export function SiteHeader() {
  const { user, isLoading, logout } = useAuth();

  return (
    <header className="sticky top-0 z-40 border-b border-navy-line bg-navy text-surface">
      <div className="mx-auto flex min-h-18 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
        <Link href="/" className="inline-flex items-center gap-3 rounded-lg font-display text-base font-bold tracking-tight text-surface" aria-label="BiletFlow home">
          <span className="grid size-9 place-items-center rounded-xl bg-primary text-sm font-bold tracking-normal text-white">B</span>
          <span>BiletFlow</span>
        </Link>
        <nav aria-label="Main navigation" className="hidden items-center gap-7 text-sm font-medium text-mist md:flex">
          <Link href="/events" className="transition-colors hover:text-white">Explore events</Link>
          <Link href="/dashboard/events" className="transition-colors hover:text-white">Host an event</Link>
        </nav>
        <div className="flex items-center gap-2 sm:gap-3">
          {isLoading ? <span className="text-sm text-mist" role="status">Loading account</span> : user ? (
            <>
              <Link href="/dashboard/events" className="hidden max-w-40 truncate text-sm font-medium text-mist hover:text-white sm:block">{user.displayName || user.email}</Link>
              <Button size="sm" variant="onDark" onClick={() => void logout()}>Sign out</Button>
            </>
          ) : (
            <>
              <Link href="/login" className="whitespace-nowrap rounded-full px-3 py-2 text-sm font-semibold text-mist hover:text-white">Sign in</Link>
              <Link href="/register" className="inline-flex min-h-10 items-center whitespace-nowrap rounded-full bg-primary px-4 text-sm font-semibold text-white transition-colors hover:bg-primary-hover">Create account</Link>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
