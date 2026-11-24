// ⚙️ One place to switch between fake data and the team's real backend.

// false = fake data (works anywhere, good for demos)
// true  = talk to Aliar's + Madi's backend running on your laptop
export const USE_REAL_BACKEND = false;

// Your laptop's address on the Wi-Fi / hotspot.
// Find it with this terminal command:  ipconfig getifaddr en0
// It changes when you switch networks, so update it if login stops working.
const LAPTOP_IP = '192.168.0.10';

export const IDENTITY_URL = `http://${LAPTOP_IP}:8000`; // Python: login, users/me
export const EVENTS_URL = `http://${LAPTOP_IP}:8080`; // Go: events

// Give up waiting after this many milliseconds
export const REQUEST_TIMEOUT_MS = 10000;
