import type { Metadata } from 'next';
import '@fontsource/onest/400.css';
import '@fontsource/onest/500.css';
import '@fontsource/onest/600.css';
import '@fontsource/onest/700.css';
import '@fontsource/unbounded/600.css';
import '@fontsource/unbounded/700.css';
import { AuthProvider } from '@/hooks/useAuth';
import { SiteHeader } from '@/components/layout/SiteHeader';
import './globals.css';

export const metadata: Metadata = {
  title: {
    default: 'BiletFlow — Find your next event',
    template: '%s | BiletFlow',
  },
  description: 'Discover events and create memorable experiences with BiletFlow.',
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" data-scroll-behavior="smooth">
      <body className="font-sans">
        <AuthProvider>
          <SiteHeader />
          {children}
        </AuthProvider>
      </body>
    </html>
  );
}
