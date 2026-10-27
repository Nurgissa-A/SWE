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
