'use client';

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';
import useSWR from 'swr';
import { useSWRConfig } from 'swr';
import { authApi, getAuthToken, setAuthToken } from '@/lib/api-client';
import { ApiRequestError, toApiRequestError } from '@/lib/api-errors';
import { loginAccount, type LoginInput, type User } from '@/lib/auth';

type AuthContextValue = {
  user: User | null | undefined;
  token: string | null;
  isLoading: boolean;
  error: unknown;
  login: (input: LoginInput) => Promise<User>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<User | undefined>;
};

type AuthProviderProps = { children: ReactNode };

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: AuthProviderProps) {
  const [token, setToken] = useState<string | null>(() => getAuthToken());
  const { mutate: globalMutate } = useSWRConfig();
  const userKey = token ? (['current-user', token] as const) : null;
  const { data: fetchedUser, error, isLoading, mutate } = useSWR<User>(
    userKey,
    async () => {
      const { data, error: responseError, response } = await authApi.GET('/api/users/me');
      const status = response.status;
      if (responseError) throw toApiRequestError(responseError, status);
      if (!data) throw toApiRequestError(undefined, status);
      return data;
    },
    {
      revalidateOnFocus: false,
      shouldRetryOnError: false,
      onError: (requestError) => {
        if (requestError instanceof ApiRequestError && requestError.status === 401) {
          setAuthToken(null);
          setToken(null);
        }
      },
    },
  );

  const login = useCallback(async (input: LoginInput) => {
    const result = await loginAccount(input);
    setAuthToken(result.token);
    setToken(result.token);
    await globalMutate(['current-user', result.token], result.user, { revalidate: false });
    return result.user;
  }, [globalMutate]);

  const logout = useCallback(async () => {
    const previousToken = token;
    setAuthToken(null);
    setToken(null);
    if (previousToken) {
      await globalMutate(['current-user', previousToken], undefined, { revalidate: false });
    }
  }, [globalMutate, token]);

  const refreshUser = useCallback(() => mutate(), [mutate]);
  const user = token ? fetchedUser : null;

  const value = useMemo<AuthContextValue>(() => ({
    user,
    token,
    isLoading: Boolean(token) && isLoading,
    error: token ? error : undefined,
    login,
    logout,
    refreshUser,
  }), [user, token, isLoading, error, login, logout, refreshUser]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used inside AuthProvider.');
  return context;
}
