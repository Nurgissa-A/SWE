'use client';

import { useEffect, useState } from 'react';
import type { ApiEvent } from '@/hooks/useEvents';
import { getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import { validateEventForm, type EventFormValues, type FieldErrors } from '@/lib/validation';
import { TIME_ZONES, utcToZoned } from '@/lib/zoned-time';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Select } from '@/components/ui/Select';
import { TextArea } from '@/components/ui/TextArea';
import { TextInput } from '@/components/ui/TextInput';

const emptyValues: EventFormValues = {
  title: '', description: '', category: '', imageUrl: '', startsAt: '', endsAt: '', timezone: 'Asia/Almaty',
  venueName: '', venueAddress: '', city: '', capacity: '', visibility: 'public',
  refundPolicy: 'No refunds unless the event is cancelled.', registrationOpensAt: '', registrationClosesAt: '',
};

type EventFormProps = {
  initialEvent?: ApiEvent;
  submitLabel: string;
  isEdit?: boolean;
  onSubmit: (values: EventFormValues) => Promise<void>;
};

function eventToValues(event?: ApiEvent): EventFormValues {
  if (!event) return emptyValues;
  const tz = event.timezone || 'Asia/Almaty';
  return {
    title: event.title ?? '', description: event.description ?? '', category: event.category ?? '', imageUrl: event.imageUrl ?? '',
    startsAt: utcToZoned(event.startsAt, tz), endsAt: utcToZoned(event.endsAt, tz), timezone: tz,
    venueName: event.venueName ?? '', venueAddress: event.venueAddress ?? '', city: event.city ?? '',
    capacity: event.capacity == null ? '' : String(event.capacity),
    visibility: event.visibility === 'unlisted' || event.visibility === 'private' ? event.visibility : 'public',
    refundPolicy: event.refundPolicy ?? '', registrationOpensAt: utcToZoned(event.registrationOpensAt, tz),
    registrationClosesAt: utcToZoned(event.registrationClosesAt, tz),
  };
}

export function EventForm({ initialEvent, submitLabel, isEdit = false, onSubmit }: EventFormProps) {
  const { t } = useI18n();
  const [values, setValues] = useState<EventFormValues>(() => eventToValues(initialEvent));
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [submitError, setSubmitError] = useState('');
  const [saved, setSaved] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (initialEvent) setValues(eventToValues(initialEvent));
  }, [initialEvent]);

  function update<K extends keyof EventFormValues>(key: K, value: EventFormValues[K]) {
    setSaved(false);
    setValues((current) => ({ ...current, [key]: value }));
  }

  async function handleSubmit() {
    const errors = validateEventForm(values);
    setFieldErrors(errors);
    setSubmitError('');
    if (Object.keys(errors).length > 0) return;
    setIsSubmitting(true);
    try {
      await onSubmit(values);
      setSaved(true);
    } catch (error) {
      setSubmitError(getApiErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  const published = initialEvent?.status === 'published';
  return (
    <Form onSubmit={handleSubmit} className="grid gap-6">
      {submitError ? <ErrorMessage message={submitError} /> : null}
      <fieldset className="grid gap-5">
        <legend className="mb-1 text-sm font-bold uppercase tracking-[0.14em] text-text-muted">{t('Basics')}</legend>
        <TextInput label="Event title" required maxLength={200} value={values.title} onChange={(e) => update('title', e.currentTarget.value)} error={fieldErrors.title} />
        <TextArea label="Description" value={values.description} onChange={(e) => update('description', e.currentTarget.value)} helpText="Help people understand what makes this event special." />
        <div className="grid gap-5 sm:grid-cols-2">
          <TextInput label="Category" value={values.category} onChange={(e) => update('category', e.currentTarget.value)} helpText="For example, music, art, or community." />
          <TextInput label="Image address (optional)" type="url" value={values.imageUrl} onChange={(e) => update('imageUrl', e.currentTarget.value)} error={fieldErrors.imageUrl} placeholder="https://…" />
        </div>
      </fieldset>
      <fieldset className="grid gap-5">
        <legend className="mb-1 text-sm font-bold uppercase tracking-[0.14em] text-text-muted">{t('When')}</legend>
        <div className="grid gap-5 sm:grid-cols-3">
          <TextInput label="Starts at" type="datetime-local" required value={values.startsAt} onChange={(e) => update('startsAt', e.currentTarget.value)} error={fieldErrors.startsAt} />
          <TextInput label="Ends at" type="datetime-local" value={values.endsAt} onChange={(e) => update('endsAt', e.currentTarget.value)} error={fieldErrors.endsAt} />
          <Select label="Time zone" value={values.timezone} onChange={(e) => update('timezone', e.currentTarget.value)} helpText="Times above are in this time zone.">
            {TIME_ZONES.map((z) => <option key={z} value={z}>{z}</option>)}
          </Select>
        </div>
        {published ? <p className="-mt-2 text-sm text-text-muted">{t('Changing the date, time or venue notifies everyone who has a ticket.')}</p> : null}
      </fieldset>
      <fieldset className="grid gap-5">
        <legend className="mb-1 text-sm font-bold uppercase tracking-[0.14em] text-text-muted">{t('Where')}</legend>
        <div className="grid gap-5 sm:grid-cols-2">
          <TextInput label="Venue name" value={values.venueName} onChange={(e) => update('venueName', e.currentTarget.value)} helpText="Required before publishing." />
          <TextInput label="City" value={values.city} onChange={(e) => update('city', e.currentTarget.value)} helpText="Required before publishing." />
        </div>
        <TextInput label="Address" value={values.venueAddress} onChange={(e) => update('venueAddress', e.currentTarget.value)} />
      </fieldset>
      <fieldset className="grid gap-5">
        <legend className="mb-1 text-sm font-bold uppercase tracking-[0.14em] text-text-muted">{t('Access and rules')}</legend>
        <div className="grid gap-5 sm:grid-cols-2">
          <TextInput label="Capacity" type="number" min="1" step="1" value={values.capacity} onChange={(e) => update('capacity', e.currentTarget.value)} error={fieldErrors.capacity} helpText="Optional limit for the whole event, across all ticket types." />
          <Select label="Visibility" value={values.visibility} onChange={(e) => update('visibility', e.currentTarget.value as EventFormValues['visibility'])}>
            <option value="public">{t('Public — discoverable by everyone')}</option>
            <option value="unlisted">{t('Unlisted — link only')}</option>
            <option value="private">{t('Private — secret link with a key')}</option>
          </Select>
          <TextInput label="Registration opens" type="datetime-local" value={values.registrationOpensAt} onChange={(e) => update('registrationOpensAt', e.currentTarget.value)} helpText="Optional." />
          <TextInput label="Registration closes" type="datetime-local" value={values.registrationClosesAt} onChange={(e) => update('registrationClosesAt', e.currentTarget.value)} error={fieldErrors.registrationClosesAt} helpText="Optional; defaults to the end of the event." />
        </div>
        <TextArea label="Refund policy" value={values.refundPolicy} onChange={(e) => update('refundPolicy', e.currentTarget.value)} helpText="Shown to attendees before they buy." />
      </fieldset>
      <div className="flex flex-col-reverse items-center gap-3 border-t border-border pt-5 sm:flex-row sm:justify-end">
        {saved && isEdit ? <p role="status" className="text-sm font-semibold text-success">{t('Saved')}</p> : null}
        <Button type="submit" size="lg" loading={isSubmitting}>{t(submitLabel)}</Button>
      </div>
    </Form>
  );
}
