'use client';

import { useI18n } from '@/lib/i18n';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { useEvents } from '@/hooks/useEvents';
import { EventList } from '@/components/events/EventList';
import { OrganizerProfileForm } from '@/components/auth/OrganizerProfileForm';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';

export default function DashboardEventsPage() {
  const { t } = useI18n();
  const { user } = useAuth();
  const { data, error, isLoading, mutate } = useEvents();

  if (user?.role === 'attendee') {
    return (
      <section className="mx-auto max-w-2xl rounded-card border border-border bg-surface p-6 sm:p-9">
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">{t('Organizer setup')}</p>
        <h1 className="mt-3 text-3xl font-bold tracking-tight text-text">{t('Turn your idea into an event.')}</h1>
        <p className="mt-3 leading-7 text-text-muted">{t('Create your organizer profile to manage event drafts and share them with your community. The optional payout field is a simulation label only.')}</p>
        <div className="mt-7"><OrganizerProfileForm /></div>
      </section>
    );
  }

  if (user?.role !== 'organizer') {
    return <ErrorMessage message="Organizer tools are available to organizer accounts." />;
  }

  const events = (data ?? []).filter((event) => event.organizerId === user.id);
  const drafts = events.filter((event) => event.status === 'draft').length;
  const published = events.filter((event) => event.status === 'published').length;

  return (
    <section className="grid gap-7">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">{t('Organizer workspace')}</p>
          <h1 className="mt-2 text-3xl font-bold tracking-tight text-text">{t('My events')}</h1>
          <p className="mt-2 text-text-muted">{t('Create, review, and publish your event details.')}</p>
        </div>
        <Link href="/dashboard/events/new" className="inline-flex min-h-11 items-center justify-center rounded-full bg-primary px-5 font-semibold text-white transition-colors hover:bg-primary-hover">{t('Create event')} <span className="ml-2" aria-hidden="true">+</span></Link>
      </div>

      {error ? <ErrorMessage message="We could not load your events. Try again." onRetry={() => void mutate()} /> : null}
      {!error ? (
        <>
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="rounded-card bg-navy p-5"><p className="text-sm font-medium text-mist">{t('All events')}</p><p className="mt-2 font-display text-3xl font-bold text-surface">{isLoading ? '—' : events.length}</p></div>
            <div className="rounded-card bg-navy p-5"><p className="text-sm font-medium text-mist">{t('Drafts')}</p><p className="mt-2 font-display text-3xl font-bold text-surface">{isLoading ? '—' : drafts}</p></div>
            <div className="rounded-card bg-navy p-5"><p className="text-sm font-medium text-mist">{t('Published')}</p><p className="mt-2 font-display text-3xl font-bold text-mint">{isLoading ? '—' : published}</p></div>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-4"><h2 className="text-xl font-bold text-text">{t('Your event drafts and listings')}</h2><Button variant="secondary" size="sm" onClick={() => void mutate()}>{t('Refresh')}</Button></div>
          <EventList
            events={events}
            isLoading={isLoading}
            error={undefined}
            onRetry={() => void mutate()}
            detailsHref={(event) => `/dashboard/events/${event.id}`}
            detailsLabel="Manage event"
            emptyTitle="Your first event is waiting"
            emptyDescription="Create a draft, add the details, and publish when it is ready."
          />
        </>
      ) : null}
    </section>
  );
}
