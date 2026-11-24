'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { OverviewEvent } from '@/lib/types';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Stat } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';
import { Tabs } from '@/components/ui/Tabs';

const GROUPS = ['upcoming', 'active', 'draft', 'completed', 'cancelled'] as const;
const LABEL: Record<(typeof GROUPS)[number], string> = { upcoming: 'Upcoming', active: 'Happening now', draft: 'Drafts', completed: 'Completed', cancelled: 'Cancelled' };

/** Organizer home: events grouped by lifecycle with headline numbers (SRS §4.16). */
export default function OrganizerOverviewPage() {
  const { t, dateBlock, money, dateTime, number } = useI18n();
  const { data, error, isLoading, mutate } = useApi<OverviewEvent[]>('overview', () => call(api.GET('/api/organizer/overview')) as Promise<OverviewEvent[]>);
  const [group, setGroup] = useState<(typeof GROUPS)[number]>('upcoming');
  if (isLoading) return <Spinner label="Loading your workspace" />;
  if (error) return <ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} />;
  const events = data ?? [];
  const live = events.filter((e) => e.lifecycle === 'upcoming' || e.lifecycle === 'active');
  const shown = events.filter((e) => e.lifecycle === group);
  const openCases = events.reduce((n, e) => n + (e.openCases ?? 0), 0);

  return (
    <section className="grid gap-7">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">{t('Organizer workspace')}</p>
          <h1 className="mt-2 text-3xl font-bold text-text">{t('Overview')}</h1>
        </div>
        <Link href="/dashboard/events/new" className="inline-flex min-h-11 items-center justify-center rounded-full bg-primary px-5 font-semibold text-white hover:bg-primary-hover">{t('Create event')} <span className="ml-2" aria-hidden="true">+</span></Link>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Stat label="Upcoming and live events" value={number(live.length)} />
        <Stat label="Tickets sold" value={number(live.reduce((n, e) => n + (e.sold ?? 0), 0))} hint="for upcoming and live events" />
        <Stat label="Sales" value={money(events.reduce((n, e) => n + (e.revenueKzt ?? 0), 0))} hint="all time, simulated payments" />
        <Stat label="Open attendee questions" value={number(openCases)} accent={openCases > 0} />
      </div>
      <Tabs label="Events by status" active={group} onChange={(g) => setGroup(g as typeof group)}
        tabs={GROUPS.map((g) => ({ id: g, label: LABEL[g], count: events.filter((e) => e.lifecycle === g).length }))} />
      {shown.length === 0 ? <p className="rounded-card border border-dashed border-border p-6 text-sm text-text-muted">{t('No events here.')}</p> : (
        <ul className="grid gap-3">
          {shown.map((e) => {
            const d = dateBlock(e.startsAt, e.timezone);
            return (
              <li key={e.id}>
                <Link href={`/dashboard/events/${e.id}`} className="grid gap-4 rounded-card border border-border bg-surface p-4 hover:border-primary sm:grid-cols-[auto_1fr_auto] sm:items-center">
                  {d ? <div className="grid h-14 w-13 place-content-center rounded-xl bg-navy text-center text-white"><span className="text-[0.65rem] font-semibold">{d.month}</span><span className="font-display text-xl font-bold leading-none">{d.day}</span></div> : null}
                  <div className="min-w-0"><p className="truncate font-semibold">{e.title}</p><p className="text-sm text-text-muted">{dateTime(e.startsAt, e.timezone)}{e.city ? ` · ${e.city}` : ''}</p></div>
                  <dl className="flex gap-5 text-sm">
                    <div><dt className="text-xs text-text-muted">{t('Sold')}</dt><dd className="font-display font-bold">{e.sold ?? 0}</dd></div>
                    <div><dt className="text-xs text-text-muted">{t('Checked in')}</dt><dd className="font-display font-bold">{e.checkedIn ?? 0}</dd></div>
                    <div><dt className="text-xs text-text-muted">{t('Sales')}</dt><dd className="font-display font-bold">{money(e.revenueKzt ?? 0)}</dd></div>
                  </dl>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
