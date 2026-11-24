'use client';

import { useI18n } from '@/lib/i18n';

type SpinnerProps = {
  label?: string;
  className?: string;
};

export function Spinner({ label = 'Loading', className = '' }: SpinnerProps) {
  const { t } = useI18n();
  return (
    <span className={`inline-flex items-center gap-3 text-sm text-text-muted ${className}`} role="status" aria-live="polite">
      <span className="size-5 animate-spin rounded-full border-2 border-primary border-r-transparent" aria-hidden="true" />
      <span>{t(label)}</span>
    </span>
  );
}
