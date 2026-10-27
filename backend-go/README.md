# Go Backend

Owner: Madi

Handles the Event service (see `docs/architecture.md` and `docs/openapi.yaml` for the shared contract).

## Structure

```text
backend-go/
├── cmd/api/main.go          entrypoint, HTTP server, route wiring
└── internal/event/
    ├── model.go             Event struct (matches openapi.yaml)
    ├── repository.go        persistence interface + in-memory stub
    ├── service.go           business rules (validation, publish logic)
    ├── handler.go           HTTP layer, maps routes to service calls
    └── handler_test.go      tests against the in-memory repository
```

Layering: handler -> service -> repository. The service has no HTTP or
DB dependency, so it can be tested in isolation.

## Status (Weeks 1-2)

- In-memory repository only — swap for Postgres once Aliar's DB setup lands.
- JWT authentication now validates Python RS256 tokens and current DB identity;
  create/publish require an organizer and publication requires event ownership.
  See `docs/python-go-auth.md` for configuration and the separate review boundary.
- Endpoints implemented: `POST /api/events`, `GET /api/events`,
  `GET /api/events/{id}`, `POST /api/events/{id}/publish`.
  `PATCH`/`DELETE`/`unpublish` are in `openapi.yaml` but not yet implemented
  — next up for Week 3-4.

## Run locally

```bash
# Configure JWT_PUBLIC_KEY_FILE, JWT_ISSUER, JWT_AUDIENCE, JWT_KEY_ID
# and IDENTITY_URL as documented in docs/python-go-auth.md.
go run ./cmd/api
# server listens on :8080
curl http://localhost:8080/health
```

## Test

```bash
go test ./... -v
```

Use Go 1.26 for the supported CI/build toolchain. The handler-service-repository architecture and in-memory persistence boundary remain unchanged. All five original tests are preserved with explicit test principals; additional tests cover authentication and ownership.
