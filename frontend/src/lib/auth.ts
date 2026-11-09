import type { paths } from './api-types';
import { toApiRequestError } from './api-errors';
import { authApi } from './api-client';

export type User = paths['/api/auth/login']['post']['responses'][200]['content']['application/json']['user'];
export type LoginInput = paths['/api/auth/login']['post']['requestBody']['content']['application/json'];
export type RegisterInput = paths['/api/auth/register']['post']['requestBody']['content']['application/json'];

export async function loginAccount(input: LoginInput) {
  const { data, error, response } = await authApi.POST('/api/auth/login', { body: input });
  const status = response.status;
  if (error) throw toApiRequestError(error, status);
  if (!data) throw toApiRequestError(undefined, status);
  return data;
}

export async function registerAccount(input: RegisterInput) {
  const { data, error, response } = await authApi.POST('/api/auth/register', { body: input });
  const status = response.status;
  if (error) throw toApiRequestError(error, status);
  if (!data) throw toApiRequestError(undefined, status);
  return data;
}

export async function verifyEmailToken(token: string): Promise<void> {
  const { error, response } = await authApi.POST('/api/auth/verify-email', {
    body: { token },
  });
  const status = response.status;
  if (error) throw toApiRequestError(error, status);
}

export async function requestPasswordReset(email: string): Promise<void> {
  const { error, response } = await authApi.POST('/api/auth/request-password-reset', {
    body: { email },
  });
  if (error) throw toApiRequestError(error, response.status);
}

export async function resetPassword(token: string, newPassword: string): Promise<void> {
  const { error, response } = await authApi.POST('/api/auth/reset-password', {
    body: { token, newPassword },
  });
  if (error) throw toApiRequestError(error, response.status);
}
