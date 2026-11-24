'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useI18n } from '@/lib/i18n';

type DashboardSidebarProps = { isOrganizer: boolean };

const links = [
  { href: '/dashboard', label: 'Overview' },
  { href: '/dashboard/events', label: 'My events' },
  { href: '/dashboard/events/new', label: 'Create event' },
  { href: '/support', label: 'Support cases' },
];

export function DashboardSidebar({ isOrganizer }: DashboardSidebarProps) {
  const pathname = usePathname();
  const { t } = useI18n();
  return (
    <aside className="w-full rounded-card border border-navy-line bg-navy p-4 lg:sticky lg:top-24 lg:h-fit" aria-label={t('Organizer navigation')}>
      <p className="px-3 pb-3 text-xs font-bold uppercase tracking-[0.14em] text-mist">{t('Workspace')}</p>
      <nav className="flex gap-2 overflow-x-auto lg:flex-col">
        {links.filter((item) => isOrganizer || item.href === '/dashboard/events').map((item) => {
          const active = item.href === pathname || (item.href === '/dashboard/events' && /^\/dashboard\/events\/(?!new)/.test(pathname));
          return (
            <Link key={item.href} href={item.href} aria-current={active ? 'page' : undefined}
              className={`whitespace-nowrap rounded-input px-3 py-2.5 text-sm font-semibold transition-colors ${active ? 'bg-primary text-white' : 'text-mist hover:bg-navy-raised hover:text-white'}`}>
              {t(item.label)}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
