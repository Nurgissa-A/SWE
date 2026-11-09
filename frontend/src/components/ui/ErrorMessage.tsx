'use client';

import { Button } from './Button';

type ErrorMessageProps = {
  message: string;
  onRetry?: () => void;
  className?: string;
};

export function ErrorMessage({ message, onRetry, className = '' }: ErrorMessageProps) {
  return (
    <div className={`rounded-card border border-error/25 bg-error/5 p-5 text-sm text-text ${className}`} role="alert">
      <p className="font-semibold text-error">Something needs attention</p>
      <p className="mt-1 text-text-muted">{message}</p>
      {onRetry ? <Button className="mt-4" size="sm" variant="secondary" onClick={onRetry}>Try again</Button> : null}
    </div>
  );
}
