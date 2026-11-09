'use client';

import Link from 'next/link';
import { useParams, useRouter } from 'next/navigation';
import { useState } from 'react';
import { EventForm } from '@/components/events/EventForm';
import { useAuth } from '@/hooks/useAuth';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Spinner } from '@/components/ui/Spinner';
import { Button } from '@/components/ui/Button';
import { useEvent, useRefreshEventLists } from '@/hooks/useEvents';
import { eventsApi } from '@/lib/api-client';
import { toApiRequestError, getApiErrorMessage } from '@/lib/api-errors';
import { toEventUpdatePayload } from '@/lib/event-payload';

export default function EditEventPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { user, isLoading: isUserLoading } = useAuth();
  const { data: event, error, isLoading, mutate } = useEvent(id);
  const refreshEventLists = useRefreshEventLists();
  const [deleteError, setDeleteError] = useState('');
  const [isDeleting, setIsDeleting] = useState(false);

  async function saveEvent(values: Parameters<typeof toEventUpdatePayload>[0]) {
    const { data, error: responseError, response } = await eventsApi.PATCH('/api/events/{eventId}', {
      params: { path: { eventId: id } },
      body: toEventUpdatePayload(values),
    });
    const statusCode = response.status;
    if (responseError) throw toApiRequestError(responseError, statusCode);
    if (data) await mutate(data, { revalidate: false });
    await refreshEventLists();
    router.push(`/dashboard/events/${id}/preview`);
  }

  async function deleteDraft() {
    if (!window.confirm('Delete this event draft? This cannot be undone.')) return;
    setDeleteError('');
    setIsDeleting(true);
    try {
      const { error: responseError, response } = await eventsApi.DELETE('/api/events/{eventId}', {
        params: { path: { eventId: id } },
      });
      const statusCode = response.status;
      if (responseError) throw toApiRequestError(responseError, statusCode);
      await refreshEventLists();
      router.replace('/dashboard/events');
    } catch (requestError) {
      setDeleteError(getApiErrorMessage(requestError));
    } finally {
      setIsDeleting(false);
    }
  }

  if (isLoading || isUserLoading) return <div className="grid min-h-64 place-items-center"><Spinner label="Loading draft" /></div>;
  if (error || !event) return <ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} />;
  if (event.organizerId !== user?.id) return <ErrorMessage message="You can only manage events you created." />;
  if (event.status !== 'draft') {
    return <div className="grid gap-4"><ErrorMessage message="Only draft events can be edited." /><Link href={`/dashboard/events/${id}/preview`} className="font-semibold text-primary hover:text-primary-hover">Return to event preview</Link></div>;
  }

  return (
    <section className="mx-auto max-w-3xl">
      <Link href={`/dashboard/events/${id}/preview`} className="text-sm font-semibold text-primary hover:text-primary-hover">← Event preview</Link>
      <div className="mt-5 rounded-card border border-border bg-surface p-5 sm:p-8">
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
          <div><p className="text-xs font-bold uppercase tracking-[0.16em] text-primary">Draft event</p><h1 className="mt-2 text-3xl font-bold tracking-tight text-text">Edit event</h1></div>
          <Button variant="danger" size="sm" loading={isDeleting} onClick={() => void deleteDraft()}>Delete draft</Button>
        </div>
        {deleteError ? <ErrorMessage className="mt-5" message={deleteError} /> : null}
        <div className="mt-7"><EventForm initialEvent={event} isEdit submitLabel="Save changes" onSubmit={saveEvent} /></div>
      </div>
    </section>
  );
}
