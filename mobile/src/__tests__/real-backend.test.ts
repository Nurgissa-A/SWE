// Tests for the real-backend wiring (src/api/real.ts) against a fake fetch, so they run without servers.
import { beforeEach, describe, expect, jest, test } from '@jest/globals';

jest.mock('../api/session', () => ({
  loadSession: async () => ({ token: 'test-token', user: { id: 'u', email: 's@x.kz', role: 'attendee', displayName: 'S' } }),
  saveSession: async () => undefined,
  clearSession: async () => undefined,
  isStaff: () => false,
}));

import * as real from '../api/real';
import { ApiError } from '../api/errors';

type Call = { method: string; path: string; body?: any; headers: Record<string, string> };
let calls: Call[] = [];
let routes: Record<string, (body?: any) => { status?: number; json: unknown }> = {};

beforeEach(() => {
  calls = [];
  routes = {};
  (globalThis as any).fetch = jest.fn(async (url: string, init: any) => {
    const path = url.replace(/^https?:\/\/[^/]+/, '');
    const method = init?.method ?? 'GET';
    const body = init?.body ? JSON.parse(init.body) : undefined;
    calls.push({ method, path, body, headers: init.headers });
    const handler = routes[`${method} ${path}`];
    const r = handler ? handler(body) : { status: 404, json: { error: { code: 'NOT_FOUND', message: 'Not found' } } };
    const status = r.status ?? 200;
    return { ok: status < 400, status, text: async () => JSON.stringify(r.json) } as Response;
  });
});

const EVENT = {
  id: 'e1', title: 'Astana Jazz Night', startsAt: '2026-12-06T14:00:00Z', timezone: 'Asia/Almaty',
  venueName: 'Concert Hall', city: 'Astana', status: 'published', role: 'event_admin', canUndo: false,
  counts: { sold: 9, checkedIn: 2 },
};
const TICKET = { id: 't1', code: 'GMRX-G994', attendeeName: 'Arman', ticketType: 'VIP', status: 'valid', checkedInAt: null };

describe('events', () => {
  test('lists assigned events with counts, venue and undo permission', async () => {
    routes['GET /api/scanner/events'] = () => ({ json: [EVENT] });
    const [e] = await real.getMyEvents();
    expect(e).toMatchObject({ id: 'e1', title: 'Astana Jazz Night', venue: 'Concert Hall, Astana', registered: 9, checkedIn: 2, canUndo: false });
    expect(e.date).toContain('19:00'); // shown in the event's time zone (UTC+5)
    expect(calls[0].headers.Authorization).toBe('Bearer test-token');
    expect(calls[0].headers['X-Locale']).toBeDefined();
  });
});

describe('scanning', () => {
  test('a scanned QR is sent to the scan endpoint, which admits in one step', async () => {
    routes['POST /api/scanner/events/e1/scan'] = () => ({ json: { result: 'admitted', message: 'Valid ticket. Let the guest in.', ticket: { ...TICKET, status: 'checked_in' }, counts: { sold: 9, checkedIn: 3 } } });
    const r = await real.verifyTicket('e1', 'BFT1.abc.sig');
    expect(calls[0]).toMatchObject({ method: 'POST', body: { payload: 'BFT1.abc.sig' } });
    expect(r).toMatchObject({ kind: 'admitted', code: 'GMRX-G994', ticketId: 't1', attendeeName: 'Arman' });
  });

  test.each([
    ['already_used', 'already_used'], ['cancelled', 'cancelled'], ['refunded', 'refunded'],
    ['wrong_event', 'wrong_event'], ['campaign_qr', 'campaign'], ['invalid', 'not_found'],
  ])('server result %s shows as %s', async (server, kind) => {
    routes['POST /api/scanner/events/e1/scan'] = () => ({ json: { result: server, message: 'x', counts: { sold: 1, checkedIn: 0 } } });
    expect((await real.verifyTicket('e1', 'https://example.kz/c/promo')).kind).toBe(kind);
  });

  test('an unpublished event shows check-in as closed', async () => {
    routes['POST /api/scanner/events/e1/scan'] = () => ({ status: 409, json: { error: { code: 'NOT_PUBLISHED', message: 'This event is not published' } } });
    expect((await real.verifyTicket('e1', 'BFT1.abc.sig')).kind).toBe('closed');
  });

  test('a ticket code from search is only looked up, never checked in', async () => {
    routes['GET /api/scanner/events/e1/attendees?q=GMRX-G994'] = () => ({ json: [TICKET] });
    const r = await real.verifyTicket('e1', 'GMRX-G994');
    expect(r.kind).toBe('valid');
    expect(calls.every((c) => c.method === 'GET')).toBe(true);
  });
});

describe('check-in and undo', () => {
  test('check-in sends the ticket id', async () => {
    routes['POST /api/scanner/events/e1/check-in'] = () => ({ json: { result: 'admitted', message: 'ok', counts: { sold: 1, checkedIn: 1 } } });
    await real.checkIn('e1', { code: 'GMRX-G994', ticketId: 't1' });
    expect(calls[0].body).toEqual({ ticketId: 't1' });
  });

  test('a check-in the server refuses is reported', async () => {
    routes['POST /api/scanner/events/e1/check-in'] = () => ({ json: { result: 'already_used', message: 'x', counts: { sold: 1, checkedIn: 1 } } });
    await expect(real.checkIn('e1', { code: 'GMRX-G994', ticketId: 't1' })).rejects.toBeInstanceOf(ApiError);
  });

  test('undo finds the ticket id from its code when needed', async () => {
    routes['GET /api/scanner/events/e1/attendees?q=GMRX-G994'] = () => ({ json: [TICKET] });
    routes['POST /api/scanner/events/e1/undo'] = () => ({ json: { ticket: TICKET, counts: { sold: 1, checkedIn: 0 } } });
    await real.undoCheckIn('e1', { code: 'GMRX-G994' });
    expect(calls[1]).toMatchObject({ method: 'POST', body: { ticketId: 't1' } });
  });
});

describe('who may use the app', () => {
  const attendee = { id: 'u', email: 's@x.kz', role: 'attendee' as const, displayName: 'S' };
  test('an account added as door staff to an event may use it', async () => {
    routes['GET /api/scanner/events'] = () => ({ json: [EVENT] });
    expect(await real.canUseCheckIn(attendee)).toBe(true);
  });
  test('an attendee with no events may not', async () => {
    routes['GET /api/scanner/events'] = () => ({ json: [] });
    expect(await real.canUseCheckIn(attendee)).toBe(false);
  });
});
