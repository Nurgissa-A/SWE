import type { ReactNode } from 'react';

type BadgeProps = {
  children: ReactNode;
  tone?: 'default' | 'success' | 'error' | 'muted';
  className?: string;
};

const toneClasses = {
  default: 'bg-secondary text-primary',
  success: 'bg-success/10 text-success',
  error: 'bg-error/10 text-error',
  muted: 'bg-background text-text-muted',
};

export function Badge({ children, tone = 'default', className = '' }: BadgeProps) {
  return (
    <span className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold capitalize ${toneClasses[tone]} ${className}`}>
      {children}
    </span>
  );
}
