'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { api } from '@/lib/api-client';
import { call } from '@/lib/api-errors';
import { useI18n } from '@/lib/i18n';
import { Spinner } from '@/components/ui/Spinner';

/** Campaign QR landing: the server counts the visit, then we open the event with the code. */
export default function CampaignLinkPage() {
  const { token } = useParams<{ token: string }>();
  const router = useRouter();
  const { t } = useI18n();
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    call(api.GET('/api/public/campaigns/{token}', { params: { path: { token } } }))
      .then((c) => router.replace(`/events/${c.eventId}${c.active && c.code ? `?promo=${encodeURIComponent(c.code)}` : ''}`))
      .catch(() => setFailed(true));
  }, [token, router]);
  return (
    <main className="mx-auto grid min-h-[50vh] max-w-xl place-items-center px-4 text-center">
      {failed ? <p className="text-text-muted">{t('This promotion link is no longer active.')}</p> : <Spinner label="Opening the event" />}
    </main>
  );
}
