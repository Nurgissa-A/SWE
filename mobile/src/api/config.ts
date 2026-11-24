// ⚙️ One place to switch between fake data and the team's real backend.
import { Platform } from 'react-native';

// Real backend by default. For a demo without a backend, start the app with
//   EXPO_PUBLIC_USE_MOCK=1 npx expo start
export const USE_REAL_BACKEND = process.env.EXPO_PUBLIC_USE_MOCK !== '1';

// Your laptop's address on the Wi-Fi / hotspot.
// Find it with:  ipconfig (Windows, "IPv4 Address")  or  ipconfig getifaddr en0 (Mac)
// It changes when you switch networks, so update it if login stops working.
const LAPTOP_IP = '192.168.0.10';

// EXPO_PUBLIC_API_URL can point at the BiletFlow web app (e.g. http://192.168.0.10:3000), which
// forwards /api/auth and /api/users to the identity service and everything else to the events
// service. Without it, the app talks to the two services directly.
// In a browser the app is served next to the website, so it uses that address.
const API_URL = process.env.EXPO_PUBLIC_API_URL
  ?? (Platform.OS === 'web' && typeof window !== 'undefined' ? window.location.origin : undefined);
export const IDENTITY_URL = API_URL ?? `http://${LAPTOP_IP}:8000`; // Python: login, users/me
export const EVENTS_URL = API_URL ?? `http://${LAPTOP_IP}:8080`; // Go: events, check-in

// Give up waiting after this many milliseconds
export const REQUEST_TIMEOUT_MS = 10000;
