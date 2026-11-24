'use client';

import { useI18n } from '@/lib/i18n';
import { Button } from './Button';

type ErrorMessageProps = {
  message: string;
  onRetry?: () => void;
  className?: string;
};

export function ErrorMessage({ message, onRetry, className = '' }: ErrorMessageProps) {
  const { t } = useI18n();
  return (
    <div className={`rounded-card border border-error/25 bg-error/5 p-5 text-sm text-text ${className}`} role="alert">
      <p className="font-semibold text-error">{t('Something needs attention')}</p>
      <p className="mt-1 text-text-muted">{t(message)}</p>
      {onRetry ? <Button className="mt-4" size="sm" variant="secondary" onClick={onRetry}>{t('Try again')}</Button> : null}
    </div>
  );
}
