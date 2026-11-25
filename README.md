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
├── mobile/           Mobile application
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
