// REAL backend calls (team repo: docs/openapi.yaml).
// Only things the backend actually has today are here: login, users/me, events.
import { request } from './http';
import { ApiError } from './errors';
import { IDENTITY_URL, EVENTS_URL } from './config';
import type { User, EventItem } from './mock';

type BackendEvent = {
  id: string;
  title: string;
  category?: string | null;
  startsAt: string;
  status: 'draft' | 'published' | 'cancelled';
  capacity?: number | null;
};

function toEventItem(e: BackendEvent): EventItem {
  const date = new Date(e.startsAt).toLocaleString([], {
    day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit',
  });
  const statusLabel = e.status === 'published' ? 'Published' : e.status === 'draft' ? 'Draft' : 'Cancelled';
  return {
    id: e.id,
    title: e.title,
    date,
    // The backend has no venue names yet, so show the status (and category if any)
    venue: e.category ? `${statusLabel} · ${e.category}` : statusLabel,
    // Ticket counts don't exist on the backend yet
    registered: null,
    checkedIn: null,
  };
}

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

// GET /api/events -> organizers see their own events
export async function getMyEvents(): Promise<EventItem[]> {
  const list = await request<BackendEvent[]>(EVENTS_URL, '/api/events', { auth: true });
  return list.map(toEventItem);
}

// GET /api/events/{id}
export async function getEvent(id: string): Promise<EventItem | null> {
  try {
    const e = await request<BackendEvent>(EVENTS_URL, `/api/events/${encodeURIComponent(id)}`, { auth: true });
    return toEventItem(e);
  } catch (err) {
    if (err instanceof ApiError && (err.code === 'http_404' || err.code === 'not_found')) return null;
    throw err;
  }
}
