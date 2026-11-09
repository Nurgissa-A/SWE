export class ApiRequestError extends Error {
  readonly status?: number;

  constructor(message: string, status?: number) {
    super(message);
    this.name = 'ApiRequestError';
    this.status = status;
  }
}

const statusMessages: Record<number, string> = {
  400: 'Some details need attention. Review the form and try again.',
  401: 'Your session has expired or the sign-in details are incorrect.',
  403: 'This action is not available for your account.',
  404: 'We could not find that page or event.',
  405: 'This operation is not available in the current backend yet.',
  409: 'This action conflicts with existing information. Review the details and try again.',
  501: 'This operation is not available in the current backend yet.',
};

export function getApiErrorMessage(error: unknown, status?: number): string {
  if (error instanceof ApiRequestError) return error.message;
  return toApiRequestError(error, status).message;
}

function getServerMessage(payload: unknown): string | undefined {
  if (!payload || typeof payload !== 'object') return undefined;

  if ('error' in payload) {
    const error = payload.error;
    if (typeof error === 'string' && error.trim()) return error;
    if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string' && error.message.trim()) {
      return error.message;
    }
  }

  if ('message' in payload && typeof payload.message === 'string' && payload.message.trim()) {
    return payload.message;
  }

  return undefined;
}

export function toApiRequestError(payload: unknown, status?: number): ApiRequestError {
  const serverMessage = getServerMessage(payload);
  if (serverMessage) return new ApiRequestError(serverMessage, status);

  if (payload instanceof Error && payload.message) {
    const message = payload instanceof TypeError
      ? 'We could not reach the service. Check your connection and try again.'
      : payload.message;
    return new ApiRequestError(message, status);
  }

  if (status && statusMessages[status]) {
    return new ApiRequestError(statusMessages[status], status);
  }

  if (status && status >= 500) {
    return new ApiRequestError('The service is temporarily unavailable. Try again shortly.', status);
  }

  return new ApiRequestError('Something went wrong. Please try again.', status);
}
