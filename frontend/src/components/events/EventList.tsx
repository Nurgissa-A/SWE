'use client';

import Link from 'next/link';
import type { ApiEvent } from '@/hooks/useEvents';
import { EventCard } from './EventCard';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Spinner } from '@/components/ui/Spinner';
import { getApiErrorMessage } from '@/lib/api-errors';

type EventListProps = {
  events?: ApiEvent[];
  isLoading: boolean;
  error?: unknown;
  onRetry: () => void;
  detailsHref?: (event: ApiEvent) => string;
  detailsLabel?: string;
  emptyTitle?: string;
  emptyDescription?: string;
};

export function EventList({ events, isLoading, error, onRetry, detailsHref, detailsLabel, emptyTitle = 'No events to show yet', emptyDescription = 'When organizers publish an event, it will appear here.' }: EventListProps) {
  if (isLoading) {
    return (
      <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3" aria-busy="true" aria-label="Loading events">
        <div className="col-span-full py-8 text-center"><Spinner label="Finding events" /></div>
        {[0, 1, 2].map((item) => <div key={item} className="h-72 animate-pulse rounded-card bg-secondary" />)}
      </div>
    );
  }

  if (error) {
    const status = typeof error === 'object' && error !== null && 'status' in error ? error.status : undefined;
    const message = status === 401
      ? 'Sign in to browse events.'
      : getApiErrorMessage(error);

    return (
      <div className="mx-auto max-w-xl">
        <ErrorMessage message={message} onRetry={onRetry} />
        {status === 401 ? <Link href="/login" className="mt-4 inline-flex font-semibold text-primary hover:text-primary-hover">Sign in</Link> : null}
      </div>
    );
  }

  if (!events?.length) {
    return (
      <div className="rounded-card border border-dashed border-border bg-surface px-6 py-14 text-center">
        <span className="mx-auto grid size-12 place-items-center rounded-full bg-secondary text-lg font-bold text-primary" aria-hidden="true">B</span>
        <h2 className="mt-4 text-xl font-bold text-text">{emptyTitle}</h2>
        <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-text-muted">{emptyDescription}</p>
      </div>
    );
  }

  return (
    <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
      {events.map((event) => (
        <EventCard key={event.id} event={event} href={detailsHref?.(event)} detailsLabel={detailsLabel} />
      ))}
    </div>
  );
}
