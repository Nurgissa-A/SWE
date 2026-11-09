# Go Events service — Madi Weeks 1–4

Go owns physical-event drafts, visibility and publication. Python owns users,
organizer profiles and authentication. This service does not issue tokens.

## Run the integrated stack

From the repository root, with Docker Desktop/Compose and Python 3.12 available:

```sh
python3 scripts/init_local.py
docker compose up --build -d --wait --wait-timeout 180 python go
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/ready
```

Compose runs Python Alembic first, then `/migrate up` for Go, then Go. PostgreSQL
persists in `postgres-data`; `docker compose down` retains it. Mailpit is at
http://localhost:8025. Register, verify email, log in through Python on :8000,
then POST your own `/api/organizer-profile` before creating events on :8080.
Interactive shared mock: `docker compose --profile tools up -d prism` (:4010).
Python `/docs` covers identity only; the shared contract is `docs/openapi.yaml`.

For native development, use Go 1.26 (module minimum 1.25, required by pinned
pgx v5.11.0). In `backend-go`, configure these environment variables then run:

```sh
go run ./cmd/migrate up
go run ./cmd/api
```

| Variable | Meaning/default |
|---|---|
| `DATABASE_URL` | Required PostgreSQL URL, `postgresql://...` (no `+psycopg`) |
| `DB_MAX_CONNS` | 5, accepted range 1–50; pool lifetime 30m, idle 5m |
| `HTTP_ADDR` | `:8080` |
| `JWT_PUBLIC_KEY_FILE` | Required trusted RSA public PEM path |
| `JWT_ISSUER` | Required, Compose `biletflow-identity` |
| `JWT_AUDIENCE` | Required, Compose `biletflow-api` |
| `JWT_KEY_ID` | Required, Compose `biletflow-local-1` |
| `IDENTITY_URL` | Required trusted Python base URL; Compose `http://python:8000` |
| `CORS_ORIGINS` | Comma-separated exact origins, default `http://localhost:3000` |

Use localhost database/identity addresses for native processes; Compose service
names resolve only in the Docker network. Keep credentials in ignored `.env`;
never put them in Git, logs or shell history. Go mounts only the public key.

## Architecture and permissions

`cmd/api` injects `PostgresRepository → Service → Handler`. Existing package
boundaries are retained. `internal/auth` verifies RS256 and calls Python's live
`/api/users/me`; `internal/httpapi` handles exact-origin CORS, request IDs,
metadata-only JSON logs, a bounded body and request context. All repository
operations inherit cancellation and have a 3s deadline. Row locks and updates
share that deadline. Server handles SIGTERM/SIGINT, drains requests for up to
10s and closes the pool. `/health` is process liveness; `/ready` requires the Go
migration version and events table in PostgreSQL. Identity failures fail closed
for protected calls; readiness intentionally checks storage, not Python.

Service methods independently enforce verified organizer role and ownership;
tests call them directly to prevent HTTP-only authorization. The trusted owner
is the Python **user UUID**, never profile UUID or a client field. Admin roles
have no implicit organizer access. List visibility is applied in parameterized
SQL, not by loading everyone's private events into HTTP handlers.

| Route | Behavior |
|---|---|
| `POST /api/events` | Verified organizer; 201 own persistent UUID draft |
| `GET /api/events?status=draft\|published\|cancelled` | Bearer required; own events plus public published; optional status filter |
| `GET /api/events/{eventId}` | Owner preview; anonymous public/unlisted published; otherwise 404 |
| `PATCH /api/events/{eventId}` | Owner organizer; atomic partial draft update, 200 |
| `POST /api/events/{eventId}/publish` | Owner organizer; validated atomic draft → published, 200 |

List remains an array without pagination, as in the existing contract. Ordering
is `createdAt DESC, id`. Unknown valid UUID is 404; malformed/zero UUID is 400.
Missing authentication is 401; wrong role or mutation of another owner's event
is 403. Mutating a published/cancelled event is 409, including repeat publish.
Storage failures are sanitized 500, cancellation/deadline failures 503. Errors
use `{error:{code,message}}`; request IDs are in `X-Request-ID`.

