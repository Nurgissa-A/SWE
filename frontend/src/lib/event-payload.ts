import type { paths } from './api-types';
import type { EventFormValues } from './validation';

export type CreateEventInput = paths['/api/events']['post']['requestBody']['content']['application/json'];
export type UpdateEventInput = paths['/api/events/{eventId}']['patch']['requestBody']['content']['application/json'];

export function toEventPayload(values: EventFormValues): CreateEventInput {
  return {
    title: values.title.trim(),
    startsAt: new Date(values.startsAt).toISOString(),
    visibility: values.visibility,
    ...(values.description.trim() ? { description: values.description.trim() } : {}),
    ...(values.category.trim() ? { category: values.category.trim() } : {}),
    ...(values.endsAt ? { endsAt: new Date(values.endsAt).toISOString() } : {}),
    ...(values.capacity.trim() ? { capacity: Number(values.capacity) } : {}),
  };
}

export function toEventUpdatePayload(values: EventFormValues): UpdateEventInput {
  return {
    title: values.title.trim(),
    description: values.description.trim(),
    startsAt: new Date(values.startsAt).toISOString(),
    visibility: values.visibility,
    ...(values.endsAt ? { endsAt: new Date(values.endsAt).toISOString() } : {}),
    ...(values.capacity.trim() ? { capacity: Number(values.capacity) } : {}),
  };
}
