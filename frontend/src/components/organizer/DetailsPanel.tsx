'use client';

import { api } from '@/lib/api-client';
import { useI18n } from '@/lib/i18n';
import { call } from '@/lib/api-errors';
import { toEventUpdatePayload } from '@/lib/event-payload';
import type { ApiEvent } from '@/lib/types';
import { EventForm } from '@/components/events/EventForm';
import { Panel } from '@/components/ui/Stat';

export function DetailsPanel({ event, onSaved }: { event: ApiEvent; onSaved: (e: ApiEvent) => void }) {
  const { t } = useI18n();
  if (event.status === 'cancelled') return <Panel title="Details"><p className="text-sm text-text-muted">{t('A cancelled event cannot be edited.')}</p></Panel>;
  return (
    <Panel title="Details">
      <EventForm initialEvent={event} isEdit submitLabel="Save changes" onSubmit={async (values) => {
        onSaved(await call(api.PATCH('/api/events/{eventId}', { params: { path: { eventId: event.id } }, body: toEventUpdatePayload(values) })));
      }} />
    </Panel>
  );
}
