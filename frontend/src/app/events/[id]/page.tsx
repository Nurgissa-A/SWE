'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useEvent } from '@/hooks/useEvents';
import { getApiErrorMessage } from '@/lib/api-errors';
import { EventPreview } from '@/components/events/EventPreview';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Spinner } from '@/components/ui/Spinner';

export default function EventDetailsPage() {
  const params = useParams<{ id: string }>();
  const { user, isLoading: isUserLoading } = useAuth();
  const { data: event, error, isLoading, mutate } = useEvent(params.id);
  const status = typeof error === 'object' && error !== null && 'status' in error && typeof error.status === 'number' ? error.status : undefined;

  if (isLoading || isUserLoading) return <main className="mx-auto grid min-h-[50vh] max-w-7xl place-items-center px-4"><Spinner label="Loading event details" /></main>;
  if (error || !event) {
    return (
      <main className="mx-auto max-w-3xl px-4 py-16 sm:px-6">
        {status === 404 ? (
          <div className="rounded-card border border-border bg-surface p-8 text-center">
            <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">Event not found</p>
            <h1 className="mt-3 text-3xl font-bold text-text">This event may have moved.</h1>
            <p className="mt-3 text-text-muted">Try browsing the latest events instead.</p>
            <Link href="/events" className="mt-6 inline-flex min-h-11 items-center rounded-full bg-primary px-5 font-semibold text-white hover:bg-primary-hover">Explore events</Link>
          </div>
        ) : <ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} />}
      </main>
    );
  }

  const isOwner = event.organizerId === user?.id;
  if (!isOwner && (event.status !== 'published' || event.visibility === 'private')) {
    return (
      <main className="mx-auto max-w-3xl px-4 py-16 sm:px-6">
        <div className="rounded-card border border-border bg-surface p-8 text-center">
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">Event not found</p>
          <h1 className="mt-3 text-3xl font-bold text-text">This event may have moved.</h1>
          <p className="mt-3 text-text-muted">Try browsing the latest events instead.</p>
          <Link href="/events" className="mt-6 inline-flex min-h-11 items-center rounded-full bg-primary px-5 font-semibold text-white hover:bg-primary-hover">Explore events</Link>
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8 lg:py-12">
      <Link href="/events" className="mb-6 inline-flex font-semibold text-primary hover:text-primary-hover">← Back to events</Link>
      <EventPreview event={event} />
      <div className="mt-8 flex flex-wrap items-center justify-between gap-4 rounded-card border border-border bg-surface p-5 sm:p-6">
        <div><h2 className="font-bold text-text">Want to bring people together?</h2><p className="mt-1 text-sm text-text-muted">Create an account to start organizing your own events.</p></div>
        <Link href="/register" className="inline-flex min-h-11 items-center rounded-full bg-primary px-5 font-semibold text-white hover:bg-primary-hover">Create account</Link>
      </div>
    </main>
  );
}
