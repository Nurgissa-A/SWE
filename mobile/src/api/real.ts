// REAL backend calls (team repo: docs/openapi.yaml).
// Sign-in comes from the identity service (Python); events and check-in from the events service (Go).
import { request } from './http';
import { ApiError } from './errors';
import { IDENTITY_URL, EVENTS_URL } from './config';
import { formatDateTime, formatTime } from '../dates';
import { currentLocale, t } from '../i18n';
import type { User, EventItem, VerifyResult, AttendeeItem } from './mock';

type Counts = { sold: number; checkedIn: number };

// GET /api/scanner/events: only events this account may check people in for
// (its own as organizer, or ones it was added to as door staff).
type ScannerEvent = {
  id: string;
  title: string;
  startsAt: string;
  timezone: string;
  venueName?: string | null;
  city?: string | null;
  status: string;
  role: 'organizer' | 'event_admin';
  canUndo: boolean;
  counts: Counts;
};

type ScannerTicket = {
  id: string;
  code: string;
  attendeeName: string;
  ticketType: string;
  status: 'valid' | 'checked_in' | 'cancelled' | 'refunded';
  checkedInAt?: string | null;
};

type ScanResult = {
  result: 'admitted' | 'already_used' | 'cancelled' | 'refunded' | 'wrong_event' | 'campaign_qr' | 'invalid';
  message: string;
  ticket?: ScannerTicket | null;
  counts: Counts;
};

// Ticket codes look like GMRX-G994. Anything else that is scanned (the signed QR text, a promo
// link, a random QR) goes to the server's scan endpoint, which decides what it is.
const TICKET_CODE = /^[A-Z0-9]{4}-[A-Z0-9]{4}$/;

// Time zone of each event, so check-in times show in the event's local time.
const zones = new Map<string, string>();

function toEventItem(e: ScannerEvent): EventItem {
  zones.set(e.id, e.timezone);
  const place = [e.venueName, e.city].filter(Boolean).join(', ');
  return {
    id: e.id,
    title: e.title,
    date: formatDateTime(currentLocale(), e.startsAt, e.timezone) ?? '',
    venue: e.status === 'published' ? place : [place, t('Not published')].filter(Boolean).join(' · '),
    registered: e.counts.sold,
    checkedIn: e.counts.checkedIn,
    canUndo: e.canUndo,
  };
}

function fromTicket(tk: ScannerTicket, kind: VerifyResult['kind'], eventId: string): VerifyResult {
  return {
    kind,
    code: tk.code,
    ticketId: tk.id,
    attendeeName: tk.attendeeName,
    ticketType: tk.ticketType,
    checkedInAt: tk.checkedInAt ? formatTime(tk.checkedInAt, zones.get(eventId)) : undefined,
  };
}

const STATUS_KIND: Record<ScannerTicket['status'], VerifyResult['kind']> = {
  valid: 'valid', checked_in: 'already_used', cancelled: 'cancelled', refunded: 'refunded',
};

const SCAN_KIND: Record<ScanResult['result'], VerifyResult['kind']> = {
  admitted: 'admitted', already_used: 'already_used', cancelled: 'cancelled', refunded: 'refunded',
  wrong_event: 'wrong_event', campaign_qr: 'campaign', invalid: 'not_found',
};

// POST /api/auth/login -> { token, user }
export async function login(email: string, password: string): Promise<{ token: string; user: User }> {
  try {
    return await request<{ token: string; user: User }>(IDENTITY_URL, '/api/auth/login', {
      method: 'POST',
      body: { email: email.trim(), password },
    });
  } catch (err) {
    if (err instanceof ApiError && err.code === 'unauthorized') {
      throw new ApiError('invalid_credentials', 'Email or password is incorrect.');
    }
    if (err instanceof ApiError && err.code === 'http_403') {
      throw new ApiError('unverified', 'Please verify your email first (check the link we sent you).');
    }
    throw err;
  }
}

