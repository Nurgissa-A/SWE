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

## Real backend or test data

All API calls go through `src/api/index.ts`; the switch is in `src/api/config.ts`.

- **Real backend (default).** Sign-in uses the identity service. Events, scanning, check-in,
  undo and attendee search use the events service's scanner endpoints (`/api/scanner/...`):
  - The list shows only events this account may check people in for (its own, or ones it was
    added to as door staff), with live counts.
  - **A scanned QR checks the ticket in straight away.** The server verifies the signed QR and
    checks it in in one atomic step, so two phones can never both admit the same ticket. The
    result screen shows "Checked in", with Undo if the organizer allowed it.
  - Picking someone in **Search attendee** only looks the ticket up; **Check in** is a separate
    tap, as before.
  - Door staff are ordinary BiletFlow accounts that an organizer added to an event. The app
    accepts organizers, event admins and anyone with at least one event to check in.
- **Where the backend is.** Set `EXPO_PUBLIC_API_URL` to the BiletFlow web app address (for
  example `http://192.168.0.10:3000`), which forwards to both services. Or set `LAPTOP_IP` in
  `config.ts` to talk to the services directly on ports 8000 and 8080. The browser build uses
  the address it is served from.
- **Test data.** Start with `EXPO_PUBLIC_USE_MOCK=1 npx expo start`. Demo login
  `admin@biletflow.kz` / `admin123`. The test buttons on the scanner and the QR codes in
  [docs/test-tickets.png](docs/test-tickets.png) only work with test data.

## Languages

Kazakh, Russian and English (`src/i18n.tsx`). Switch on the sign-in screen or the events list;
the choice is remembered and sent to the servers as `X-Locale`. `npm run i18n:check` lists any
phrase without a translation.

## Tests

```sh
npm test            # 31 automated tests: verification rules, duplicate check-in, undo, promo QR rejection,
                    # search, roles (test data) and the real-backend wiring against a fake server
npm run typecheck   # type check
npm run i18n:check  # every phrase has Kazakh and Russian
npm run export:web  # browser build (used by the end-to-end tests in ../e2e)
```

Manual test plan and results: [docs/TEST_PLAN.md](docs/TEST_PLAN.md).

## Structure

```
src/
  api/          config (switch), http client, real + mock API, session, storage, errors
  app/          screens (expo-router): index (login), events/, scan, result, search
  components/   ErrorView
  i18n.tsx      Kazakh / Russian / English and the language switch
  dates.ts      dates in the event's time zone with our own month names (works without kk ICU data)
  __tests__/    automated tests
```
