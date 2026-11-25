import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import QR from 'qrcode';
import { apiGet, apiPost, apiToken, WEB_URL } from './helpers';

type Ticket = { id: string; code: string; attendeeName: string; status: string; qrPayload?: string };
type Order = { id: string; tickets?: Ticket[]; event?: { title: string } };

/**
 * Checks the stack is up, then prepares the door-app fake camera: a real, valid Jazz Night ticket
 * of the demo attendee Arman, rendered as a QR video for Chromium's fake camera.
 */
export default async function globalSetup() {
  const ping = await fetch(`${WEB_URL}/api/events`).catch(() => null);
  if (!ping?.ok) throw new Error(`BiletFlow is not reachable at ${WEB_URL}. Start the stack and load the demo data first (see e2e/README.md).`);

  mkdirSync('.cache', { recursive: true });
  const staff = await apiToken('staff@biletflow.kz');
  const jazz = (await apiGet<{ id: string; title: string }[]>(staff, '/api/scanner/events')).find((e) => e.title.startsWith('Astana Jazz'));
  if (!jazz) throw new Error('Demo event "Astana Jazz Night" not found for staff@biletflow.kz.');

  // Start from a clean state: Arman's Jazz tickets not yet checked in.
  for (const tk of await apiGet<Ticket[]>(staff, `/api/scanner/events/${jazz.id}/attendees?q=Arman`)) {
    if (tk.status === 'checked_in') await apiPost(staff, `/api/scanner/events/${jazz.id}/undo`, { ticketId: tk.id });
  }

  const arman = await apiToken('arman@example.kz');
  let ticket: Ticket | undefined;
  for (const o of await apiGet<Order[]>(arman, '/api/orders')) {
    const full = await apiGet<Order>(arman, `/api/orders/${o.id}`);
    if (full.event && !full.event.title.startsWith('Astana Jazz')) continue;
    ticket = full.tickets?.find((t) => t.status === 'valid' && t.qrPayload);
    if (ticket) break;
  }
  if (!ticket?.qrPayload) throw new Error('No valid Jazz Night ticket for arman@example.kz in the demo data.');

  await QR.toFile('.cache/ticket.png', ticket.qrPayload, { width: 520, margin: 4 });
  writeFileSync('.cache/ticket.json', JSON.stringify({ eventId: jazz.id, ticketId: ticket.id, name: ticket.attendeeName, code: ticket.code }));
  try {
    execFileSync('ffmpeg', ['-loglevel', 'error', '-y', '-loop', '1', '-i', '.cache/ticket.png', '-vf', 'scale=440:440,pad=640:480:100:20:white,format=yuv420p', '-t', '3', '-r', '15', '.cache/ticket.y4m']);
  } catch {
    console.warn('ffmpeg not found: camera scanning tests will be skipped.');
  }
}
