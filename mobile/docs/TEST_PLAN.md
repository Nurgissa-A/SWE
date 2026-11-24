# Check-in app test plan

Covers SRS 4.8 (mobile verification), 4.14 (campaign QR never admits), and the mobile parts of SRS 7 and 11.

## Environment

| Item | Value |
| --- | --- |
| Commit tested | |
| Date | |
| iPhone (model, iOS) | |
| Android (model, version) | |
| Data | Test data / real backend |

Result values: **Pass**, **Fail** (open a GitHub Issue and put its number), **Not run** (say why).

## Automated tests (`npm test`)

| Area | Tests | Result |
| --- | --- | --- |
| Verification: valid, used, cancelled, refunded, wrong event, promo QR, unknown, whitespace | 7 | |
| Check-in once, second check-in refused, undo, refunded cannot check in | 4 | |
| Attendee search by name, by code, empty search | 3 | |
| Wrong password, staff-only roles | 2 | |

## Manual journeys

| ID | Steps | Expected | iPhone | Android |
| --- | --- | --- | --- | --- |
| J1 | Sign in as admin | My Events list opens | | |
| J2 | Close Expo Go fully, reopen | Opens on My Events, no login | | |
| J3 | Open NU Tech Fest | Registered 120, Checked in 45 | | |
| J4 | Scan BF-T-1001, tap Check in | Green "Checked in"; back on dashboard count is 46 | | |
| J5 | Scan BF-T-1001 again | Orange "Already checked in" with time | | |
| J6 | Tap Undo check-in | Ticket valid again; count back to 45 | | |
| J7 | Search "dana", tap her, check in | Same result screen; status shows Checked in | | |
| J8 | Sign out | Confirmation, then login screen | | |

## Failure cases

| ID | Steps | Expected | iPhone | Android |
| --- | --- | --- | --- | --- |
| F1 | Sign in with wrong password | "Email or password is incorrect." | | |
| F2 | Sign in as attendee@biletflow.kz / att123 | "This app is only for event staff" | | |
| F3 | Sign in with empty fields | "Please enter your email and password." | | |
| F4 | Scan BF-T-1003 | Red "Ticket cancelled", no Check in button | | |
| F5 | Scan BF-T-1004 | Red "Ticket refunded", no Check in button | | |
| F6 | Scan BF-T-2001 while in NU Tech Fest | Red "different event" | | |
| F7 | Scan BF-CAMPAIGN-SPRING10 | Red "Promo QR code, not a ticket" | | |
| F8 | Scan any other QR (e.g. a website) | Red "Invalid ticket" | | |
| F9 | Deny camera permission | "Camera access is needed" + Allow button | | |
| F10 | Set SIMULATE_OFFLINE = true in mock.ts | Every screen shows the no-internet message + Try again | | |
| F11 | Hold the camera on one QR for 5 seconds | Result opens once, not repeatedly | | |

## Real backend (when available)

| ID | Steps | Expected | Result |
| --- | --- | --- | --- |
| R1 | USE_REAL_BACKEND = true, sign in with a real organizer | My Events shows that organizer's events | |
| R2 | Wrong password against real backend | "Email or password is incorrect." | |
| R3 | Unverified account | "Please verify your email first" | |
| R4 | Wait 15 minutes (token expiry), open an event | Back to login with "Session expired" | |
| R5 | Stop the backend, pull to reload | No-internet message + Try again | |
| R6 | Validation time: scan 10 tickets | Median under 2 s (SRS 7) | |

## Bugs

Open each failure as a GitHub Issue with label `bug` and `mobile`, and list it here.

| Issue | Found in | Summary | Status |
| --- | --- | --- | --- |
| | | | |
