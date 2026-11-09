'use client';

import { useMemo, useState } from 'react';
import { useEvents } from '@/hooks/useEvents';
import { EventList } from './EventList';
import { TextInput } from '@/components/ui/TextInput';

type EventDiscoveryProps = { initialQuery: string; initialCategory: string };

export function EventDiscovery({ initialQuery, initialCategory }: EventDiscoveryProps) {
  const [query, setQuery] = useState(initialQuery);
  const [selectedCategory, setSelectedCategory] = useState(initialCategory);
  const { data, error, isLoading, mutate } = useEvents();
  const publishedEvents = useMemo(
    () => (data ?? []).filter((event) => event.status === 'published' && event.visibility === 'public'),
    [data],
  );

  const categories = useMemo(() => {
    const values = publishedEvents.map((event) => event.category?.trim()).filter((value): value is string => Boolean(value));
    return Array.from(new Set(values)).sort((left, right) => left.localeCompare(right));
  }, [publishedEvents]);

  const visibleEvents = useMemo(() => {
    const search = query.trim().toLocaleLowerCase();
    return publishedEvents.filter((event) => {
      const matchesCategory = selectedCategory === 'all' || event.category?.toLocaleLowerCase() === selectedCategory.toLocaleLowerCase();
      const matchesQuery = !search || `${event.title} ${event.description ?? ''} ${event.category ?? ''}`.toLocaleLowerCase().includes(search);
      return matchesCategory && matchesQuery;
    });
  }, [publishedEvents, query, selectedCategory]);

  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8 lg:py-14">
      <div className="flex flex-col justify-between gap-6 border-b border-border pb-8 md:flex-row md:items-end">
        <div className="max-w-2xl">
          <p className="text-xs font-bold uppercase tracking-[0.18em] text-primary">Explore BiletFlow</p>
          <h1 className="mt-3 text-3xl font-bold tracking-tight text-text sm:text-4xl">Find an event that feels like you.</h1>
          <p className="mt-3 leading-7 text-text-muted">Browse published events and discover what&apos;s happening around you.</p>
        </div>
        <div className="w-full md:max-w-sm">
          <TextInput label="Search events" type="search" value={query} onChange={(event) => setQuery(event.currentTarget.value)} placeholder="Name or category" />
        </div>
      </div>

      <div className="mt-7 flex flex-wrap gap-2" aria-label="Filter events by category">
        <button type="button" aria-pressed={selectedCategory === 'all'} onClick={() => setSelectedCategory('all')} className={`min-h-10 rounded-full px-4 text-sm font-semibold transition-colors ${selectedCategory === 'all' ? 'bg-primary text-white' : 'border border-border bg-surface text-text-muted hover:bg-secondary'}`}>All events</button>
        {categories.map((category) => (
          <button key={category} type="button" aria-pressed={selectedCategory.toLocaleLowerCase() === category.toLocaleLowerCase()} onClick={() => setSelectedCategory(category)} className={`min-h-10 rounded-full px-4 text-sm font-semibold transition-colors ${selectedCategory.toLocaleLowerCase() === category.toLocaleLowerCase() ? 'bg-primary text-white' : 'border border-border bg-surface text-text-muted hover:bg-secondary'}`}>
            {category}
          </button>
        ))}
      </div>

      <div className="mt-8 flex items-center justify-between gap-4">
        <h2 className="text-lg font-bold text-text">{error ? 'Events unavailable' : isLoading ? 'Loading events' : `${visibleEvents.length} ${visibleEvents.length === 1 ? 'event' : 'events'}`}</h2>
        <p className="hidden text-sm text-text-muted sm:block">Dates shown in Almaty time</p>
      </div>
      <div className="mt-5">
        <EventList events={visibleEvents} isLoading={isLoading} error={error} onRetry={() => void mutate()} emptyTitle={query || selectedCategory !== 'all' ? 'No events match these filters' : 'No events to show yet'} emptyDescription={query || selectedCategory !== 'all' ? 'Try a different search or category.' : 'When organizers publish an event, it will appear here.'} />
      </div>
    </main>
  );
}
