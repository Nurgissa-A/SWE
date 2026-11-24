import { Platform } from 'react-native';
import * as SecureStore from 'expo-secure-store';

// Keychain (iPhone) / Keystore (Android) on phones. Browsers have no secure store, so the web build
// keeps values for the current tab only (sessionStorage).
const web = () => { try { return globalThis.sessionStorage ?? null; } catch { return null; } };

export const storage = {
  get: (key: string): Promise<string | null> =>
    Platform.OS === 'web' ? Promise.resolve(web()?.getItem(key) ?? null) : SecureStore.getItemAsync(key),
  set: (key: string, value: string): Promise<void> =>
    Platform.OS === 'web' ? Promise.resolve(web()?.setItem(key, value)) : SecureStore.setItemAsync(key, value),
  remove: (key: string): Promise<void> =>
    Platform.OS === 'web' ? Promise.resolve(web()?.removeItem(key)) : SecureStore.deleteItemAsync(key),
};
