'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { EventForm } from '@/components/events/EventForm';
import { eventsApi } from '@/lib/api-client';
import { toApiRequestError } from '@/lib/api-errors';
import { toEventPayload } from '@/lib/event-payload';
import { useRefreshEventLists } from '@/hooks/useEvents';

export default function NewEventPage() {
  const router = useRouter();
  const refreshEventLists = useRefreshEventLists();

  async function createEvent(values: Parameters<typeof toEventPayload>[0]) {
    const { data, error, response } = await eventsApi.POST('/api/events', { body: toEventPayload(values) });
    const statusCode = response.status;
    if (error) throw toApiRequestError(error, statusCode);
    if (!data) throw toApiRequestError(undefined, statusCode);
    await refreshEventLists();
    router.push(`/dashboard/events/${data.id}/preview`);
  }

  return (
    <section className="mx-auto max-w-3xl">
      <Link href="/dashboard/events" className="text-sm font-semibold text-primary hover:text-primary-hover">← My events</Link>
      <div className="mt-5 rounded-card border border-border bg-surface p-5 sm:p-8">
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">New draft</p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight text-text">Create an event</h1>
        <p className="mt-2 mb-7 text-sm leading-6 text-text-muted">Add the essential details first. You can review the attendee preview before publishing.</p>
        <EventForm submitLabel="Create draft" onSubmit={createEvent} />
      </div>
    </section>
  );
}
