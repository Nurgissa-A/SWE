'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useAuth } from '@/hooks/useAuth';
import { authApi } from '@/lib/api-client';
import { call, getApiErrorMessage } from '@/lib/api-errors';
import { LOCALES, useI18n, type Locale } from '@/lib/i18n';
import { RequireSignIn } from '@/components/layout/RequireSignIn';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Form } from '@/components/ui/Form';
import { Select } from '@/components/ui/Select';
import { TextInput } from '@/components/ui/TextInput';

export default function AccountPage() {
  return <RequireSignIn><Account /></RequireSignIn>;
}

function Account() {
  const { t, locale, setLocale, date } = useI18n();
  const { user, refreshUser } = useAuth();
  const [name, setName] = useState('');
  const [lang, setLang] = useState<Locale>(locale);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    setName(user?.displayName ?? '');
    if (user?.locale) setLang(user.locale);
  }, [user]);
  if (!user) return null;
  const role = { attendee: 'Attendee', organizer: 'Organizer', event_admin: 'Event Admin', platform_admin: 'Platform admin' }[user.role];
  return (
    <main className="mx-auto grid max-w-2xl gap-6 px-4 py-8 sm:px-6 lg:py-12">
      <h1 className="text-3xl font-bold text-text">{t('Account')}</h1>
      <dl className="grid gap-2 rounded-card border border-border bg-surface p-5 text-sm">
        <div className="flex justify-between gap-4"><dt className="text-text-muted">{t('E-mail')}</dt><dd className="font-semibold">{user.email}</dd></div>
        <div className="flex justify-between gap-4"><dt className="text-text-muted">{t('Role')}</dt><dd className="font-semibold">{t(role)}</dd></div>
        <div className="flex justify-between gap-4"><dt className="text-text-muted">{t('Member since')}</dt><dd className="font-semibold">{date(user.createdAt)}</dd></div>
      </dl>
      <Form onSubmit={async () => {
        setSaved(false); setError('');
        try {
          await call(authApi.PATCH('/api/users/me', { body: { displayName: name.trim(), locale: lang } }));
          setLocale(lang);
          await refreshUser();
          setSaved(true);
        } catch (e) { setError(getApiErrorMessage(e)); }
      }} className="grid gap-4 rounded-card border border-border bg-surface p-5">
        <TextInput label="Display name" maxLength={100} value={name} onChange={(e) => setName(e.currentTarget.value)} helpText="Shown to organizers and on support messages." />
        <Select label="Language for the site and e-mails" value={lang} onChange={(e) => setLang(e.currentTarget.value as Locale)}>
          {LOCALES.map((l) => <option key={l.code} value={l.code}>{l.label}</option>)}
        </Select>
        {error ? <ErrorMessage message={error} /> : null}
        <div className="flex items-center gap-3"><Button type="submit">{t('Save')}</Button>{saved ? <span role="status" className="text-sm font-semibold text-success">{t('Saved')}</span> : null}</div>
      </Form>
      {user.role === 'attendee' ? <Link href="/dashboard/events" className="font-semibold text-primary">{t('Want to host events? Become an organizer')} →</Link> : null}
      <p className="text-sm text-text-muted">{t('To change your password, sign out and use “Forgot password?” on the sign-in page.')}</p>
    </main>
  );
}
