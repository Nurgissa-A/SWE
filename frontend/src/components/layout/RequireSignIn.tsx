'use client';

import { useEffect, type ReactNode } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { LoadingScreen } from './LoadingScreen';

/** Sends signed-out visitors to the sign-in page and brings them back afterwards. */
export function RequireSignIn({ children }: { children: ReactNode }) {
  const { token, user, isLoading, ready } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  useEffect(() => {
    if (ready && !token) router.replace(`/login?next=${encodeURIComponent(pathname)}`);
  }, [ready, token, pathname, router]);
  if (!token || isLoading || !user) return <LoadingScreen label={token ? 'Loading your account' : 'Taking you to sign in'} />;
  return <>{children}</>;
}
