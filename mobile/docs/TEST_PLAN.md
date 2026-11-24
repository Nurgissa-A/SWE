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

## Real backend (when available)

| ID | Steps | Expected | Result |
| --- | --- | --- | --- |
| R1 | USE_REAL_BACKEND = true, sign in with a real organizer | My Events shows that organizer's events | |
| R2 | Wrong password against real backend | "Email or password is incorrect." | |
| R3 | Unverified account | "Please verify your email first" | |
| R4 | Wait 15 minutes (token expiry), open an event | Back to login with "Session expired" | |
| R5 | Stop the backend, pull to reload | No-internet message + Try again | |
| R6 | Validation time: scan 10 tickets | Median under 2 s (SRS 7) | |

## Summary

Tested on 2026-11-24 by Aigerim Muratova: 16/16 automated tests passed; 8/8 journeys and 11/11 failure cases passed on iPhone with test data. Android and real-backend checks (R1-R6) not yet run.

## Bugs

Open each failure as a GitHub Issue with label `bug` and `mobile`, and list it here.

| Issue | Found in | Summary | Status |
| --- | --- | --- | --- |
| none | | No bugs found in this run | |
