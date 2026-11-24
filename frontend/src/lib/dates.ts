import type { Locale } from './locale';

/*
 * Dates are composed from our own month and weekday names instead of Intl's
 * locale data: many browsers (and headless Chromium) ship without Kazakh ICU
 * data and fall back to forms like "2026 M12 6". Intl is only used to read the
 * numeric parts in the event's time zone, which every browser supports.
 */

const MONTHS: Record<Locale, string[]> = {
  kk: ['қаңтар', 'ақпан', 'наурыз', 'сәуір', 'мамыр', 'маусым', 'шілде', 'тамыз', 'қыркүйек', 'қазан', 'қараша', 'желтоқсан'],
  // Genitive forms, as used after a day number ("6 декабря").
  ru: ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'],
  en: ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'],
};

const MONTH_BLOCK: Record<Locale, string[]> = {
  kk: ['ҚАҢ', 'АҚП', 'НАУ', 'СӘУ', 'МАМ', 'МАУ', 'ШІЛ', 'ТАМ', 'ҚЫР', 'ҚАЗ', 'ҚАР', 'ЖЕЛ'],
  ru: ['ЯНВ', 'ФЕВ', 'МАР', 'АПР', 'МАЙ', 'ИЮН', 'ИЮЛ', 'АВГ', 'СЕН', 'ОКТ', 'НОЯ', 'ДЕК'],
  en: ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'],
};

// Sunday first, matching Date#getDay.
const WEEKDAYS: Record<Locale, string[]> = {
  kk: ['Жс', 'Дс', 'Сс', 'Ср', 'Бс', 'Жм', 'Сб'],
  ru: ['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'],
  en: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'],
};

type Parts = { year: number; month: number; day: number; weekday: number; hour: string; minute: string };

const partFormatters = new Map<string, Intl.DateTimeFormat>();

/** Calendar parts of an instant in the given time zone (month is 0-based). */
export function zonedParts(date: Date, timeZone = 'Asia/Almaty'): Parts {
  let f = partFormatters.get(timeZone);
  if (!f) {
    f = new Intl.DateTimeFormat('en-US', {
      timeZone, year: 'numeric', month: 'numeric', day: 'numeric', weekday: 'short', hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
    });
    partFormatters.set(timeZone, f);
  }
  const p: Record<string, string> = {};
  for (const { type, value } of f.formatToParts(date)) p[type] = value;
  return {
    year: Number(p.year), month: Number(p.month) - 1, day: Number(p.day),
    weekday: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].indexOf(p.weekday),
    hour: p.hour === '24' ? '00' : p.hour, minute: p.minute,
  };
}

function parse(iso: string | null | undefined): Date | null {
  if (!iso) return null;
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? null : d;
}

/** "Sun, 6 December 2026, 19:00" / "Вс, 6 декабря 2026 г., 19:00" / "Жс, 2026 ж. 6 желтоқсан, 19:00". */
export function formatDateTime(locale: Locale, iso: string | null | undefined, timeZone?: string, withWeekday = true): string | null {
  const d = parse(iso);
  if (!d) return null;
  const p = zonedParts(d, timeZone);
  const time = `${p.hour}:${p.minute}`;
  const weekday = withWeekday ? `${WEEKDAYS[locale][p.weekday]}, ` : '';
  const month = MONTHS[locale][p.month];
  if (locale === 'kk') return `${weekday}${p.year} ж. ${p.day} ${month}, ${time}`;
  if (locale === 'ru') return `${weekday}${p.day} ${month} ${p.year} г., ${time}`;
  return `${weekday}${p.day} ${month} ${p.year}, ${time}`;
}

/** Compact date: "06.12.2026" for kk/ru, "6 Dec 2026" for English. */
export function formatDate(locale: Locale, iso: string | null | undefined, timeZone?: string): string | null {
  const d = parse(iso);
  if (!d) return null;
  const p = zonedParts(d, timeZone);
  if (locale === 'en') return `${p.day} ${MONTHS.en[p.month].slice(0, 3)} ${p.year}`;
  return `${String(p.day).padStart(2, '0')}.${String(p.month + 1).padStart(2, '0')}.${p.year}`;
}

/** Month and day for the date block on event cards. */
export function formatDateBlock(locale: Locale, iso: string | null | undefined, timeZone?: string): { month: string; day: string } | null {
  const d = parse(iso);
  if (!d) return null;
  const p = zonedParts(d, timeZone);
  return { month: MONTH_BLOCK[locale][p.month], day: String(p.day).padStart(2, '0') };
}
