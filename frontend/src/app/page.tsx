import Image from 'next/image';
import Link from 'next/link';
import { EventSearch } from '@/components/events/EventSearch';

const experienceTypes = [
  { label: 'Music & nightlife', category: 'Music', number: '01' },
  { label: 'Arts & culture', category: 'Arts', number: '02' },
  { label: 'Workshops & talks', category: 'Community', number: '03' },
];

export default function HomePage() {
  return (
    <main>
      <section className="mx-auto grid max-w-7xl gap-8 px-4 pb-16 pt-10 sm:px-6 md:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)] md:items-center md:gap-10 md:pb-24 md:pt-16 lg:px-8">
        <div className="max-w-xl">
          <p className="inline-flex items-center gap-2 rounded-full border border-border bg-surface px-3 py-1.5 text-xs font-semibold text-text-muted">
            <span className="size-2 rounded-full bg-success" aria-hidden="true" /> Made for moments that stay with you
          </p>
          <h1 className="mt-6 text-4xl font-bold leading-[1.08] tracking-tight text-text sm:text-5xl lg:text-6xl">Make plans.<br /><span className="text-primary">Find your people.</span></h1>
          <p className="mt-5 max-w-lg text-base leading-7 text-text-muted sm:text-lg">A better night out starts with the right idea. Find events worth leaving home for, or bring your own to life.</p>
          <div className="mt-8"><EventSearch /></div>
          <div className="mt-5 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs font-medium text-text-muted">
            <span>Discover local experiences</span><span aria-hidden="true" className="size-1 rounded-full bg-border" />
            <span>Plan with confidence</span><span aria-hidden="true" className="size-1 rounded-full bg-border" />
            <Link href="/dashboard/events" className="font-semibold text-primary hover:text-primary-hover">Host your own</Link>
          </div>
        </div>
        <div className="relative min-h-[24rem] overflow-hidden rounded-card bg-secondary sm:min-h-[30rem]">
          <Image
            src="/images/biletflow-hero.webp"
            alt="People watching a live band at an outdoor concert at dusk"
            fill
            priority
            sizes="(min-width: 768px) 52vw, 100vw"
            className="object-cover"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-text/75 via-transparent to-text/10" />
          <div className="absolute inset-x-5 bottom-5 flex items-end justify-between gap-4 sm:inset-x-7 sm:bottom-7">
            <div className="text-white">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-white/80">Live music under open skies</p>
              <p className="mt-2 text-2xl font-bold">Plan a night out</p>
            </div>
            <span className="grid size-12 shrink-0 place-items-center rounded-full bg-surface text-xl text-primary shadow-lg" aria-hidden="true">↗</span>
          </div>
        </div>
      </section>

      <section className="border-y border-border bg-surface">
        <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8 lg:py-16">
          <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.18em] text-primary">Choose your kind of day</p>
              <h2 className="mt-3 text-2xl font-bold tracking-tight text-text sm:text-3xl">There&apos;s a place for every plan.</h2>
            </div>
            <Link href="/events" className="font-semibold text-primary hover:text-primary-hover">See all events <span aria-hidden="true">→</span></Link>
          </div>
          <div className="mt-8 grid gap-4 md:grid-cols-3">
            {experienceTypes.map((item) => (
              <Link key={item.number} href={`/events?category=${encodeURIComponent(item.category)}`} className="group flex min-h-36 items-end justify-between rounded-card border border-border bg-background p-5 transition-all hover:-translate-y-1 hover:border-primary/50 hover:bg-secondary sm:min-h-44 sm:p-6">
                <div>
                  <p className="text-xs font-bold tracking-[0.12em] text-text-muted">{item.number}</p>
                  <h3 className="mt-4 text-xl font-bold text-text">{item.label}</h3>
                </div>
                <span className="grid size-10 place-items-center rounded-full bg-surface text-primary transition-transform group-hover:translate-x-1" aria-hidden="true">→</span>
              </Link>
            ))}
          </div>
        </div>
      </section>

      <section className="mx-auto flex max-w-7xl flex-col gap-6 px-4 py-14 sm:px-6 md:flex-row md:items-center md:justify-between lg:px-8 lg:py-20">
        <div className="max-w-2xl">
          <p className="text-xs font-bold uppercase tracking-[0.18em] text-primary">For the people who make it happen</p>
          <h2 className="mt-3 text-2xl font-bold tracking-tight text-text sm:text-3xl">Your idea deserves a room full of people.</h2>
          <p className="mt-3 max-w-xl leading-7 text-text-muted">Create an event, share the details, and make it easy for your community to find you.</p>
        </div>
        <Link href="/dashboard/events" className="inline-flex min-h-12 shrink-0 items-center justify-center rounded-full bg-primary px-6 font-semibold text-white transition-colors hover:bg-primary-hover">Start hosting <span className="ml-2" aria-hidden="true">→</span></Link>
      </section>
    </main>
  );
}
