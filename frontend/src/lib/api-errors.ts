import { currentLocale } from './i18n';
import { translate, type Params } from './locale';

export class ApiRequestError extends Error {
  readonly status?: number;
  readonly code?: string;
  readonly params?: Params;

  constructor(message: string, status?: number, code?: string, params?: Params) {
    super(message);
    this.name = 'ApiRequestError';
    this.status = status;
    this.code = code;
    this.params = params;
  }
}

const statusMessages: Record<number, string> = {
  400: 'Some details need attention. Review the form and try again.',
  401: 'Your session has expired or the sign-in details are incorrect.',
  403: 'This action is not available for your account.',
  404: 'We could not find that page or event.',
  405: 'This operation is not available in the current backend yet.',
  409: 'This action conflicts with existing information. Review the details and try again.',
  429: 'Too many attempts. Please wait a moment and try again.',
  501: 'This operation is not available in the current backend yet.',
};

/** A translated, user-facing message for any error (server messages are English phrase keys). */
export function getApiErrorMessage(error: unknown, status?: number): string {
  const e = error instanceof ApiRequestError ? error : toApiRequestError(error, status);
  return translate(currentLocale(), e.message, e.params);
}

type ServerError = { message?: string; code?: string; params?: Params };

function getServerError(payload: unknown): ServerError | undefined {
  if (!payload || typeof payload !== 'object') return undefined;
  if ('error' in payload) {
    const error = payload.error;
    if (typeof error === 'string' && error.trim()) return { message: error };
    if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string' && error.message.trim()) {
      const e = error as { message: string; code?: unknown; params?: unknown };
      return {
        message: e.message,
        code: typeof e.code === 'string' ? e.code : undefined,
        params: e.params && typeof e.params === 'object' ? (e.params as Params) : undefined,
      };
    }
  }
  if ('message' in payload && typeof payload.message === 'string' && payload.message.trim()) {
    return { message: payload.message };
  }
  return undefined;
}

export function toApiRequestError(payload: unknown, status?: number): ApiRequestError {
  const server = getServerError(payload);
  // Generic identity/validation messages are replaced by friendlier text for that status.
  if (server?.message && server.message !== 'Request validation failed') {
    return new ApiRequestError(server.message, status, server.code, server.params);
  }
  if (payload instanceof Error && payload.message) {
    const message = payload instanceof TypeError ? 'We could not reach the service. Check your connection and try again.' : payload.message;
    return new ApiRequestError(message, status);
  }
  if (status && statusMessages[status]) return new ApiRequestError(statusMessages[status], status);
  if (status && status >= 500) return new ApiRequestError('The service is temporarily unavailable. Try again shortly.', status);
  return new ApiRequestError('Something went wrong. Please try again.', status);
}

/** Unwrap an openapi-fetch result: return data or throw a typed ApiRequestError. */
export async function call<T>(promise: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await promise;
  if (error !== undefined || !response.ok) throw toApiRequestError(error, response.status);
  return data as T;
}

/** Like call() for endpoints that answer 204 No Content. */
export async function callEmpty(promise: Promise<{ error?: unknown; response: Response }>): Promise<void> {
  const { error, response } = await promise;
  if (error !== undefined || !response.ok) throw toApiRequestError(error, response.status);
}
