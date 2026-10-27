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