Draft creation requires a nonblank title and RFC3339 `startsAt`. Visibility is
public (default), unlisted or private. Optional `endsAt` must be after start;
capacity is 1..2147483647. `venueId`, when present, is a nonzero UUID. Publication
also requires `venueId` because the SRS describes physical venue-based events.
Description/category remain optional; no invented mandatory content or future
start-date rule. Registration windows and images have no current API fields and
remain later contract work.

PATCH supports title, description, category, startsAt, endsAt, venueId, capacity
and visibility. Omitted fields retain their values; null/empty object, duplicate
keys and unknown fields are 400. Optional text can be set to an empty string.
Clearing nullable dates/venue/capacity through PATCH is deliberately not part of
the existing non-null PATCH contract. Failed validation rolls back every field.
A `SELECT ... FOR UPDATE` serializes PATCH/publish: independent patches preserve
one another and concurrent publications yield one 200, the rest 409.

## PostgreSQL and migrations

Go owns only schema `events`, table `events.events` and checksum ledger
`events.schema_migrations`. UUID primary key is cryptographically generated v4.
`organizer_id` references `public.identity_organizer_profiles.user_id` with
`ON DELETE RESTRICT`, which also references the Python user. Both services share
one database, so this is a valid cross-schema FK. Python Alembic remains unchanged
and manages default-schema identity tables only. Never downgrade identity before
removing dependent Go events; prefer disabling users to deleting business owners.

`venue_id` is the current contract's external UUID reference. No venue catalog
exists yet, so it has UUID validation but no fabricated FK or seeded production
venue. Future venue details/catalog must resolve that reference. Future ticket
types will have `event_id → events.events.id`; no ticket tables are created now.

Migration `001_events` adds DB CHECKs for title, statuses, visibility, dates,
capacity and venue on publication. Owner/status/date and partial public/date
indexes support scoped reads. Apply is transactional, guarded by an advisory
lock, idempotent and checksum verified. App startup never creates tables.

```sh
# Re-run safely in Compose
docker compose run --rm go-migrate up
# Destructive rollback ONLY on a disposable development database:
# docker compose run --rm go-migrate down
```

Back up retained data before schema downgrades. Down drops the events table and
removes version 1, retaining the empty schema and ledger. There is no migration
of the former in-memory data: it never survived process exit.

## Verification

Use a separate **disposable database whose name ends `_go_test`**. The Go suite
exercises migration down/up and must never target development or production data.
First apply the actual Python migration to that test database (from
`backend-python`, `DATABASE_URL` uses `postgresql+psycopg://...`):

```sh
alembic upgrade head
```

Then in `backend-go`, with `TEST_DATABASE_URL=postgresql://.../..._go_test`:

```sh
test -z "$(gofmt -l .)"
go vet ./...
go test -race ./... -v
go build ./cmd/api ./cmd/migrate
```

Without TEST_DATABASE_URL, only PostgreSQL integration skips locally; in CI a
missing variable is a failure. In-memory repository is compiled only into tests.
Integration covers real migrations/constraints/FK, reopen persistence, concurrent
updates/publications, rollback, timeout/cancellation, persisted handlers and closed
pool errors. Existing JWT negative fixtures remain intact.

From the repository root, install the locked Python tooling and run real HTTP
verification (creates labeled example.com users/events; restarts Go):

```sh
python3 -m venv .venv
.venv/bin/pip install -r backend-python/requirements.lock
.venv/bin/python scripts/smoke_identity.py
.venv/bin/python scripts/smoke_integration.py
npm ci --ignore-scripts
npm run api:lint
npm run api:types
```

HTTP smoke checks actual Python JWTs, onboarding, ownership, PATCH, physical-venue
publication, conflicts, OpenAPI schemas, Go restart persistence, development CORS
and immediate password-reset revocation. Do not run against a production stack.
CI also keeps all Python tests and checks generated frontend types for drift.

DELETE/unpublish are still explicitly planned shared-contract routes, outside
Madi's five required Weeks 1–4 operations. No tickets, checkout or payment system
is implemented here. Before production: deploy with TLS, least-privilege DB roles,
backups/monitoring and a real venue catalog. See `docs/go-events-weeks-1-4.md`.
