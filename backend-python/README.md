# BiletFlow Python identity service

Owner: Aliar. Python owns identity, verification and organizer profiles; Go owns events and future ticket/order business data.

## Docker development

From the repository root:

```sh
python3 scripts/init_local.py
docker compose up --build -d
docker compose logs migrate python
curl http://localhost:8000/health
curl http://localhost:8000/ready
```

The script generates ignored local RSA keys and a random database password. PostgreSQL persists in `postgres-data`. Never use `docker compose down -v` on data you need. Compose runs Alembic before the application; readiness fails if the database/migrations are unavailable. Go remains on port 8080, web development on 3000, Python on 8000. Mailpit at http://localhost:8025 is a local mail sink, **not** a production email provider. All published ports bind to loopback.

## Python development

Use Python 3.12. Create a virtual environment, install the complete lock and the local package without resolving additional dependencies:

```sh
python3 -m venv .venv
.venv/bin/pip install -r backend-python/requirements.lock
.venv/bin/pip install --no-deps -e backend-python
cp backend-python/.env.example backend-python/.env
# Set DATABASE_URL to the actual local password and database.
cd backend-python
../.venv/bin/alembic upgrade head
../.venv/bin/uvicorn app.main:app --reload --no-access-log
```

`/docs`, `/redoc`, `/openapi.json` expose identity documentation. Keep access logging off for verification/reset query data; the service emits sanitized JSON request metadata. Startup never creates database tables implicitly. Shutdown disposes pooled connections. Test/development dependencies are reproducibly pinned in `requirements.lock`.

```sh
.venv/bin/ruff check backend-python scripts
.venv/bin/ruff format --check backend-python scripts
.venv/bin/pytest backend-python/tests
```

Run migrations from `backend-python/`. Downgrade is destructive and intended for disposable test databases only. Never let Python migrations create or change Go-owned Event tables.

## Authentication and permissions

Register, consume the single-use token from the Mailpit message with `POST /api/auth/verify-email`, then log in. The returned `token` is a 15-minute RS256 bearer JWT. Use `Authorization: Bearer <token>` for current-user/profile operations. Request bodies and validation inputs are not logged; identity bodies are limited to 16 KiB. Never paste real passwords or tokens into issue reports.

Registration always creates **attendee**. A verified attendee may create their **own** organizer profile with `POST /api/organizer-profile`; this promotes the current DB role to organizer. `GET` reads it; `PATCH` replaces contact settings. Only `demo:` payout labels or null are accepted. No real KYC, payout or financial integration is provided. The existing reset-password contract is implemented with expiring single-use tokens and JWT revocation.

Privileged roles have no public assignment API. An authorized operator with container/DB access may run:

```sh
docker compose exec python python -m app.cli set-role verified-staff@example.com --role event_admin
docker compose exec python python -m app.cli set-role verified-admin@example.com --role platform_admin
docker compose exec python python -m app.cli disable-user verified-staff@example.com
```

These commands invalidate old JWTs; log in again. Event Admin's platform role grants no access to arbitrary events. Event-specific assignment/check-in is Go-owned future work. Protect operator/DB access as an administrative privilege.

## PostgreSQL tests

Use a **separate database ending in `_test`**, never the demonstration/production database. The suite truncates Python identity tables in that test database. In the example below set the password to your generated local value without committing it:

```sh
docker compose exec postgres createdb -U biletflow biletflow_test
export TEST_DATABASE_URL='postgresql+psycopg://biletflow:YOUR_LOCAL_PASSWORD@localhost:5432/biletflow_test'
.venv/bin/pytest backend-python/tests -v
```

CI creates this database independently. Tests inject a memory mail adapter and temporary RSA keys; real mail/production credentials are never required. See [API contract/tooling](../docs/api-contract.md), [ERD](../docs/erd.md) and [architecture decisions](../docs/architecture.md). SMTP delivery's transactional limitation and production deployment boundaries are documented there.
