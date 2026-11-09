'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

type DashboardSidebarProps = { isOrganizer: boolean };

const links = [
  { href: '/dashboard/events', label: 'My events' },
  { href: '/dashboard/events/new', label: 'Create event' },
];

export function DashboardSidebar({ isOrganizer }: DashboardSidebarProps) {
  const pathname = usePathname();

  return (
    <aside className="w-full rounded-card border border-border bg-surface p-4 lg:sticky lg:top-24 lg:h-fit" aria-label="Organizer navigation">
      <p className="px-3 pb-3 text-xs font-bold uppercase tracking-[0.14em] text-text-muted">Workspace</p>
      <nav className="flex gap-2 overflow-x-auto lg:flex-col">
        {links.filter((item) => isOrganizer || item.href === '/dashboard/events').map((item) => {
          const active = item.href === pathname || (item.href !== '/dashboard/events' && pathname.startsWith(item.href));
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={active ? 'page' : undefined}
              className={`whitespace-nowrap rounded-input px-3 py-2.5 text-sm font-semibold transition-colors ${active ? 'bg-secondary text-primary' : 'text-text-muted hover:bg-background hover:text-text'}`}
            >
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
