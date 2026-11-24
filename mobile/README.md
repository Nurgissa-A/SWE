# BiletFlow Check-in (mobile)

React Native (Expo) app for **event admins** to verify tickets and check attendees in (SRS 4.8).
Owner: Aigerim Muratova (Aikookia).

## Features

| Screen | What it does |
| --- | --- |
| Login | Staff sign-in; attendees are refused; token kept in secure storage (iOS Keychain); stays signed in; sign out |
| My Events | Events the admin can check in for |
| Event dashboard | Registered and checked-in counts; Scan ticket; Search attendee |
| Scanner | Camera QR scanning; one scan per code |
| Result | Valid / already checked in / cancelled / refunded / different event / promo QR rejected / invalid; colour + icon + text |
| Check-in | Check in once, undo an accidental check-in |
| Attendee search | Find by name or ticket code; manual check-in |

Every screen shows a clear message with **Try again** when the network or server fails (online-only, SRS 4.8).

## Run

```sh
cd mobile
npm install
npx expo start
```

Scan the QR code in the terminal with Expo Go (iPhone camera or the Expo Go app on Android). Phone and laptop must be on the same network; otherwise use `npx expo start --tunnel`.

## Test data or real backend

All API calls go through `src/api/index.ts`. The switch is in `src/api/config.ts`:

- `USE_REAL_BACKEND = false` (default): test data in `src/api/mock.ts`. Demo login `admin@biletflow.kz` / `admin123`.
- `USE_REAL_BACKEND = true`: login (`POST /api/auth/login`), current user (`GET /api/users/me`) and events (`GET /api/events`) use the identity service (port 8000) and events service (port 8080). Set `LAPTOP_IP` to the laptop's address (`ipconfig getifaddr en0`).

Ticket verification, check-in, undo and attendee search use test data until the backend provides those endpoints.

Test QR codes for every result type: [docs/test-tickets.png](docs/test-tickets.png).

## Tests

```sh
npm test        # 16 automated tests: verification rules, duplicate check-in, undo, promo QR rejection, search, roles
npx tsc --noEmit  # type check
```

Manual test plan and results: [docs/TEST_PLAN.md](docs/TEST_PLAN.md).

## Structure

```
src/
  api/          config (switch), http client, real + mock API, session, errors
  app/          screens (expo-router): index (login), events/, scan, result, search
  components/   ErrorView
  __tests__/    automated tests
```
