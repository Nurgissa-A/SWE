// Automated tests for ticket verification and check-in rules (SRS 4.8, 4.14).
// Run with: npm test
import { describe, test, expect } from '@jest/globals';
import { verifyTicket, checkIn, undoCheckIn, searchAttendees, getEvent, login } from '../api/mock';
import { isStaff } from '../api/session';

describe('ticket verification', () => {
  test('a valid ticket for this event is accepted', async () => {
    const r = await verifyTicket('1', 'BF-T-1001');
    expect(r.kind).toBe('valid');
    expect(r.attendeeName).toBe('Dana Sultanova');
  });

  test('an already used ticket is reported with its check-in time', async () => {
    const r = await verifyTicket('1', 'BF-T-1002');
    expect(r.kind).toBe('already_used');
    expect(r.checkedInAt).toBe('18:05');
  });

  test('cancelled and refunded tickets are rejected', async () => {
    expect((await verifyTicket('1', 'BF-T-1003')).kind).toBe('cancelled');
    expect((await verifyTicket('1', 'BF-T-1004')).kind).toBe('refunded');
  });

  test('a ticket for another event is rejected', async () => {
    expect((await verifyTicket('1', 'BF-T-2001')).kind).toBe('wrong_event');
  });

  test('campaign / promo QR codes are never accepted as entry (SRS 4.14)', async () => {
    expect((await verifyTicket('1', 'BF-CAMPAIGN-SPRING10')).kind).toBe('campaign');
    expect((await verifyTicket('1', 'https://biletflow.kz/e/1?promo=abc')).kind).toBe('campaign');
  });

  test('an unknown code is invalid', async () => {
    expect((await verifyTicket('1', 'HELLO-123')).kind).toBe('not_found');
  });

  test('surrounding spaces in a scanned code are ignored', async () => {
    expect((await verifyTicket('2', '  BF-T-2001  ')).kind).toBe('valid');
  });
});

describe('check-in and undo', () => {
  test('a ticket can be checked in once, then shows as already used', async () => {
    const before = (await getEvent('2'))!.checkedIn!;
    await checkIn('BF-T-2001');
    expect((await verifyTicket('2', 'BF-T-2001')).kind).toBe('already_used');
    expect((await getEvent('2'))!.checkedIn).toBe(before + 1);
  });

  test('a second check-in of the same ticket is refused', async () => {
    await expect(checkIn('BF-T-2001')).rejects.toThrow();
  });

  test('undo makes the ticket valid again and lowers the count', async () => {
    const before = (await getEvent('2'))!.checkedIn!;
    await undoCheckIn('BF-T-2001');
    expect((await verifyTicket('2', 'BF-T-2001')).kind).toBe('valid');
    expect((await getEvent('2'))!.checkedIn).toBe(before - 1);
  });

  test('a refunded ticket cannot be checked in', async () => {
    await expect(checkIn('BF-T-1004')).rejects.toThrow();
  });
});

describe('attendee search', () => {
  test('finds attendees by part of the name, only for this event', async () => {
    const r = await searchAttendees('1', 'dana');
    expect(r.map((a) => a.code)).toEqual(['BF-T-1001']);
  });

  test('finds attendees by ticket code', async () => {
    const r = await searchAttendees('1', 'bf-t-1004');
    expect(r).toHaveLength(1);
  });

  test('an empty search lists everyone at the event', async () => {
    expect(await searchAttendees('1', '')).toHaveLength(4);
  });
});

describe('sign-in and roles', () => {
  test('a wrong password is rejected with a clear message', async () => {
    await expect(login('admin@biletflow.kz', 'wrong')).rejects.toThrow('Email or password is incorrect.');
  });

  test('event admins and organizers may use the app, attendees may not', async () => {
    const admin = await login('admin@biletflow.kz', 'admin123');
    const attendee = await login('attendee@biletflow.kz', 'att123');
    expect(isStaff(admin.user)).toBe(true);
    expect(isStaff(attendee.user)).toBe(false);
  });
});
