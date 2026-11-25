# Check-in app test plan

Covers SRS 4.8 (mobile verification), 4.14 (campaign QR never admits), and the mobile parts of SRS 7 and 11.

## Environment

| Item | Value |
| --- | --- |
| Commit tested | 92606d4 |
| Date | 2026-11-24 |
| iPhone (model, iOS) | iPhone 15 Pro, iOS 27.2 |
| Android (model, version) | Not tested: no Android device available yet |
| Data | Test data (USE_REAL_BACKEND = false) |

Result values: **Pass**, **Fail** (open a GitHub Issue and put its number), **Not run** (say why).

## Automated tests (`npm test`)

| Area | Tests | Result |
| --- | --- | --- |
| Verification: valid, used, cancelled, refunded, wrong event, promo QR, unknown, whitespace | 7 | Pass |
| Check-in once, second check-in refused, undo, refunded cannot check in | 4 | Pass |
| Attendee search by name, by code, empty search | 3 | Pass |
| Wrong password, staff-only roles | 2 | Pass |

## Manual journeys

| ID | Steps | Expected | iPhone | Android |
| --- | --- | --- | --- | --- |
| J1 | Sign in as admin | My Events list opens | Pass | Not run |
| J2 | Close Expo Go fully, reopen | Opens on My Events, no login | Pass | Not run |
| J3 | Open NU Tech Fest | Registered 120, Checked in 45 | Pass | Not run |
| J4 | Scan BF-T-1001, tap Check in | Green "Checked in"; back on dashboard count is 46 | Pass | Not run |
| J5 | Scan BF-T-1001 again | Orange "Already checked in" with time | Pass | Not run |
| J6 | Tap Undo check-in | Ticket valid again; count back to 45 | Pass | Not run |
| J7 | Search "dana", tap her, check in | Same result screen; status shows Checked in | Pass | Not run |
| J8 | Sign out | Confirmation, then login screen | Pass | Not run |

## Failure cases

| ID | Steps | Expected | iPhone | Android |
| --- | --- | --- | --- | --- |
| F1 | Sign in with wrong password | "Email or password is incorrect." | Pass | Not run |
| F2 | Sign in as attendee@biletflow.kz / att123 | "This app is only for event staff" | Pass | Not run |
| F3 | Sign in with empty fields | "Please enter your email and password." | Pass | Not run |
| F4 | Scan BF-T-1003 | Red "Ticket cancelled", no Check in button | Pass | Not run |
| F5 | Scan BF-T-1004 | Red "Ticket refunded", no Check in button | Pass | Not run |
| F6 | Scan BF-T-2001 while in NU Tech Fest | Red "different event" | Pass | Not run |
| F7 | Scan BF-CAMPAIGN-SPRING10 | Red "Promo QR code, not a ticket" | Pass | Not run |
| F8 | Scan any other QR (e.g. a website) | Red "Invalid ticket" | Pass | Not run |
| F9 | Deny camera permission | "Camera access is needed" + Allow button | Pass | Not run |
| F10 | Set SIMULATE_OFFLINE = true in mock.ts | Every screen shows the no-internet message + Try again | Pass | Not run |
| F11 | Hold the camera on one QR for 5 seconds | Result opens once, not repeatedly | Pass | Not run |

## Real backend

Run on 2026-11-24 against main at commit 8a2855e: full stack on the laptop (`docker compose up`: PostgreSQL, identity, Go business API, Mailpit) and the web app (`npm run dev`, port 3000). App started with `EXPO_PUBLIC_API_URL=http://<laptop-ip>:3000 npx expo start`. Ticket issued through the web app: organizer created and published an event with a free ticket type; a second, attendee account registered and received the QR ticket.

| ID | Steps | Expected | Result |
| --- | --- | --- | --- |
| R1 | Sign in on the phone as the real organizer | My Events shows the organizer's event | Pass |
| R7 | Scan the attendee's real QR ticket | Ticket admitted (checked in by the server) | Pass |
| R8 | Scan the same ticket again | Orange "Already checked in" | Pass |
| R9 | Open the event dashboard | Checked-in count is 1 | Pass |
| R10 | Search the attendee by name | Attendee found with status Checked in | Pass |
| R2 | Wrong password against real backend | "Email or password is incorrect." | Pass, 2026-11-25 ([screenshot](evidence/r2-wrong-password.png)) |
| R3 | Unverified account | Sign-in refused with a clear message | Pass, 2026-11-25: "Email verification required" ([screenshot](evidence/r3-unverified-account.png)) |
| R4 | Wait 15 minutes (token expiry), open an event | Back to login with "Session expired" | Not run |
| R5 | Stop the backend (`docker compose stop`), open an event | Clear error + Try again, no false result | Pass with note, 2026-11-25: "The server returned an error (500). Please try again." + Try again. The web proxy answers 500 when the API is down, so the app shows the server-error text, not the no-internet text ([screenshot](evidence/r5-backend-stopped.png)) |
| R6 | Validation time: scan the same real ticket 5 times while recording the screen | Each result under 2 s (SRS 7) | Pass, 2026-11-25: first scan admitted, next four "Already checked in"; each result within 1 s of the QR entering the frame, read from the recording at 4 frames per second ([video](evidence/r6-five-scans-2026-11-25.mp4)) |
| R11 | Print the ticket PDF in black and white, scan it from paper, then scan again | Admitted, then "Already checked in" | Pass, 2026-11-26 ([video](evidence/r11-printed-ticket-2026-11-26.mp4)) |

## Summary

Test data (2026-11-24, commit 92606d4): 16/16 automated tests; 8/8 journeys and 11/11 failure cases passed on iPhone.
Real backend (2026-11-24, commit 8a2855e): a real ticket issued by the web app was admitted once and refused on the second scan; counts and search matched.
Real backend (2026-11-25 and 2026-11-26, iPhone): R2, R3, R5, R6 and R11 passed (R5 with a note); R4 and Android not run.

Known bug (2026-11-25): with English selected, screen titles and the Back button stay in Russian (visible in the R5 and R11 evidence).

## Bugs

Open each failure as a GitHub Issue with label `bug` and `mobile`, and list it here.

| Issue | Found in | Summary | Status |
| --- | --- | --- | --- |
| none | | No bugs found in this run | |
