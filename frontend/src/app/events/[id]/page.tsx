import { EventPage } from '@/components/events/EventPage';

type Props = { params: Promise<{ id: string }>; searchParams: Promise<{ key?: string; promo?: string }> };

export default async function EventDetailsPage({ params, searchParams }: Props) {
  const { id } = await params;
  const query = await searchParams;
  return <EventPage id={id} accessKey={query.key} promo={query.promo} />;
}
