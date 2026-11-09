'use client';

import { useState, type FormEvent, type KeyboardEvent } from 'react';
import { useRouter } from 'next/navigation';
import { Button } from '@/components/ui/Button';

export function EventSearch() {
  const router = useRouter();
  const [query, setQuery] = useState('');

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const search = query.trim();
    router.push(search ? `/events?q=${encodeURIComponent(search)}` : '/events');
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Enter' && (event.nativeEvent.isComposing || event.keyCode === 229)) event.preventDefault();
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3 rounded-card border border-border bg-surface p-3 shadow-lg shadow-text/5 sm:flex-row sm:items-center">
      <label htmlFor="event-search" className="sr-only">Search events by name or category</label>
      <div className="flex min-w-0 flex-1 items-center gap-3 px-3">
        <svg className="size-5 shrink-0 text-text-muted" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="10.8" cy="10.8" r="6.8" stroke="currentColor" strokeWidth="1.8" />
          <path d="m16 16 4.2 4.2" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </svg>
        <input
          id="event-search"
          type="search"
          value={query}
          onChange={(event) => setQuery(event.currentTarget.value)}
          onKeyDown={handleKeyDown}
          placeholder="Search events"
          className="min-h-12 w-full min-w-0 bg-transparent text-sm text-text outline-none placeholder:text-text-muted"
        />
      </div>
      <Button type="submit" size="lg" className="shrink-0">Explore events</Button>
    </form>
  );
}
