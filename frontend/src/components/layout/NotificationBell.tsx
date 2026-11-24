'use client';

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import useSWR from 'swr';
import { api } from '@/lib/api-client';
import { call } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { components } from '@/lib/api-types';

type Notification = components['schemas']['Notification'];
type Feed = { items?: Notification[]; unread?: number };

/** Bell with unread count; polls every 30 s (SRS §9 allows polling instead of sockets). */
export function NotificationBell() {
  const { t, locale, dateTime } = useI18n();
  const [open, setOpen] = useState(false);
  const panel = useRef<HTMLDivElement>(null);
  const { data, mutate } = useSWR<Feed>(['notifications', locale], () => call(api.GET('/api/notifications')), { refreshInterval: 30_000 });
  const unread = data?.unread ?? 0;

  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => { if (!panel.current?.contains(event.target as Node)) setOpen(false); };
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape') setOpen(false); };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', escape);
    return () => { document.removeEventListener('mousedown', close); document.removeEventListener('keydown', escape); };
  }, [open]);

  async function markAll() {
    await api.POST('/api/notifications/read-all');
    void mutate();
  }

  return (
    <div className="relative" ref={panel}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-label={unread ? t('Notifications, {n} unread', { n: unread }) : t('Notifications')}
        className="relative grid size-10 place-items-center rounded-full border border-navy-line text-mist hover:bg-navy-raised hover:text-white"
      >
        <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" /><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" /></svg>
        {unread ? <span className="absolute -right-0.5 -top-0.5 grid min-w-5 place-items-center rounded-full bg-mint px-1 text-[0.7rem] font-bold text-navy">{unread}</span> : null}
      </button>
      {open ? (
        <div className="absolute right-0 top-12 z-50 w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-card border border-border bg-surface text-text shadow-xl">
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <p className="font-semibold">{t('Notifications')}</p>
            {unread ? <button type="button" onClick={() => void markAll()} className="text-sm font-semibold text-primary hover:text-primary-hover">{t('Mark all as read')}</button> : null}
          </div>
          <ul className="max-h-96 overflow-y-auto">
            {(data?.items ?? []).length === 0 ? <li className="px-4 py-6 text-sm text-text-muted">{t('No notifications yet.')}</li> : null}
            {(data?.items ?? []).map((n) => (
              <li key={n.id} className={`border-b border-border last:border-0 ${n.read ? '' : 'bg-secondary/60'}`}>
                <Link href={n.link ?? '#'} onClick={() => { setOpen(false); if (!n.read && n.id) void api.POST('/api/notifications/{id}/read', { params: { path: { id: n.id } } }).then(() => mutate()); }} className="block px-4 py-3 hover:bg-background">
                  <p className="text-sm font-semibold">{n.title}</p>
                  {n.body ? <p className="mt-1 line-clamp-2 text-sm text-text-muted">{n.body}</p> : null}
                  <p className="mt-1 text-xs text-text-muted">{dateTime(n.createdAt, undefined, false)}</p>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
