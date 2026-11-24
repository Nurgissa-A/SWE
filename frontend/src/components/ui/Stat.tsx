'use client';

import type { ReactNode } from 'react';
import { useI18n } from '@/lib/i18n';

/** Navy stat tile with an Unbounded number (design system). */
export function Stat({ label, value, hint, accent = false }: { label: string; value: ReactNode; hint?: string; accent?: boolean }) {
  const { t } = useI18n();
  return (
    <div className="rounded-card bg-navy p-5">
      <p className="text-sm font-medium text-mist">{t(label)}</p>
      <p className={`mt-2 font-display text-2xl font-bold sm:text-3xl ${accent ? 'text-mint' : 'text-surface'}`}>{value}</p>
      {hint ? <p className="mt-1 text-xs text-mist">{t(hint)}</p> : null}
    </div>
  );
}

/** Light panel with a title, used for dashboard sections. */
export function Panel({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  const { t } = useI18n();
  return (
    <section className="rounded-card border border-border bg-surface p-5 sm:p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-sans text-lg font-bold tracking-normal text-text">{t(title)}</h2>
        {action}
      </div>
      {children}
    </section>
  );
}

/** Notice that payments are simulated (FR-PAY-3). */
export function SimulationNotice({ compact = false }: { compact?: boolean }) {
  const { t } = useI18n();
  return (
    <p role="note" className={`rounded-input border border-[#e6c35c] bg-[#fff7dd] text-[#5c4300] ${compact ? 'px-3 py-2 text-xs' : 'px-4 py-3 text-sm'}`}>
      <strong>{t('Demo payments.')}</strong> {t('No real money moves. Use test card 4242 4242 4242 4242; 4000 0000 0000 0002 is declined.')}
    </p>
  );
}
