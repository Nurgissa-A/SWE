import { redirect } from 'next/navigation';

/** The organizer preview is the real event page (owners can open their own drafts). */
export default async function PreviewEventPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  redirect(`/events/${id}`);
}
