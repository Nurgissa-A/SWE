import Image from 'next/image';
import Link from 'next/link';
import type { ReactNode } from 'react';

type AuthShellProps = { children: ReactNode; eyebrow: string; title: string; description: string };

export function AuthShell({ children, eyebrow, title, description }: AuthShellProps) {
  return (
    <main className="mx-auto grid min-h-[calc(100vh-4.5rem)] max-w-7xl gap-8 px-4 py-8 sm:px-6 md:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)] md:items-center lg:px-8 lg:py-14">
      <section className="mx-auto w-full max-w-md">
        <Link href="/events" className="text-sm font-semibold text-primary hover:text-primary-hover">← Back to events</Link>
        <p className="mt-8 text-xs font-bold uppercase tracking-[0.18em] text-primary">{eyebrow}</p>
        <h1 className="mt-3 text-3xl font-bold tracking-tight text-text sm:text-4xl">{title}</h1>
        <p className="mt-3 leading-7 text-text-muted">{description}</p>
        <div className="mt-8 rounded-card border border-border bg-surface p-5 shadow-sm sm:p-7">{children}</div>
      </section>
      <aside className="relative hidden min-h-[34rem] overflow-hidden rounded-card bg-primary md:block" aria-label="BiletFlow event community">
        <Image src="/images/biletflow-hero.webp" alt="People watching a live band at an outdoor concert at dusk" fill priority sizes="(min-width: 768px) 50vw, 100vw" className="object-cover" />
        <div className="absolute inset-0 bg-gradient-to-t from-text/85 via-text/10 to-transparent" />
        <div className="absolute inset-x-8 bottom-8 max-w-md text-white lg:inset-x-10 lg:bottom-10">
          <p className="text-sm font-semibold uppercase tracking-[0.16em] text-white/80">Find your people</p>
          <p className="mt-3 text-3xl font-bold leading-tight lg:text-4xl">Make room for something unforgettable.</p>
        </div>
      </aside>
    </main>
  );
}
