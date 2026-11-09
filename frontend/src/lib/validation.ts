export type FieldErrors = Record<string, string>;

export type EventFormValues = {
  title: string;
  description: string;
  category: string;
  startsAt: string;
  endsAt: string;
  capacity: string;
  visibility: 'public' | 'unlisted' | 'private';
};

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function validateEmail(email: string): string | undefined {
  if (!email.trim()) return 'Enter your email address.';
  if (!emailPattern.test(email.trim())) return 'Enter a valid email address.';
  if (email.length > 254) return 'Email addresses must be 254 characters or fewer.';
  return undefined;
}

export function validatePassword(password: string): string | undefined {
  if (!password) return 'Enter your password.';
  if (password.length < 8) return 'Use at least 8 characters.';
  if (password.length > 1024) return 'Password is too long.';
  return undefined;
}

export function validateEventForm(values: EventFormValues): FieldErrors {
  const errors: FieldErrors = {};

  if (!values.title.trim()) {
    errors.title = 'Add an event title.';
  }

  if (!values.startsAt || Number.isNaN(new Date(values.startsAt).getTime())) {
    errors.startsAt = 'Choose a valid start date and time.';
  }

  if (values.endsAt) {
    const startsAt = new Date(values.startsAt).getTime();
    const endsAt = new Date(values.endsAt).getTime();
    if (!Number.isNaN(startsAt) && (Number.isNaN(endsAt) || endsAt <= startsAt)) {
      errors.endsAt = 'The end date must be after the start date.';
    }
  }

  if (values.capacity.trim()) {
    const capacity = Number(values.capacity);
    if (!Number.isInteger(capacity) || capacity < 0) {
      errors.capacity = 'Capacity must be a non-negative whole number.';
    }
  }

  return errors;
}
