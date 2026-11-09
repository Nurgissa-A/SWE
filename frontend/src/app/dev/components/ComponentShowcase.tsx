import { notFound } from 'next/navigation';
import ComponentShowcase from './ComponentShowcase';

export default function DevComponentsPage() {
  if (process.env.NODE_ENV === 'production') notFound();
  return <ComponentShowcase />;
}
