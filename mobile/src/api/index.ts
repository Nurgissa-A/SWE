// Screens import everything from here (not from mock.ts or real.ts directly).
// The switch in config.ts decides which one is used.
import { USE_REAL_BACKEND } from './config';
import * as mock from './mock';
import * as real from './real';
import { loadSession, isStaff } from './session';

export type { User, EventItem, VerifyResult, AttendeeItem } from './mock';

type TicketRef = { code: string; ticketId?: string };

export const login = USE_REAL_BACKEND ? real.login : mock.login;
export const getMyEvents = USE_REAL_BACKEND ? real.getMyEvents : mock.getMyEvents;
export const getEvent = USE_REAL_BACKEND ? real.getEvent : mock.getEvent;
export const verifyTicket = USE_REAL_BACKEND ? real.verifyTicket : mock.verifyTicket;
export const searchAttendees = USE_REAL_BACKEND ? real.searchAttendees : mock.searchAttendees;

export const checkIn = (eventId: string, ticket: TicketRef) =>
  USE_REAL_BACKEND ? real.checkIn(eventId, ticket) : mock.checkIn(ticket.code);
export const undoCheckIn = (eventId: string, ticket: TicketRef) =>
  USE_REAL_BACKEND ? real.undoCheckIn(eventId, ticket) : mock.undoCheckIn(ticket.code);

// May this account use the check-in app?
export const canUseCheckIn = (user: mock.User) =>
  USE_REAL_BACKEND ? real.canUseCheckIn(user) : Promise.resolve(isStaff(user));

// Is the saved login still valid? (real: asks the server, fake: trusts the saved user)
export async function getMe(): Promise<mock.User> {
  if (USE_REAL_BACKEND) return real.getMe();
  const session = await loadSession();
  if (!session) throw new Error('No saved session');
  return session.user;
}
