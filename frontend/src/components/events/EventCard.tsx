import Link from 'next/link';
import type { components } from '@/lib/api-types';
import { formatEventDate } from '@/lib/format';
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

  return (
    <article className="group overflow-hidden rounded-card border border-border bg-surface shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-lg">
      <div className="relative flex min-h-44 items-end overflow-hidden bg-secondary p-5 sm:min-h-52">
        <div className="absolute -right-8 -top-10 size-48 rounded-full bg-primary/15" aria-hidden="true" />
        <div className="absolute right-14 top-12 size-20 rounded-full border border-primary/20" aria-hidden="true" />
        <div className="relative flex w-full items-end justify-between gap-3">
          <div>
            {event.category ? <p className="mb-2 text-xs font-bold uppercase tracking-[0.15em] text-primary">{event.category}</p> : null}
            <p className="text-2xl font-bold leading-tight tracking-tight text-text sm:text-3xl">{event.title}</p>
          </div>
          {event.status ? <Badge tone={statusTone(event.status)}>{event.status}</Badge> : null}
        </div>
      </div>
      <div className="grid gap-4 p-5">
        <p className="text-sm font-medium text-text-muted">{formatEventDate(event.startsAt)}</p>
        <Link href={destination} className="inline-flex min-h-10 w-fit items-center rounded-full border border-border px-4 text-sm font-semibold text-text transition-colors hover:border-primary hover:text-primary">
          {detailsLabel}<span className="ml-2" aria-hidden="true">→</span>
        </Link>
      </div>
    </article>
  );
}
