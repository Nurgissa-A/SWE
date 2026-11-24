import Link from 'next/link';
import type { components } from '@/lib/api-types';
import { eventDateParts, formatEventDate } from '@/lib/format';
import { Badge } from '@/components/ui/Badge';

type EventCardData = Pick<components['schemas']['Event'], 'id' | 'title' | 'startsAt' | 'category' | 'status'>;
type EventCardProps = { event: EventCardData; href?: string; detailsLabel?: string };

function statusTone(status: string | undefined): 'default' | 'success' | 'error' | 'muted' {
  if (status === 'published') return 'success';
  if (status === 'cancelled') return 'error';
  if (status === 'draft') return 'muted';
  return 'default';
}

export function EventCard({ event, href, detailsLabel = 'View details' }: EventCardProps) {
  const destination = href ?? `/events/${event.id}`;
  const date = eventDateParts(event.startsAt);

  return (
    <article className="group flex flex-col overflow-hidden rounded-card border border-border bg-surface shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-lg">
      <div className="flex min-h-44 flex-col justify-between gap-6 bg-navy p-5 sm:min-h-48">
        <div className="flex items-start justify-between gap-3">
          {date ? (
            <div className="grid h-16 w-15 shrink-0 place-content-center rounded-xl bg-primary text-center text-white">
              <span className="text-[0.7rem] font-semibold tracking-[0.1em]">{date.month}</span>
              <span className="font-display text-2xl font-bold leading-none">{date.day}</span>
            </div>
          ) : <span />}
          {event.status ? <Badge tone={statusTone(event.status)} onDark>{event.status}</Badge> : null}
        </div>
        <div>
          {event.category ? <p className="mb-2 text-xs font-bold uppercase tracking-[0.15em] text-mist">{event.category}</p> : null}
          <p className="text-xl font-bold leading-tight tracking-tight text-surface sm:text-2xl">{event.title}</p>
        </div>
      </div>
      <div className="grid flex-1 content-between gap-4 p-5">
        <p className="text-sm font-medium text-text-muted">{formatEventDate(event.startsAt)}</p>
        <Link href={destination} className="inline-flex min-h-11 w-fit items-center rounded-full border border-border px-4 text-sm font-semibold text-text transition-colors hover:border-primary hover:text-primary">
          {detailsLabel}<span className="ml-2" aria-hidden="true">→</span>
        </Link>
      </div>
    </article>
  );
}
