'use client';

import type { ReactNode } from 'react';
import type { ApiEvent } from '@/hooks/useEvents';
import { useI18n } from '@/lib/i18n';
import { Badge } from '@/components/ui/Badge';

type EventPreviewProps = { event: ApiEvent; aside?: ReactNode; footer?: ReactNode };

/** Event page body: navy hero with date block, description and practical details. */
export function EventPreview({ event, aside, footer }: EventPreviewProps) {
  const { t, dateBlock, dateTime } = useI18n();
  const statusTone = event.status === 'published' ? 'success' : event.status === 'cancelled' ? 'error' : 'muted';
  const date = dateBlock(event.startsAt, event.timezone ?? undefined);
  const tz = event.timezone ?? 'Asia/Almaty';
  const place = [event.venueName, event.venueAddress, event.city].filter(Boolean).join(', ');

  return (
    <article className="overflow-hidden rounded-card border border-border bg-surface shadow-sm">
      <div className="relative bg-navy p-6 text-surface sm:p-10">
        {event.imageUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={event.imageUrl} alt="" className="absolute inset-0 size-full object-cover opacity-30" />
        ) : null}
        <div className="relative flex flex-col gap-6 sm:flex-row sm:items-end">
          {date ? (
            <div className="grid h-20 w-18 shrink-0 place-content-center rounded-2xl bg-primary text-center text-white">
              <span className="text-xs font-semibold tracking-[0.1em]">{date.month}</span>
              <span className="font-display text-3xl font-bold leading-none">{date.day}</span>
            </div>
          ) : null}
          <div className="max-w-3xl">
            {event.category ? <p className="mb-2 text-xs font-bold uppercase tracking-[0.18em] text-mist">{event.category}</p> : null}
            <h1 className="text-3xl font-bold leading-tight text-surface sm:text-4xl">{event.title}</h1>
            <p className="mt-3 text-mist">{dateTime(event.startsAt, tz)}{place ? ` · ${place}` : ''}</p>
            <div className="mt-4 flex flex-wrap items-center gap-2">
              {event.lifecycle && event.lifecycle !== 'upcoming' ? <Badge tone={statusTone} onDark>{event.lifecycle}</Badge> : null}
              {event.status !== 'published' ? <Badge tone={statusTone} onDark>{event.status ?? 'draft'}</Badge> : null}
              {event.visibility && event.visibility !== 'public' ? <Badge onDark>{event.visibility}</Badge> : null}
            </div>
          </div>
        </div>
      </div>
      <div className="grid gap-8 p-6 sm:p-10 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <section className="grid content-start gap-8">
          <div>
            <h2 className="font-sans text-lg font-bold tracking-normal text-text">{t('About this event')}</h2>
            <p className="mt-3 whitespace-pre-line leading-7 text-text-muted">{event.description?.trim() || t('No description has been added yet.')}</p>
          </div>
          <dl className="grid gap-4 rounded-card border border-border bg-background p-5 sm:grid-cols-2">
            <div>
              <dt className="text-xs font-bold uppercase tracking-[0.14em] text-text-muted">{t('Date and time')}</dt>
              <dd className="mt-1 font-semibold text-text">{dateTime(event.startsAt, tz)}</dd>
              {event.endsAt ? <dd className="mt-1 text-sm text-text-muted">{t('Until {time}', { time: dateTime(event.endsAt, tz, false) })}</dd> : null}
              <dd className="mt-1 text-xs text-text-muted">{t('Time zone: {tz}', { tz })}</dd>
            </div>
            <div>
              <dt className="text-xs font-bold uppercase tracking-[0.14em] text-text-muted">{t('Venue')}</dt>
              <dd className="mt-1 font-semibold text-text">{event.venueName ?? t('Venue to be announced')}</dd>
              {event.venueAddress || event.city ? <dd className="mt-1 text-sm text-text-muted">{[event.venueAddress, event.city].filter(Boolean).join(', ')}</dd> : null}
            </div>
            <div className="sm:col-span-2">
              <dt className="text-xs font-bold uppercase tracking-[0.14em] text-text-muted">{t('Refund policy')}</dt>
              <dd className="mt-1 text-sm text-text">{event.refundPolicy ? t(event.refundPolicy) : '—'}</dd>
            </div>
          </dl>
          {footer}
        </section>
        {aside ? <aside className="h-fit lg:sticky lg:top-24">{aside}</aside> : null}
      </div>
    </article>
  );
}
