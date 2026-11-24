// FAKE backend for now. Later, only this file changes to call the real backend.
import { ApiError } from './errors';

// ⚙️ Testing switch: set to true to pretend the phone has no internet
const SIMULATE_OFFLINE = false;

// ---------- Types (match docs/openapi.yaml in the team repo) ----------

export type User = {
  id: string;
  email: string;
  role: 'attendee' | 'organizer' | 'event_admin' | 'platform_admin';
  displayName: string | null;
};

export type EventItem = {
  id: string;
  title: string;
  date: string;
  venue: string;
  registered: number | null; // null = not available from the backend yet
  checkedIn: number | null;
  canUndo?: boolean; // real backend: may this account undo check-ins here? (fake data: always)
};

type TicketStatus = 'valid' | 'checked_in' | 'cancelled' | 'refunded';

type Ticket = {
  code: string;
  eventId: string;
  attendeeName: string;
  ticketType: string;
  status: TicketStatus;
  checkedInAt?: string;
};

export type VerifyResult = {
  // 'admitted' = the real backend checked the ticket in as part of the scan (one step, no double entry)
  // 'closed' = the event is not published, so nobody can be checked in
  kind: 'valid' | 'admitted' | 'already_used' | 'cancelled' | 'refunded' | 'wrong_event' | 'campaign' | 'not_found' | 'closed';
  code: string; // the ticket code (e.g. GMRX-G994), not the scanned QR text
  ticketId?: string; // real backend only
  attendeeName?: string;
  ticketType?: string;
  checkedInAt?: string;
};

export type AttendeeItem = {
  id?: string; // real backend only
  code: string;
  attendeeName: string;
  ticketType: string;
  status: string;
};

// ---------- Fake data ----------

const accounts: { password: string; user: User }[] = [
  { password: 'admin123', user: { id: 'u1', email: 'admin@biletflow.kz', role: 'event_admin', displayName: 'Event Admin' } },
  { password: 'org123', user: { id: 'u2', email: 'organizer@biletflow.kz', role: 'organizer', displayName: 'Organizer' } },
  { password: 'att123', user: { id: 'u3', email: 'attendee@biletflow.kz', role: 'attendee', displayName: 'Attendee' } },
];

const events: EventItem[] = [
  { id: '1', title: 'NU Tech Fest', date: '20 Nov 2026, 18:00', venue: 'NU Main Atrium', registered: 120, checkedIn: 45 },
  { id: '2', title: 'Robotics Club Demo Day', date: '25 Nov 2026, 15:00', venue: 'Block C, Room 3', registered: 60, checkedIn: 0 },
  { id: '3', title: 'Winter Concert', date: '12 Dec 2026, 19:00', venue: 'Astana Concert Hall', registered: 300, checkedIn: 0 },
];

const tickets: Ticket[] = [
  { code: 'BF-T-1001', eventId: '1', attendeeName: 'Dana Sultanova', ticketType: 'General Admission', status: 'valid' },
  { code: 'BF-T-1002', eventId: '1', attendeeName: 'Arman Bekov', ticketType: 'VIP', status: 'checked_in', checkedInAt: '18:05' },
  { code: 'BF-T-1003', eventId: '1', attendeeName: 'Aliya Nurlanova', ticketType: 'General Admission', status: 'cancelled' },
  { code: 'BF-T-1004', eventId: '1', attendeeName: 'Timur Kassym', ticketType: 'Student', status: 'refunded' },
  { code: 'BF-T-2001', eventId: '2', attendeeName: 'Madina Ospan', ticketType: 'General Admission', status: 'valid' },
];

// Pretend the network takes a moment (and can fail)
const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

async function fakeNetwork(ms: number) {
  await wait(ms);
  if (SIMULATE_OFFLINE) {
    throw new ApiError('network', 'No internet connection. Check-in needs internet. Check Wi-Fi or mobile data and try again.');
  }
}

// ---------- "API" functions ----------

// Real version: POST /api/auth/login -> { token, user }
export async function login(email: string, password: string): Promise<{ token: string; user: User }> {
  await fakeNetwork(600);
  const acc = accounts.find((a) => a.user.email === email.trim().toLowerCase());
  if (!acc || acc.password !== password) {
    throw new ApiError('invalid_credentials', 'Email or password is incorrect.');
  }
  return { token: `fake-token-${acc.user.id}`, user: acc.user };
}

export async function getMyEvents(): Promise<EventItem[]> {
  await fakeNetwork(500);
  return events.map((e) => ({ ...e }));
}

export async function getEvent(id: string): Promise<EventItem | null> {
  await fakeNetwork(300);
  const e = events.find((ev) => ev.id === id);
  return e ? { ...e } : null;
}

export async function verifyTicket(eventId: string, rawCode: string): Promise<VerifyResult> {
  await fakeNetwork(400);
  const code = rawCode.trim();

  // Promo / campaign QR codes must NEVER let someone in (SRS 4.14)
  if (code.startsWith('BF-CAMPAIGN-') || code.includes('promo=')) {
    return { kind: 'campaign', code };
  }

  const t = tickets.find((tk) => tk.code === code);
  if (!t) return { kind: 'not_found', code };

  const info = { code, attendeeName: t.attendeeName, ticketType: t.ticketType };
  if (t.eventId !== eventId) return { kind: 'wrong_event', ...info };
  if (t.status === 'checked_in') return { kind: 'already_used', ...info, checkedInAt: t.checkedInAt };
  if (t.status === 'cancelled') return { kind: 'cancelled', ...info };
  if (t.status === 'refunded') return { kind: 'refunded', ...info };
  return { kind: 'valid', ...info };
}

export async function checkIn(code: string): Promise<void> {
  await fakeNetwork(300);
  const t = tickets.find((tk) => tk.code === code);
  if (!t || t.status !== 'valid') {
    throw new ApiError('not_checkable', 'This ticket can no longer be checked in. Scan it again to see why.');
  }
  t.status = 'checked_in';
  t.checkedInAt = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  const e = events.find((ev) => ev.id === t.eventId);
  if (e && e.checkedIn !== null) e.checkedIn += 1;
}

export async function undoCheckIn(code: string): Promise<void> {
  await fakeNetwork(300);
  const t = tickets.find((tk) => tk.code === code);
  if (!t || t.status !== 'checked_in') return;
  t.status = 'valid';
  t.checkedInAt = undefined;
  const e = events.find((ev) => ev.id === t.eventId);
  if (e && e.checkedIn !== null && e.checkedIn > 0) e.checkedIn -= 1;
}

export async function searchAttendees(eventId: string, query: string): Promise<AttendeeItem[]> {
  await fakeNetwork(300);
  const q = query.trim().toLowerCase();
  return tickets
    .filter((t) => t.eventId === eventId)
    .filter((t) => !q || t.attendeeName.toLowerCase().includes(q) || t.code.toLowerCase().includes(q))
    .map((t) => ({ code: t.code, attendeeName: t.attendeeName, ticketType: t.ticketType, status: t.status }));
}
