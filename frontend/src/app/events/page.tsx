import { EventDiscovery } from '@/components/events/EventDiscovery';

type EventsPageProps = {
  searchParams: Promise<{ q?: string; category?: string }>;
};

export default async function EventsPage({ searchParams }: EventsPageProps) {
  const params = await searchParams;
  return <EventDiscovery initialQuery={params.q ?? ''} initialCategory={params.category ?? 'all'} />;
}
