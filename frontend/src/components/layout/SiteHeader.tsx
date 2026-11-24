'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useI18n } from '@/lib/i18n';
import { Button } from '@/components/ui/Button';
import { LanguageSwitcher } from './LanguageSwitcher';
import { NotificationBell } from './NotificationBell';

export function SiteHeader() {
  const { user, isLoading, logout } = useAuth();
  const { t } = useI18n();
  const pathname = usePathname();

  const links = [
    { href: '/events', label: 'Explore events', show: true },
    { href: '/orders', label: 'My tickets', show: Boolean(user) },
    { href: '/dashboard', label: user?.role === 'organizer' ? 'Dashboard' : 'Host an event', show: user?.role !== 'platform_admin' },
    { href: '/support', label: 'Support', show: Boolean(user) },
    { href: '/admin', label: 'Admin', show: user?.role === 'platform_admin' },
  ].filter((l) => l.show);

  const nav = (className: string) => (
    <nav aria-label={t('Main navigation')} className={className}>
      {links.map((l) => {
        const active = pathname === l.href || pathname.startsWith(l.href + '/');
        return (
          <Link key={l.href} href={l.href} aria-current={active ? 'page' : undefined}
            className={`whitespace-nowrap transition-colors hover:text-white ${active ? 'text-white' : ''}`}>
            {t(l.label)}
          </Link>
        );
      })}
    </nav>
  );

  return (
    <header className="sticky top-0 z-40 border-b border-navy-line bg-navy text-surface">
      <div className="mx-auto flex min-h-18 max-w-7xl items-center justify-between gap-3 px-4 sm:px-6 lg:px-8">
        <Link href="/" className="inline-flex items-center gap-3 rounded-lg font-display text-base font-bold tracking-tight text-surface" aria-label={t('BiletFlow home')}>
          <span className="grid size-9 place-items-center rounded-xl bg-primary text-sm font-bold tracking-normal text-white">B</span>
          <span className="hidden sm:inline">BiletFlow</span>
        </Link>
        {nav('hidden items-center gap-6 text-sm font-medium text-mist lg:flex')}
        <div className="flex items-center gap-2">
          <LanguageSwitcher />
          {isLoading ? <span className="text-sm text-mist" role="status">{t('Loading account')}</span> : user ? (
            <>
              <NotificationBell />
              <Link href="/account" className="hidden max-w-40 truncate text-sm font-medium text-mist hover:text-white md:block">{user.displayName || user.email}</Link>
              <Button size="sm" variant="onDark" onClick={() => void logout()}>{t('Sign out')}</Button>
            </>
          ) : (
            <>
              <Link href="/login" className="whitespace-nowrap rounded-full px-3 py-2 text-sm font-semibold text-mist hover:text-white">{t('Sign in')}</Link>
              <Link href="/register" className="hidden min-h-10 items-center whitespace-nowrap rounded-full bg-primary px-4 text-sm font-semibold text-white transition-colors hover:bg-primary-hover sm:inline-flex">{t('Create account')}</Link>
            </>
          )}
        </div>
      </div>
      <div className="border-t border-navy-line lg:hidden">
        {nav('mx-auto flex max-w-7xl gap-5 overflow-x-auto px-4 py-2.5 text-sm font-medium text-mist sm:px-6')}
      </div>
    </header>
  );
}
