import { Alert } from 'react-native';
import { router } from 'expo-router';
import { clearSession } from './session';

// Same shape as the backend's error format: { error: { code, message } }
export class ApiError extends Error {
  code: string;
  constructor(code: string, message: string) {
    super(message);
    this.code = code;
  }
}

export function friendlyMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return 'Something went wrong. Please try again.';
}

// Use this in every screen's catch block.
// If the login has expired, it signs out and goes back to the login screen.
// Otherwise it shows the error message on the screen.
export async function handleApiError(err: unknown, setError: (message: string) => void) {
  if (err instanceof ApiError && err.code === 'unauthorized') {
    await clearSession();
    Alert.alert('Session expired', 'Please sign in again.');
    router.replace('/');
    return;
  }
  setError(friendlyMessage(err));
}
