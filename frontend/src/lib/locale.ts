import { phrases } from './phrases';

/** Shared by server and client code (no React here). */
export type Locale = 'kk' | 'ru' | 'en';
export const LOCALE_COOKIE = 'bf_locale';
export type Params = Record<string, string | number | null | undefined>;

export function isLocale(value: unknown): value is Locale {
  return value === 'kk' || value === 'ru' || value === 'en';
}

/**
 * Translate an English phrase. The English text is the key; Kazakh and Russian live in
 * phrases.ts. Placeholders like {name} are filled from params. Unknown phrases fall back to
 * English, and `npm run i18n:check` fails when a phrase used in code is missing.
 */
export function translate(locale: Locale, english: string, params?: Params): string {
  const entry = phrases[english];
  let text = locale === 'en' || !entry ? english : entry[locale] || english;
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      text = text.split(`{${key}}`).join(value == null ? '' : String(value));
    }
  }
  return text;
}
