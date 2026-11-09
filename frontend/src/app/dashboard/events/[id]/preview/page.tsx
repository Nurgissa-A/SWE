'use client';

import Link from 'next/link';
import { useParams, useRouter } from 'next/navigation';
import { useState } from 'react';
import { eventsApi } from '@/lib/api-client';
import { ApiRequestError, toApiRequestError, getApiErrorMessage } from '@/lib/api-errors';
import { useAuth } from '@/hooks/useAuth';
import { useEvent, useRefreshEventLists } from '@/hooks/useEvents';
import { EventPreview } from '@/components/events/EventPreview';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Spinner } from '@/components/ui/Spinner';

export default function EventPreviewPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { user, isLoading: isUserLoading } = useAuth();
  const { data: event, error, isLoading, mutate } = useEvent(id);
  const refreshEventLists = useRefreshEventLists();
  const [actionError, setActionError] = useState('');
  const [pendingAction, setPendingAction] = useState<'publish' | 'unpublish' | null>(null);

  async function changePublishState(action: 'publish' | 'unpublish') {
    setPendingAction(action);
    setActionError('');
    try {
      const result = action === 'publish'
        ? await eventsApi.POST('/api/events/{eventId}/publish', { params: { path: { eventId: id } } })
        : await eventsApi.POST('/api/events/{eventId}/unpublish', { params: { path: { eventId: id } } });
      const statusCode = result.response.status;
      if (result.error) throw toApiRequestError(result.error, statusCode);
      if (result.data) await mutate(result.data, { revalidate: false });
      await refreshEventLists();
    } catch (requestError) {
      const status = requestError instanceof ApiRequestError ? requestError.status : undefined;
      const unavailableUnpublish = action === 'unpublish' && (status === 404 || status === 405);
      setActionError(unavailableUnpublish
        ? 'Moving a published event back to draft is not supported by the current backend yet.'
        : getApiErrorMessage(requestError));
    } finally {
      setPendingAction(null);
    }
  }

  if (isLoading || isUserLoading) return <div className="grid min-h-64 place-items-center"><Spinner label="Loading event preview" /></div>;
  if (error || !event) return <ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} />;
  if (event.organizerId !== user?.id) return <ErrorMessage message="You can only manage events you created." />;

  const isOwner = event.organizerId === user?.id;
  const isDraft = event.status === 'draft';
  const isPublished = event.status === 'published';

  return (
    <section className="grid gap-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <Link href="/dashboard/events" className="text-sm font-semibold text-primary hover:text-primary-hover">← My events</Link>
          <p className="mt-5 text-xs font-bold uppercase tracking-[0.16em] text-primary">Attendee preview</p>
          <h1 className="mt-2 text-3xl font-bold tracking-tight text-text">Review your event</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          {isOwner && isDraft ? <Link href={`/dashboard/events/${id}/edit`} className="inline-flex min-h-11 items-center rounded-full border border-border bg-surface px-5 text-sm font-semibold text-text hover:bg-secondary">Edit draft</Link> : null}
          {isOwner && isDraft ? <Button loading={pendingAction === 'publish'} disabled={pendingAction !== null} onClick={() => void changePublishState('publish')}>Publish event</Button> : null}
          {isOwner && isPublished ? <Button variant="secondary" loading={pendingAction === 'unpublish'} disabled={pendingAction !== null} onClick={() => void changePublishState('unpublish')}>Move back to draft</Button> : null}
        </div>
      </div>
      {actionError ? <ErrorMessage message={actionError} /> : null}
      {isPublished ? <p className="rounded-input border border-success/20 bg-success/5 px-4 py-3 text-sm text-success">This event is published and visible according to its visibility settings.</p> : null}
      {!isDraft && !isPublished ? <p className="rounded-input border border-border bg-surface px-4 py-3 text-sm text-text-muted">This event is not currently editable.</p> : null}
      <EventPreview event={event} />
    </section>
  );
}
