'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { LOCALE_COOKIE, translate, type Locale, type Params } from './locale';
import { formatDate, formatDateBlock, formatDateTime } from './dates';

export { translate, type Locale, type Params };

export const LOCALES: { code: Locale; label: string; short: string }[] = [
  { code: 'kk', label: 'Қазақша', short: 'ҚАЗ' },
  { code: 'ru', label: 'Русский', short: 'РУС' },
  { code: 'en', label: 'English', short: 'ENG' },
];

let activeLocale: Locale = 'ru';

/** The language currently shown; read by the API client for the X-Locale header. */
export function currentLocale(): Locale {
  return activeLocale;
}

type I18nValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (english: string, params?: Params) => string;
  /** Money in whole tenge, e.g. 8 000 ₸. */
  money: (kzt: number | null | undefined) => string;
  /** Date and time in the event's time zone. */
  dateTime: (iso: string | null | undefined, timeZone?: string, withWeekday?: boolean) => string;
  date: (iso: string | null | undefined, timeZone?: string) => string;
  number: (n: number) => string;
  /** Month and day for the date block on event cards. */
  dateBlock: (iso: string | null | undefined, timeZone?: string) => { month: string; day: string } | null;
};

const I18nContext = createContext<I18nValue | null>(null);

// Kazakh groups digits like Russian (8 000); ru-RU is used because kk ICU data is often missing.
const numberLocale: Record<Locale, string> = { kk: 'ru-RU', ru: 'ru-RU', en: 'en-GB' };

export function I18nProvider({ initialLocale, children }: { initialLocale: Locale; children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(initialLocale);
  activeLocale = locale;

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const setLocale = useCallback((next: Locale) => {
    activeLocale = next;
    setLocaleState(next);
    document.cookie = `${LOCALE_COOKIE}=${next}; path=/; max-age=31536000; samesite=lax`;
  }, []);

  const value = useMemo<I18nValue>(() => {
    const nf = new Intl.NumberFormat(numberLocale[locale]);
    return {
      locale,
      setLocale,
      t: (english, params) => translate(locale, english, params),
      money: (kzt) => (kzt == null ? '—' : `${nf.format(kzt)} ₸`),
      number: (n) => nf.format(n),
      dateTime: (iso, timeZone = 'Asia/Almaty', withWeekday = true) =>
        formatDateTime(locale, iso, timeZone, withWeekday) ?? translate(locale, 'Date to be confirmed'),
      date: (iso, timeZone = 'Asia/Almaty') => formatDate(locale, iso, timeZone) ?? '—',
      dateBlock: (iso, timeZone = 'Asia/Almaty') => formatDateBlock(locale, iso, timeZone),
    };
  }, [locale, setLocale]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const context = useContext(I18nContext);
  if (!context) throw new Error('useI18n must be used inside I18nProvider.');
  return context;
}

export function isLocaleCode(value: unknown): value is Locale {
  return value === 'kk' || value === 'ru' || value === 'en';
}