// GET /api/users/me -> checks the saved login is still valid
export async function getMe(): Promise<User> {
  return request<User>(IDENTITY_URL, '/api/users/me', { auth: true });
}

export async function getMyEvents(): Promise<EventItem[]> {
  const list = await request<ScannerEvent[]>(EVENTS_URL, '/api/scanner/events', { auth: true });
  return list.map(toEventItem);
}

export async function getEvent(id: string): Promise<EventItem | null> {
  return (await getMyEvents()).find((e) => e.id === id) ?? null;
}

async function findTicket(eventId: string, code: string): Promise<ScannerTicket | undefined> {
  const list = await request<ScannerTicket[]>(EVENTS_URL, `/api/scanner/events/${encodeURIComponent(eventId)}/attendees?q=${encodeURIComponent(code)}`, { auth: true });
  return list.find((tk) => tk.code === code);
}

// Scanned QR: the server checks the signature and checks the ticket in, in one atomic step, so two
// phones scanning the same ticket can never both admit it.
// Ticket code (from attendee search): only looks the ticket up; "Check in" is a separate tap.
export async function verifyTicket(eventId: string, rawCode: string): Promise<VerifyResult> {
  const code = rawCode.trim();
  if (TICKET_CODE.test(code)) {
    const tk = await findTicket(eventId, code);
    return tk ? fromTicket(tk, STATUS_KIND[tk.status], eventId) : { kind: 'not_found', code };
  }
  try {
    const r = await request<ScanResult>(EVENTS_URL, `/api/scanner/events/${encodeURIComponent(eventId)}/scan`, {
      method: 'POST', body: { payload: code }, auth: true,
    });
    const kind = SCAN_KIND[r.result] ?? 'not_found';
    return r.ticket ? fromTicket(r.ticket, kind, eventId) : { kind, code: kind === 'campaign' ? '' : code.slice(0, 40) };
  } catch (err) {
    if (err instanceof ApiError && err.code === 'NOT_PUBLISHED') return { kind: 'closed', code: '' };
    throw err;
  }
}

async function ticketId(eventId: string, ticket: { code: string; ticketId?: string }): Promise<string> {
  if (ticket.ticketId) return ticket.ticketId;
  const tk = await findTicket(eventId, ticket.code);
  if (!tk) throw new ApiError('not_found', 'Ticket not found');
  return tk.id;
}

export async function checkIn(eventId: string, ticket: { code: string; ticketId?: string }): Promise<void> {
  const r = await request<ScanResult>(EVENTS_URL, `/api/scanner/events/${encodeURIComponent(eventId)}/check-in`, {
    method: 'POST', body: { ticketId: await ticketId(eventId, ticket) }, auth: true,
  });
  if (r.result !== 'admitted') {
    throw new ApiError('not_checkable', 'This ticket can no longer be checked in. Scan it again to see why.');
  }
}

export async function undoCheckIn(eventId: string, ticket: { code: string; ticketId?: string }): Promise<void> {
  await request(EVENTS_URL, `/api/scanner/events/${encodeURIComponent(eventId)}/undo`, {
    method: 'POST', body: { ticketId: await ticketId(eventId, ticket) }, auth: true,
  });
}

export async function searchAttendees(eventId: string, query: string): Promise<AttendeeItem[]> {
  const list = await request<ScannerTicket[]>(EVENTS_URL, `/api/scanner/events/${encodeURIComponent(eventId)}/attendees?q=${encodeURIComponent(query.trim())}`, { auth: true });
  return list.map((tk) => ({ id: tk.id, code: tk.code, attendeeName: tk.attendeeName, ticketType: tk.ticketType, status: tk.status }));
}

// Door staff in BiletFlow are ordinary accounts that an organizer added to an event, so "staff"
// means: organizer / event admin role, or at least one event to check people in for.
export async function canUseCheckIn(user: User): Promise<boolean> {
  if (user.role === 'organizer' || user.role === 'event_admin' || user.role === 'platform_admin') return true;
  return (await getMyEvents()).length > 0;
}
