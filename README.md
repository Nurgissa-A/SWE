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

## Aliar component release through Week 12

Local Docker HTTPS, Python support/notifications/admin/images and release/recovery
are documented in [the local release runbook](docs/local-release.md).
See [Weeks 1–12 scope and dependencies](docs/aliar-weeks-1-12.md) and
[executed validation](docs/aliar-validation.md). This is an Aliar component
release candidate; Go business modules and whole-team acceptance remain separate.
