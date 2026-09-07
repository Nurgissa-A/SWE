// src/components/EventCard.tsx
import { Button } from './Button';

type Event = {
  id: string;
  title: string;
  startsAt: string;
  status: 'draft' | 'published' | 'cancelled';
};

export function EventCard({ event, onSelect }: { event: Event; onSelect: (id: string) => void }) {
  const statusColor =
    event.status === 'published'
      ? 'bg-green-100 text-green-800'
      : event.status === 'cancelled'
      ? 'bg-red-100 text-red-800'
      : 'bg-gray-100 text-gray-700';

  return (
    <div className="p-4 rounded-card border border-gray-200 shadow-sm flex flex-col gap-2">
      <div className="flex justify-between items-start">
        <h2 className="text-lg font-bold">{event.title}</h2>
        <span className={`text-xs px-2 py-1 rounded-full ${statusColor}`}>{event.status}</span>
      </div>
      <p className="text-gray-500 text-sm">
        {new Date(event.startsAt).toLocaleDateString()}
      </p>
      <Button label="View Details" onClick={() => onSelect(event.id)} variant="secondary" />
    </div>
  );
}