import type { ReactNode } from 'react';

type BadgeProps = {
  children: ReactNode;
  tone?: 'default' | 'success' | 'error' | 'muted';
  /** Use the navy-background palette (event card headers, dark panels). */
  onDark?: boolean;
  className?: string;
};

const toneClasses = {
  default: 'bg-secondary text-primary',
  success: 'bg-success/10 text-success',
  error: 'bg-error/10 text-error',
  muted: 'bg-background text-text-muted',
};

const darkToneClasses = {
  default: 'bg-navy-raised text-mist',
  success: 'bg-mint/15 text-mint',
  error: 'bg-[#4a2230] text-[#ffb4a8]',
  muted: 'bg-navy-raised text-mist',
};

export function Badge({ children, tone = 'default', onDark = false, className = '' }: BadgeProps) {
  const tones = onDark ? darkToneClasses : toneClasses;
  return (
    <span className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold capitalize ${tones[tone]} ${className}`}>
      {children}
    </span>
  );
}
