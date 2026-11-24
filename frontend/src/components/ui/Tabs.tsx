'use client';

import { useI18n } from '@/lib/i18n';

type Tab = { id: string; label: string; count?: number };

/** Accessible tab list; panels are rendered by the parent for the active id. */
export function Tabs({ tabs, active, onChange, label }: { tabs: Tab[]; active: string; onChange: (id: string) => void; label: string }) {
  const { t } = useI18n();
  return (
    <div role="tablist" aria-label={t(label)} className="flex gap-1 overflow-x-auto rounded-card border border-border bg-surface p-1.5">
      {tabs.map((tab) => (
        <button key={tab.id} type="button" role="tab" aria-selected={active === tab.id} onClick={() => onChange(tab.id)}
          className={`min-h-10 whitespace-nowrap rounded-input px-4 text-sm font-semibold transition-colors ${active === tab.id ? 'bg-navy text-white' : 'text-text-muted hover:bg-background hover:text-text'}`}>
          {t(tab.label)}{tab.count != null ? <span className="ml-1.5 opacity-70">{tab.count}</span> : null}
        </button>
      ))}
    </div>
  );
}
