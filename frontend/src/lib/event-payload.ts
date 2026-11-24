import type { paths } from './api-types';
import type { EventFormValues } from './validation';
import { zonedToUtc } from './zoned-time';

export type CreateEventInput = paths['/api/events']['post']['requestBody']['content']['application/json'];
export type UpdateEventInput = paths['/api/events/{eventId}']['patch']['requestBody']['content']['application/json'];

const text = (v: string) => v.trim() || null;

/** Times are entered as wall-clock time in the event's time zone. */
export function toEventPayload(values: EventFormValues): CreateEventInput {
  const tz = values.timezone || 'Asia/Almaty';
  const at = (v: string) => (v ? zonedToUtc(v, tz) : null);
  return {
    title: values.title.trim(),
    startsAt: zonedToUtc(values.startsAt, tz),
    endsAt: at(values.endsAt),
    timezone: tz,
    visibility: values.visibility,
    description: text(values.description),
    category: text(values.category),
    imageUrl: text(values.imageUrl),
    venueName: text(values.venueName),
    venueAddress: text(values.venueAddress),
    city: text(values.city),
    capacity: values.capacity.trim() ? Number(values.capacity) : null,
    refundPolicy: values.refundPolicy.trim() || undefined,
    registrationOpensAt: at(values.registrationOpensAt),
    registrationClosesAt: at(values.registrationClosesAt),
  };
}

export function toEventUpdatePayload(values: EventFormValues): UpdateEventInput {
  const payload = toEventPayload(values);
  // PATCH changes only the fields it receives. Empty text clears a text field (the API treats
  // "" as "remove"); times and capacity are sent only when set.
  const textFields = { description: values.description, category: values.category, imageUrl: values.imageUrl,
    venueName: values.venueName, venueAddress: values.venueAddress, city: values.city };
  const set = Object.fromEntries(Object.entries(payload).filter(([, v]) => v !== null && v !== undefined));
  return { ...set, ...Object.fromEntries(Object.entries(textFields).map(([k, v]) => [k, v.trim()])) } as UpdateEventInput;
}
