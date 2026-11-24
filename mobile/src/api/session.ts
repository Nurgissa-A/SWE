import { storage } from './storage';
import type { User } from './mock';

// Keeps the login token in the phone's secure storage (Keychain on iPhone),
// so the admin stays signed in after closing the app.

export type Session = { token: string; user: User };

const KEY = 'biletflow_session';

// Who is allowed to use the check-in app (SRS 4.8: event staff only)
const STAFF_ROLES: User['role'][] = ['event_admin', 'organizer'];

export function isStaff(user: User): boolean {
  return STAFF_ROLES.includes(user.role);
}

export async function saveSession(session: Session): Promise<void> {
  await storage.set(KEY, JSON.stringify(session));
}

export async function loadSession(): Promise<Session | null> {
  try {
    const raw = await storage.get(KEY);
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}

export async function clearSession(): Promise<void> {
  await storage.remove(KEY);
}
