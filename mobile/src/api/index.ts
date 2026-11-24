// Screens import everything from here (not from mock.ts or real.ts directly).
// The switch in config.ts decides which one is used.
import { USE_REAL_BACKEND } from './config';
import * as mock from './mock';
import * as real from './real';
import { loadSession } from './session';

export type { User, EventItem, VerifyResult, AttendeeItem } from './mock';

const source = USE_REAL_BACKEND ? real : mock;

export const login = source.login;
export const getMyEvents = source.getMyEvents;
export const getEvent = source.getEvent;

// Is the saved login still valid? (real: asks the server, fake: trusts the saved user)
export async function getMe(): Promise<mock.User> {
  if (USE_REAL_BACKEND) return real.getMe();
  const session = await loadSession();
  if (!session) throw new Error('No saved session');
  return session.user;
}

// ⚠️ Not on the backend yet: these always use fake data until the team adds
// ticket / check-in endpoints.
// (With the real backend on, real events have different IDs than the fake tickets,
// so we pretend every real event is fake event '1' — that way the demo tickets still scan.)
const fakeEventId = (eventId: string) => (USE_REAL_BACKEND ? '1' : eventId);

export const verifyTicket = (eventId: string, code: string) => mock.verifyTicket(fakeEventId(eventId), code);
export const searchAttendees = (eventId: string, query: string) => mock.searchAttendees(fakeEventId(eventId), query);
export const checkIn = mock.checkIn;
export const undoCheckIn = mock.undoCheckIn;
