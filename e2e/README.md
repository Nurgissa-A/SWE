# End-to-end tests

Browser tests for the whole product, run with Playwright against a running local stack with
the demo data loaded. They create real orders and check-ins, so use a development database only.

| Project | What it covers |
|---|---|
| `web` | Attendee browsing and buying in Kazakh: declined card, then a paid order with QR. Organizer overview and every event hub tab. Every admin console tab. A scan of every screen in Kazakh and Russian for leftover English. |
| `door-app` | The check-in app (`mobile/`) on a phone-sized screen. It covers translated sign-in errors, attendees refused, and only assigned events with counts. A **fake camera shows a real ticket QR**: checked in on scan, undo, check in again, then a second scan is refused. The same ticket is refused at another event in Kazakh, with no undo offered. Attendee search opens the same result screen. |

Every test also fails if the page throws or logs an error.

## Run

1. Start the stack and load the demo data (see the root README).
2. Start the web app on port 3000, or set `WEB_URL`.
3. Build the check-in app for the web: `cd mobile && npm install && npm run export:web`.
   The tests serve it on port 3200 and forward `/api` to the web app.
4. Run the tests:

```bash
cd e2e
npm install
npx playwright install chromium   # once
npm test                          # or: npm run test:web / npm run test:door
npm run report                    # open the HTML report
```

`ffmpeg` is needed to turn the ticket QR into the fake camera video. Without it, the two camera
tests are skipped.

| Variable | Default |
|---|---|
| `WEB_URL` | `http://localhost:3000` |
| `DOOR_APP_URL` | `http://localhost:3200` |
| `DOOR_APP_DIST` | `../mobile/dist` |
| `MOBILE_DIR` | `../mobile` (for the barcode decoder's WebAssembly file) |
| `DEMO_PASSWORD` | `demo12345` |

CI runs the same suite in `.github/workflows/e2e.yml`.
