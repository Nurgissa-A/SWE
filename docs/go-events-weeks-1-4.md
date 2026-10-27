# Madi — Go Events/Data Weeks 1–4 handover

## Scope and inspected baseline

The live repository was fetched before work. `origin/main` was `d0ce370`, with
Python Auth PR #4 and Go JWT integration PR #5 already merged by a teammate;
there were no open PRs. The working tree was clean. Existing Go handler tests,
RS256/live-identity verification, Python identity implementation, Compose,
OpenAPI and the supplied SRS/task assignments were reviewed. No teammate
implementation or unmerged dependency was overwritten. Main was not committed
to or merged by this work.

Baseline: Go had authenticated create/list/detail/publish backed only by a map,
32-hex IDs, HTTP-only ownership checks, no PATCH, no migrations or restart
persistence. This change completes that existing Event feature rather than
creating another backend/auth system. One coherent PR contains the repository
interface, SQL implementation, transactional service and HTTP contract together,
so every reviewable commit can build without a broken intermediate interface.

## Implementation

- Five `/api/events` routes persist canonical UUID drafts and enforce service
  role/owner/visibility rules. PATCH preserves omitted fields; publication is a
  validated draft-only transition. Both serialize with PostgreSQL row locks.
- PostgreSQL schema `events` separates Go migrations from Python Alembic.
  The organizer FK references the real profile's `user_id`, not a duplicate user.
  Constraints/indexes and a transactional checksum/advisory-lock migration ledger
  cover invariants and repeatable startup. No in-memory runtime fallback exists.
- Existing Python JWT verifier is reused, including live revocation/role lookup.
  Only the public key enters Go. CORS, request IDs, sanitized JSON logs,
  bounded requests/pool, cancellation, health/readiness and graceful shutdown
  complete the runtime boundary.
- Shared OpenAPI and generated frontend types describe actual behavior. PATCH
  gains optional category/venueId; physical-event publication requires venueId;
  repeat publication/published edit returns 409. No frontend UI or mobile code
  is modified. Python application/migration files remain unchanged.

## Actual validation

Local Go toolchain: Docker `golang:1.26-alpine`, PostgreSQL 16; dedicated
`biletflow_go_test` initialized with the real Python migration. Commands:

```sh
# In backend-go, TEST_DATABASE_URL points to the disposable *_go_test DB:
test -z "$(gofmt -l .)"
go vet ./...
go test -race ./... -json
go build ./cmd/api ./cmd/migrate
```

Result: **28 top-level tests, 84 including named subtests; no failures or skips**.
This includes original JWT/handler tests, direct service denial, visibility matrix,
input/UUID validation, real PG up/down/idempotence/checksum/FK/CHECKs, pool reopen,
rollback, concurrent PATCH, concurrent publish, cancellation/lock timeout and
persistent HTTP handlers. CI requires the database variable and runs race tests
against its own PostgreSQL service rather than silently skipping integration.

```sh
.venv/bin/pytest backend-python/tests -q
.venv/bin/ruff check backend-python scripts
.venv/bin/ruff format --check backend-python scripts
# With DATABASE_URL pointing at the Go test DB, from backend-python:
alembic check
npm run api:lint
npm run api:types
.venv/bin/python -m openapi_spec_validator docs/openapi.yaml
node_modules/.bin/tsc --noEmit --skipLibCheck frontend/src/lib/api-types.ts
```

Python: **50 passed**, one existing upstream Starlette/httpx deprecation warning.
Alembic check with Go schema present: **no new upgrade operations**. Ruff,
OpenAPI validation/Spectral and generated TypeScript check passed. A full frontend
app build is not claimed: the existing homepage is an unfinished scaffold.

```sh
docker compose up --build -d --wait --wait-timeout 180 python go
.venv/bin/python scripts/smoke_identity.py
.venv/bin/python scripts/smoke_integration.py
```

Both real SMTP/HTTP smoke scripts passed. The second obtains JWTs from actual
Python registration/verification/login, onboards two organizers, denies an
attendee/non-owner, creates/patches/publishes a real Event, validates Event schemas,
checks 400/409 paths and CORS, restarts Go and verifies identical persisted data,
then resets the owner's password and verifies immediate JWT rejection by Go.
It creates labeled demo accounts/events in the local database and prints no
credentials. The unit/PG suites use a separate disposable database.

GitHub CI status and review URL are reported in the PR and final delivery after
checks complete; local success alone is not a claim of passing remote CI.

## Task assessment

| Task | Result and evidence |
|---|---|
| 1 ERD/entities/ownership | Done: ERD, native UUIDs, organizer FK, documented venue/future TicketType boundary |
| 2 shared OpenAPI | Done: existing paths preserved, constraints/errors reviewed, types regenerated |
| 3 architecture | Done: handler → service → repository; context/config/CORS/logging/shutdown/probes |
| 4 PostgreSQL | Done: bounded pool, transactional migrations and real repository/constraints/indexes |
| 5 initial tests | Done: validation, service/handler/repository, failures and missing resources |
| 6 create | Done: real Python organizer identity, UUID draft, PostgreSQL insert |
| 7 list | Done: SQL visibility/status filter, stable order, existing array contract |
| 8 detail | Done: UUID validation, preview/public/unlisted/private rules, 404 |
| 9 PATCH | Done: partial owned draft changes, field validation, transaction and denial tests |
| 10 publish | Done: venue validation, one atomic transition, conflict and concurrency tests |

All 23 Definition-of-Done items are implemented and locally verified; the final
review/CI state is recorded in the delivery message. PR merge remains the owner's
decision. Full run/config/migration instructions are in
[backend-go/README.md](../backend-go/README.md).

## Remaining boundaries

- DELETE/unpublish remain explicitly planned routes, outside the five assigned
  operations. Tickets/orders/payments/check-in and registration-window/image API
  fields are later work, not implemented placeholders.
- `venueId` is validated and required for publication, but no venue catalog/details
  API exists. Frontend/team must supply a real reference when that catalog lands;
  arbitrary catalog contents or cross-database FKs have not been invented.
- Frontend owners must use separate Python :8000 / Go :8080 bases, an organizer
  account, bearer token and venueId before publication. Nullable field clearing is
  not in the current non-null PATCH contract.
- Shared-database deployments must apply identity then Go migrations; same DB
  constraints are intentional. Production TLS/DB roles/key provisioning and the
  existing two-backend SRS deviation require deployment/team decisions.
- Existing development-only Prism upstream advisories and the SMTP commit/delivery
  window documented in prior work are unchanged. No production readiness claim
  beyond the explicitly tested academic scope is made.
