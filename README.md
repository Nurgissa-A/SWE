# SWE

Software Engineering team project.

## Team

| Member | Responsibility |
|---|---|
| Aliar | Python Backend |
| Madi | Go Backend |
| Nurgul | Frontend |
| Aigerim | Frontend & UI/UX Design |
| Aikookia | Mobile Application |

## Project Structure

```text
SWE/
├── backend-python/   Python backend
├── backend-go/       Go backend
├── frontend/         Web frontend
├── mobile/           Mobile application (Expo door check-in app)
├── e2e/              End-to-end browser tests (web app and check-in app)
├── scripts/          Local setup, demo data and smoke tests
├── docs/             Architecture and API documentation
├── .gitignore
├── CONTRIBUTING.md
└── README.md
```

## Development Workflow

All development should be done through feature branches.

```text
main
 ↓
feature branch
 ↓
commit
 ↓
push
 ↓
Pull Request
 ↓
review
 ↓
merge into main
```

Do not push unfinished work directly to `main`.

## What BiletFlow does

BiletFlow is an event ticketing platform for Kazakhstan, built from the SRS (initial draft):

- **Attendees** browse events and register or buy tickets with a short hold timer. Payments use
  a card simulator. Attendees get e-mailed PDF tickets with signed QR codes, can cancel free
  registrations, and can open support cases (for example, to ask for a refund).
- **Organizers** create and publish events with ticket types. They activate paid sales (terms
  and an activation fee), run promo codes, add door staff, see analytics and orders, issue
  refunds, cancel events, and keep an append-only event history.
- **Door staff** check people in with the mobile app: QR scan, attendee search, undo and live
  counts, online only.
- **Platform admins** moderate events, users, orders, payments, activations, promo activity,
  reports and settings.
- The web and mobile apps and the ticket e-mails are available in **Kazakh, Russian and English**.
  Account verification e-mails from the identity service are still English only.

| Part | Stack | Where |
|---|---|---|
| Identity (accounts, sign-in, JWT) | Python, FastAPI | `backend-python/` |
| Business API (events, checkout, tickets, check-in, admin) | Go | `backend-go/` |
| Web app | Next.js, React, Tailwind | `frontend/` |
| Door check-in app | Expo, React Native | `mobile/` |
| Shared API contract | OpenAPI 3 | `docs/openapi.yaml` |

## Run everything locally

```bash
python3 scripts/init_local.py                       # secrets and .env, once
docker compose up --build -d python go mailpit      # identity, business API, test mailbox
python3 scripts/seed_demo.py                        # demo events, orders and accounts
docker compose exec python python -m app.cli set-role admin@biletflow.kz --role platform_admin
cd frontend && npm install && npm run dev           # http://localhost:3000
```

E-mails (verification links and tickets) arrive in Mailpit at http://localhost:8025. All demo
accounts use the password `demo12345`:

| Account | Role |
|---|---|
| `aigerim@example.kz` | attendee (Kazakh) |
| `arman@example.kz` | attendee (Russian) |
| `organizer@biletflow.kz` | organizer of the demo events |
| `staff@biletflow.kz` | door staff for Jazz Night (can undo) and the Hackathon |
| `admin@biletflow.kz` | platform admin |

For the check-in app see [mobile/README.md](mobile/README.md), and for the browser tests see
[e2e/README.md](e2e/README.md).

## Local backend environment

See [Python backend setup](backend-python/README.md) for Docker Compose, PostgreSQL migrations, health checks and testing. Existing Go and frontend development commands remain available.

JWT service integration and review boundaries are documented in [Python-Go authentication](docs/python-go-auth.md).

## Local HTTPS with the identity integration

From the repository root, with Docker Desktop running and ports 8443 and 8025 available:

```sh
python3 scripts/init_local.py
python3 scripts/init_tls.py
docker compose -p biletflow-integrated -f compose.yaml -f deploy/compose.https.yaml up --build -d --wait
```

Use [BiletFlow](https://localhost:8443) and [Mailpit](http://localhost:8025).
For browser trust, import only the local public CA at `.secrets/tls/ca.crt`.
The separate Compose project uses its own database volume. If the previous
local stack occupies either port, stop its containers in Docker Desktop first.
Register and verify accounts through Mailpit. To enable the Users tab for a
verified local account, assign its platform administrator role:

```sh
docker compose -p biletflow-integrated -f compose.yaml -f deploy/compose.https.yaml exec python \
  python -m app.cli set-role admin@example.com --role platform_admin
```

Sign in again after the role change. An organizer can add a different verified
account on the event's Staff tab. Language preferences are saved in Account.
