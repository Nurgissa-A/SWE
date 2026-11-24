'use client';

import Link from 'next/link';
import type { components } from '@/lib/api-types';
import { useI18n } from '@/lib/i18n';
import { Badge } from '@/components/ui/Badge';

type EventCardData = Pick<components['schemas']['Event'], 'id' | 'title' | 'startsAt' | 'category' | 'status' | 'timezone' | 'city' | 'venueName' | 'priceFromKzt' | 'ticketsLeft' | 'lifecycle'>;
type EventCardProps = { event: EventCardData; href?: string; detailsLabel?: string };

function statusTone(status: string | undefined): 'default' | 'success' | 'error' | 'muted' {
  if (status === 'published') return 'success';
  if (status === 'cancelled') return 'error';
  if (status === 'draft') return 'muted';
  return 'default';
}

export function EventCard({ event, href, detailsLabel = 'View details' }: EventCardProps) {
  const { t, dateBlock, dateTime, money } = useI18n();
  const destination = href ?? `/events/${event.id}`;
  const date = dateBlock(event.startsAt, event.timezone ?? undefined);

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
          {event.status && event.status !== 'published' ? <Badge tone={statusTone(event.status)} onDark>{event.status}</Badge>
            : event.ticketsLeft === 0 ? <Badge tone="error" onDark>{t('Sold out')}</Badge> : event.lifecycle === 'active' ? <Badge tone="success" onDark>{t('Happening now')}</Badge> : null}
        </div>
        <div>
          {event.category ? <p className="mb-2 text-xs font-bold uppercase tracking-[0.15em] text-mist">{event.category}</p> : null}
          <p className="text-xl font-bold leading-tight tracking-tight text-surface sm:text-2xl">{event.title}</p>
        </div>
      </div>
      <div className="grid flex-1 content-between gap-4 p-5">
        <div className="grid gap-1 text-sm">
          <p className="font-medium text-text">{dateTime(event.startsAt, event.timezone)}</p>
          {event.venueName || event.city ? <p className="text-text-muted">{[event.venueName, event.city].filter(Boolean).join(', ')}</p> : null}
          {event.priceFromKzt != null ? <p className="font-semibold text-text">{event.priceFromKzt === 0 ? t('Free') : t('From {price}', { price: money(event.priceFromKzt) })}</p> : null}
        </div>
        <Link href={destination} className="inline-flex min-h-11 w-fit items-center rounded-full border border-border px-4 text-sm font-semibold text-text transition-colors hover:border-primary hover:text-primary">
          {t(detailsLabel)}<span className="ml-2" aria-hidden="true">→</span>
        </Link>
      </div>
    </article>
  );
}
