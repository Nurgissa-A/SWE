'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useApi } from '@/hooks/useApi';
import { api } from '@/lib/api-client';
import { call, callEmpty, getApiErrorMessage } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import type { ApiEvent, Order, TicketType } from '@/lib/types';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { SimulationNotice } from '@/components/ui/Stat';
import { Spinner } from '@/components/ui/Spinner';
import { TextInput } from '@/components/ui/TextInput';

type Props = { event: ApiEvent; accessKey?: string; initialPromo?: string };

/** Choose tickets, reserve them (held for a few minutes), then pay with a test card (SRS §4.4–4.6). */
export function TicketPanel({ event, accessKey, initialPromo }: Props) {
  const { t, money } = useI18n();
  const { user } = useAuth();
  const router = useRouter();
  const eventId = event.id!;
  const { data: types, error, isLoading, mutate } = useApi<TicketType[]>(`types:${eventId}:${accessKey ?? ''}`,
    () => call(api.GET('/api/events/{eventId}/ticket-types', { params: { path: { eventId }, query: accessKey ? { key: accessKey } : {} } })));
  const [qty, setQty] = useState<Record<string, number>>({});
  const [promo, setPromo] = useState(initialPromo ?? '');
  const [discount, setDiscount] = useState<{ code: string; kzt: number } | null>(null);
  const [promoError, setPromoError] = useState('');
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [order, setOrder] = useState<Order | null>(null);
  const [card, setCard] = useState('4242 4242 4242 4242');
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState('');

  useEffect(() => {
    if (user) {
      setName((v) => v || user.displayName || '');
      setEmail((v) => v || user.email || '');
    }
  }, [user]);

  const items = useMemo(() => Object.entries(qty).filter(([, q]) => q > 0).map(([ticketTypeId, quantity]) => ({ ticketTypeId, quantity })), [qty]);
  const subtotal = (types ?? []).reduce((sum, type) => sum + (qty[type.id] ?? 0) * type.priceKzt, 0);
  const count = items.reduce((sum, it) => sum + it.quantity, 0);
  const closed = event.status !== 'published' || event.lifecycle === 'completed' || event.lifecycle === 'cancelled';

  function setQuantity(type: TicketType, next: number) {
    setDiscount(null);
    setQty((current) => ({ ...current, [type.id]: Math.max(0, Math.min(next, type.maxPerOrder, type.available)) }));
  }

  async function applyPromo() {
    setPromoError('');
    try {
      const res = await call(api.POST('/api/events/{eventId}/promo-preview', { params: { path: { eventId } }, body: { code: promo.trim(), items } }));
      setDiscount({ code: res.code ?? promo, kzt: res.discountKzt ?? 0 });
    } catch (e) {
      setDiscount(null);
      setPromoError(getApiErrorMessage(e));
    }
  }

  async function reserve() {
    setFormError('');
    if (!name.trim() || !email.trim()) { setFormError('Enter the buyer\'s name and a valid e-mail address'); return; }
    setBusy(true);
    try {
      const created = await call(api.POST('/api/checkout/reserve', { body: {
        eventId, items, buyer: { name: name.trim(), email: email.trim() }, promoCode: discount?.code ?? (promo.trim() || null), ...(accessKey ? { accessKey } : {}),
      } }));
      if (created.status === 'completed') router.push(`/orders/${created.id}?new=1`);
      else setOrder(created);
      void mutate();
    } catch (e) {
      setFormError(getApiErrorMessage(e));
      void mutate();
    } finally {
      setBusy(false);
    }
  }

  async function pay() {
    if (!order) return;
    setFormError('');
    setBusy(true);
    try {
      const paid = await call(api.POST('/api/checkout/{orderId}/pay', { params: { path: { orderId: order.id } }, body: { cardNumber: card } }));
      router.push(`/orders/${paid.id}?new=1`);
    } catch (e) {
      setFormError(getApiErrorMessage(e));
      // A failed payment or an expired hold ends this order; start again.
      setOrder(null);
      void mutate();
    } finally {
      setBusy(false);
    }
  }

  async function abandon() {
    if (order) await callEmpty(api.POST('/api/checkout/{orderId}/abandon', { params: { path: { orderId: order.id } } })).catch(() => undefined);
    setOrder(null);
    void mutate();
  }

  const box = 'rounded-card border border-border bg-surface p-5 shadow-sm';
  if (closed) return <div className={box}><p className="font-semibold text-text">{t(event.status === 'cancelled' ? 'This event was cancelled.' : 'Registration is closed.')}</p></div>;
  if (isLoading) return <div className={box}><Spinner label="Loading tickets" /></div>;
  if (error) return <div className={box}><ErrorMessage message={getApiErrorMessage(error)} onRetry={() => void mutate()} /></div>;

  if (order) {
    return (
      <div className={`${box} grid gap-4`}>
        <h2 className="font-sans text-lg font-bold tracking-normal text-text">{t('Payment')}</h2>
        <HoldTimer expiresAt={order.expiresAt} onExpired={() => { setOrder(null); setFormError('Your reservation expired. Please select tickets again.'); void mutate(); }} />
        <dl className="grid gap-1 text-sm">
          {order.items.map((it) => <div key={it.ticketTypeId} className="flex justify-between"><dt>{it.quantity} × {it.name}</dt><dd>{money(it.quantity * it.unitPriceKzt)}</dd></div>)}
          {order.discountKzt ? <div className="flex justify-between text-success"><dt>{t('Discount')}</dt><dd>−{money(order.discountKzt)}</dd></div> : null}
          <div className="mt-1 flex justify-between border-t border-border pt-2 text-base font-bold"><dt>{t('Total')}</dt><dd>{money(order.totalKzt)}</dd></div>
        </dl>
        <SimulationNotice compact />
        {formError ? <ErrorMessage message={formError} /> : null}
        <Form onSubmit={pay} className="grid gap-3">
          <TextInput label="Card number" inputMode="numeric" autoComplete="off" value={card} onChange={(e) => setCard(e.currentTarget.value)} helpText="Test cards only. Card numbers are never stored." />
          <Button type="submit" size="lg" loading={busy}>{t('Pay {amount}', { amount: money(order.totalKzt) })}</Button>
          <Button variant="quiet" onClick={() => void abandon()}>{t('Cancel and release tickets')}</Button>
        </Form>
      </div>
    );
  }

  return (
    <div className={`${box} grid gap-4`}>
      <h2 className="font-sans text-lg font-bold tracking-normal text-text">{t('Tickets')}</h2>
      {(types ?? []).length === 0 ? <p className="text-sm text-text-muted">{t('No tickets are on sale yet.')}</p> : null}
      <ul className="grid gap-3">
        {(types ?? []).map((type) => {
          const q = qty[type.id] ?? 0;
          const notStarted = type.salesStartAt && new Date(type.salesStartAt) > new Date();
          const ended = type.salesEndAt && new Date(type.salesEndAt) < new Date();
          const soldOut = type.available <= 0;
          const unavailable = notStarted || ended || soldOut || (type.priceKzt > 0 && event.paidSalesStatus !== 'active');
          return (
            <li key={type.id} className="rounded-input border border-border p-3">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="font-semibold text-text">{type.name}</p>
                  <p className="text-sm text-text-muted">{type.priceKzt ? money(type.priceKzt) : t('Free')}{type.description ? ` · ${type.description}` : ''}</p>
                  <p className="mt-1 text-xs text-text-muted">
                    {soldOut ? t('Sold out') : notStarted ? t('Sales have not started') : ended ? t('Sales have ended')
                      : type.priceKzt > 0 && event.paidSalesStatus !== 'active' ? t('Paid tickets are not on sale yet')
                      : type.available <= 10 ? t('Only {n} left', { n: type.available }) : t('Available')}
                  </p>
                </div>
                <div className="flex items-center gap-2" aria-label={t('Quantity of {name}', { name: type.name })}>
                  <button type="button" disabled={unavailable || q === 0} onClick={() => setQuantity(type, q - 1)} aria-label={t('Fewer {name}', { name: type.name })}
                    className="grid size-9 place-items-center rounded-full border border-border text-lg disabled:opacity-40">−</button>
                  <span className="w-6 text-center font-display font-bold" aria-live="polite">{q}</span>
                  <button type="button" disabled={unavailable || q >= Math.min(type.maxPerOrder, type.available)} onClick={() => setQuantity(type, q + 1)} aria-label={t('More {name}', { name: type.name })}
                    className="grid size-9 place-items-center rounded-full border border-border text-lg disabled:opacity-40">+</button>
                </div>
              </div>
            </li>
          );
        })}
      </ul>
      {count > 0 ? (
        <>
          <div className="flex items-end gap-2">
            <div className="flex-1"><TextInput label="Promo code" value={promo} onChange={(e) => { setPromo(e.currentTarget.value.toUpperCase()); setDiscount(null); }} error={promoError} /></div>
            <Button variant="secondary" disabled={!promo.trim()} onClick={() => void applyPromo()} className="mb-0.5">{t('Apply')}</Button>
          </div>
          {discount ? <p className="text-sm font-semibold text-success">{t('Code {code} applied: −{amount}', { code: discount.code, amount: money(discount.kzt) })}</p> : null}
          <div className="flex justify-between border-t border-border pt-3 text-base font-bold">
            <span>{t('Total')}</span><span>{money(Math.max(0, subtotal - (discount?.kzt ?? 0)))}</span>
          </div>
          {user ? (
            <Form onSubmit={reserve} className="grid gap-3">
              <TextInput label="Buyer name" required value={name} onChange={(e) => setName(e.currentTarget.value)} autoComplete="name" />
              <TextInput label="E-mail for tickets" type="email" required value={email} onChange={(e) => setEmail(e.currentTarget.value)} autoComplete="email" />
              {formError ? <ErrorMessage message={formError} /> : null}
              <Button type="submit" size="lg" loading={busy}>{subtotal - (discount?.kzt ?? 0) > 0 ? t('Continue to payment') : t('Register')}</Button>
              <p className="text-xs text-text-muted">{t('Tickets are held for you for a few minutes while you pay.')}</p>
            </Form>
          ) : (
            <Link href={`/login?next=${encodeURIComponent(`/events/${eventId}${accessKey ? `?key=${accessKey}` : ''}`)}`}
              className="inline-flex min-h-12 items-center justify-center rounded-full bg-primary px-6 font-semibold text-white hover:bg-primary-hover">
              {t('Sign in to get tickets')}
            </Link>
          )}
        </>
      ) : formError ? <ErrorMessage message={formError} /> : null}
    </div>
  );
}

function HoldTimer({ expiresAt, onExpired }: { expiresAt?: string | null; onExpired: () => void }) {
  const { t } = useI18n();
  const [left, setLeft] = useState(() => (expiresAt ? new Date(expiresAt).getTime() - Date.now() : 0));
  const expired = useRef(onExpired);
  useEffect(() => { expired.current = onExpired; }, [onExpired]);
  useEffect(() => {
    if (!expiresAt) return;
    const timer = setInterval(() => {
      const ms = new Date(expiresAt).getTime() - Date.now();
      setLeft(ms);
      if (ms <= 0) { clearInterval(timer); expired.current(); }
    }, 1000);
    return () => clearInterval(timer);
  }, [expiresAt]);
  if (!expiresAt) return null;
  const s = Math.max(0, Math.floor(left / 1000));
  return (
    <p role="timer" aria-live="off" className="rounded-input bg-secondary px-3 py-2 text-sm text-text">
      {t('Tickets held for {time}', { time: `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}` })}
    </p>
  );
}
