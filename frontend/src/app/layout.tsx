import type { Metadata } from 'next';
import { cookies } from 'next/headers';
import '@fontsource/onest/400.css';
import '@fontsource/onest/500.css';
import '@fontsource/onest/600.css';
import '@fontsource/onest/700.css';
import '@fontsource/unbounded/600.css';
import '@fontsource/unbounded/700.css';
import { AuthProvider } from '@/hooks/useAuth';
import { SiteHeader } from '@/components/layout/SiteHeader';
import { I18nProvider } from '@/lib/i18n';
import { LOCALE_COOKIE, isLocale, type Locale } from '@/lib/locale';
import './globals.css';

export const metadata: Metadata = {
  title: {
    default: 'BiletFlow — Find your next event',
    template: '%s | BiletFlow',
  },
  description: 'Discover events and create memorable experiences with BiletFlow.',
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  // The language cookie lets the server render the chosen language, so there is no flash of Russian.
  const saved = (await cookies()).get(LOCALE_COOKIE)?.value;
  const locale: Locale = isLocale(saved) ? saved : 'ru';
  return (
    <html lang={locale} data-scroll-behavior="smooth">
      <body className="font-sans">
        <I18nProvider initialLocale={locale}>
          <AuthProvider>
            <SiteHeader />
            {children}
          </AuthProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
