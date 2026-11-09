import type { ApiEvent } from '@/hooks/useEvents';
import { formatEventDate } from '@/lib/format';
import { Badge } from '@/components/ui/Badge';

type EventPreviewProps = { event: ApiEvent };

export function EventPreview({ event }: EventPreviewProps) {
  const statusTone = event.status === 'published' ? 'success' : event.status === 'cancelled' ? 'error' : 'muted';

  return (
    <article className="overflow-hidden rounded-card border border-border bg-surface shadow-sm">
      <div className="relative flex min-h-60 items-end overflow-hidden bg-secondary p-6 sm:min-h-80 sm:p-10">
        <div className="absolute -right-10 -top-14 size-72 rounded-full bg-primary/15" aria-hidden="true" />
        <div className="absolute right-1/4 top-1/4 size-24 rounded-full border border-primary/20" aria-hidden="true" />
        <div className="relative max-w-3xl">
          {event.category ? <p className="mb-3 text-xs font-bold uppercase tracking-[0.18em] text-primary">{event.category}</p> : null}
          <h1 className="text-3xl font-bold leading-tight tracking-tight text-text sm:text-5xl">{event.title}</h1>
          <div className="mt-5 flex flex-wrap items-center gap-2">
            <Badge tone={statusTone}>{event.status ?? 'draft'}</Badge>
            {event.visibility ? <Badge>{event.visibility}</Badge> : null}
          </div>
        </div>
      </div>
      <div className="grid gap-8 p-6 sm:grid-cols-[minmax(0,1fr)_16rem] sm:p-10">
        <section>
          <h2 className="text-lg font-bold text-text">About this event</h2>
          <p className="mt-3 whitespace-pre-line leading-7 text-text-muted">{event.description?.trim() || 'No description has been added yet.'}</p>
        </section>
        <aside className="h-fit rounded-card border border-border bg-background p-5">
          <p className="text-xs font-bold uppercase tracking-[0.14em] text-text-muted">Date and time</p>
          <p className="mt-2 font-semibold text-text">{formatEventDate(event.startsAt)}</p>
          {event.endsAt ? <p className="mt-1 text-sm text-text-muted">Until {formatEventDate(event.endsAt)}</p> : null}
          {event.capacity != null ? <p className="mt-4 border-t border-border pt-4 text-sm text-text-muted">Capacity: <span className="font-semibold text-text">{event.capacity}</span></p> : null}
        </aside>
      </div>
    </article>
  );
}
