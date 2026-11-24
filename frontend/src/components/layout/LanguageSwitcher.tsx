'use client';

import { useEffect, useRef } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { authApi } from '@/lib/api-client';
import { LOCALES, isLocaleCode, useI18n, type Locale } from '@/lib/i18n';

/** Language picker in the header. Signed-in users also get it saved to their account. */
export function LanguageSwitcher({ onDark = true }: { onDark?: boolean }) {
  const { locale, setLocale, t } = useI18n();
  const { user } = useAuth();
  const synced = useRef<string | null>(null);

  // When someone signs in, their saved language wins once per session.
  useEffect(() => {
    if (!user || synced.current === user.id) return;
    synced.current = user.id;
    const saved = (user as { locale?: string }).locale;
    if (isLocaleCode(saved) && saved !== locale) setLocale(saved);
  }, [user, locale, setLocale]);

  async function change(next: Locale) {
    setLocale(next);
if (user) await authApi.PATCH('/api/users/me', { body: { locale: next } as any }).catch(() => undefined);  }

  return (
    <label className="relative inline-flex items-center">
      <span className="sr-only">{t('Language')}</span>
      <select
        value={locale}
        onChange={(event) => void change(event.currentTarget.value as Locale)}
        className={`min-h-10 appearance-none rounded-full border px-3 pr-7 text-sm font-semibold ${onDark ? 'border-navy-line bg-navy text-mist hover:text-white' : 'border-border bg-surface text-text'}`}
      >
        {LOCALES.map((l) => <option key={l.code} value={l.code}>{l.short}</option>)}
      </select>
      <svg aria-hidden="true" className={`pointer-events-none absolute right-2.5 size-3 ${onDark ? 'text-mist' : 'text-text-muted'}`} viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2"><path d="m3 4.5 3 3 3-3" /></svg>
    </label>
  );
}
