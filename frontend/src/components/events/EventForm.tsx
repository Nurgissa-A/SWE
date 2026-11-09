'use client';

import { useEffect, useState, type FormEvent } from 'react';
import type { ApiEvent } from '@/hooks/useEvents';
import { getApiErrorMessage } from '@/lib/api-errors';
import { toLocalDateTimeInput } from '@/lib/format';
import { validateEventForm, type EventFormValues, type FieldErrors } from '@/lib/validation';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Select } from '@/components/ui/Select';
import { TextArea } from '@/components/ui/TextArea';
import { TextInput } from '@/components/ui/TextInput';

const emptyValues: EventFormValues = {
  title: '',
  description: '',
  category: '',
  startsAt: '',
  endsAt: '',
  capacity: '',
  visibility: 'public',
};

type EventFormProps = {
  initialEvent?: ApiEvent;
  submitLabel: string;
  isEdit?: boolean;
  onSubmit: (values: EventFormValues) => Promise<void>;
};

function eventToValues(event?: ApiEvent): EventFormValues {
  if (!event) return emptyValues;
  const visibility = event.visibility === 'unlisted' || event.visibility === 'private' ? event.visibility : 'public';
  return {
    title: event.title ?? '',
    description: event.description ?? '',
    category: event.category ?? '',
    startsAt: toLocalDateTimeInput(event.startsAt),
    endsAt: toLocalDateTimeInput(event.endsAt),
    capacity: event.capacity == null ? '' : String(event.capacity),
    visibility,
  };
}

export function EventForm({ initialEvent, submitLabel, isEdit = false, onSubmit }: EventFormProps) {
  const [values, setValues] = useState<EventFormValues>(() => eventToValues(initialEvent));
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [submitError, setSubmitError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (initialEvent) setValues(eventToValues(initialEvent));
  }, [initialEvent]);

  function update<K extends keyof EventFormValues>(key: K, value: EventFormValues[K]) {
    setValues((current) => ({ ...current, [key]: value }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    const errors = validateEventForm(values);
    if (isEdit && initialEvent?.endsAt && !values.endsAt) {
      errors.endsAt = 'The current API does not support clearing an event end time.';
    }
    setFieldErrors(errors);
    setSubmitError('');
    if (Object.keys(errors).length > 0) return;

    setIsSubmitting(true);
    try {
      await onSubmit(values);
    } catch (error) {
      setSubmitError(getApiErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Form onSubmit={handleSubmit} className="grid gap-6">
      {submitError ? <ErrorMessage message={submitError} /> : null}
      <TextInput label="Event title" required maxLength={200} value={values.title} onChange={(event) => update('title', event.currentTarget.value)} error={fieldErrors.title} />
      <TextArea label="Description" value={values.description} onChange={(event) => update('description', event.currentTarget.value)} helpText="Help people understand what makes this event special." />
      <TextInput label="Category" value={values.category} onChange={(event) => update('category', event.currentTarget.value)} disabled={isEdit} helpText={isEdit ? 'Category changes are not included in the current event update endpoint.' : 'For example, music, art, or community.'} />
      <div className="grid gap-5 sm:grid-cols-2">
        <TextInput label="Starts at" type="datetime-local" required value={values.startsAt} onChange={(event) => update('startsAt', event.currentTarget.value)} error={fieldErrors.startsAt} />
        <TextInput label="Ends at" type="datetime-local" required={isEdit && Boolean(initialEvent?.endsAt)} value={values.endsAt} onChange={(event) => update('endsAt', event.currentTarget.value)} error={fieldErrors.endsAt} helpText={isEdit && initialEvent?.endsAt ? 'An existing end time cannot be cleared with the current update endpoint.' : undefined} />
      </div>
      <p className="-mt-3 text-sm text-text-muted">Times are entered in your browser&apos;s local timezone; event listings display Almaty time.</p>
      <div className="grid gap-5 sm:grid-cols-2">
        <TextInput label="Capacity" type="number" min="0" step="1" value={values.capacity} onChange={(event) => update('capacity', event.currentTarget.value)} error={fieldErrors.capacity} helpText={isEdit ? 'Leave blank to keep the current capacity; the update endpoint cannot clear it.' : 'Optional; use a whole number.'} />
        <Select label="Visibility" value={values.visibility} onChange={(event) => update('visibility', event.currentTarget.value as EventFormValues['visibility'])}>
          <option value="public">Public — discoverable by everyone</option>
          <option value="unlisted">Unlisted — link only</option>
          <option value="private">Private — organizer access</option>
        </Select>
      </div>
      <div className="flex flex-col-reverse gap-3 border-t border-border pt-5 sm:flex-row sm:justify-end">
        <Button type="submit" size="lg" loading={isSubmitting}>{submitLabel}</Button>
      </div>
    </Form>
  );
}

