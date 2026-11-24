import { redirect } from 'next/navigation';

/** Editing now lives in the event hub's Details tab. */
export default async function EditEventPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  redirect(`/dashboard/events/${id}?tab=details`);
}
