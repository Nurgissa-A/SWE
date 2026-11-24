/**
 * Convert between a wall-clock time in an IANA time zone ("2026-12-12T19:00" in Asia/Almaty)
 * and a UTC ISO string, so organizers enter times in the event's own time zone rather than
 * their browser's.
 */
function offsetMinutes(utc: Date, timeZone: string): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone, hourCycle: 'h23', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).formatToParts(utc);
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value);
  const asUtc = Date.UTC(get('year'), get('month') - 1, get('day'), get('hour'), get('minute'), get('second'));
  return Math.round((asUtc - utc.getTime()) / 60000);
}

export function zonedToUtc(local: string, timeZone: string): string {
  const [date, time = '00:00'] = local.split('T');
  const [y, m, d] = date.split('-').map(Number);
  const [hh, mm] = time.split(':').map(Number);
  const guess = new Date(Date.UTC(y, m - 1, d, hh, mm));
  const first = new Date(guess.getTime() - offsetMinutes(guess, timeZone) * 60000);
  // Second pass settles times near daylight-saving changes (none in Kazakhstan, but cheap).
  return new Date(guess.getTime() - offsetMinutes(first, timeZone) * 60000).toISOString();
}

export function utcToZoned(iso: string | null | undefined, timeZone: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return new Date(d.getTime() + offsetMinutes(d, timeZone) * 60000).toISOString().slice(0, 16);
}

/** Time zones of Kazakhstan, with UTC+5 Almaty time as the default. */
export const TIME_ZONES = ['Asia/Almaty', 'Asia/Qyzylorda', 'Asia/Aqtobe', 'Asia/Aqtau', 'Asia/Atyrau', 'Asia/Oral', 'Asia/Qostanay', 'UTC'];
