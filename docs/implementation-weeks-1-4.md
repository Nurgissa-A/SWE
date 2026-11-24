# Aliar Weeks 1 to 4 implementation and handover

## Implemented scope

FastAPI identity service with PostgreSQL/Alembic, injectable development SMTP, Argon2id passwords, single-use hashed verification/reset tokens, RS256 JWT, live identity/revocation checks, four-role reusable RBAC, operator role management and own organizer profiles. Docker Compose, pinned Python/npm dependencies, API validation/Prism/Spectral, ERD and cross-service security documentation are supplied. The isolated Go integration resolves the hardcoded organizer path and enforces ownership without implementing Madi's remaining Event business work.

## Task completion matrix

| Task | Assessment | Concrete evidence |
|---|---|---|
| 1 Repository and Python structure | Implemented/tested | app API/core/db/models/repositories/schemas/services; isolated feature commits |
| 2 Docker Compose | Implemented/tested | PostgreSQL volume/health, migration service, Python/Go, Mailpit; actual local startup |
| 3 CI | Implemented and executed | Python/PostgreSQL, Go race/vet/format, contract/type/Prism jobs; live Compose integration added with Go PR |
| 4 FastAPI | Implemented/tested | ASGI factory/lifespan, health/ready, typed DTOs, safe errors/logs and docs |
| 5 PostgreSQL | Implemented/tested | Four identity tables, constraints, Alembic, schema-drift and persistence tests |
| 6 ERD | Implemented/documented | docs/erd.md distinguishes implemented Python tables from planned Go entities |
| 7 Shared OpenAPI | Implemented/validated | Original /api paths/login shape preserved; explicit new validation/error/PATCH semantics; generated types |
| 8 API tools | Implemented/exercised | Swagger/ReDoc, host and container Prism HTTP, Spectral, schema validator; upstream dev-tool advisories remain documented |
| 9 Registration | Implemented/tested | Normalization, unique email, attendee-only, Argon2id and persistence |
| 10 Email verification | Implemented/tested | Random hashed expiring tokens, atomic single-use, SMTP adapter and Mailpit journey |
| 11 Login | Implemented/tested | Verified/active identity, generic bad-credential rejection, bounded JWT TTL |
| 12 Current user | Implemented/tested | JWT + live DB lookup, 401 for removed/inactive/revoked identity |
| 13 JWT | Implemented/tested | RS256, issuer/audience/kid/claims, private/public separation and JWKS |
| 14 RBAC | Implemented/tested | Four roles, reusable dependencies, safe public onboarding, operator-only admin assignment; no global event-admin access |
| 15 Organizer profile | Implemented/tested | Own create/read/replace, server user identity, simulation-only payout label |
| 16 Python-Go Auth | Implemented/tested for existing slice | Real Python-issued JWT -> Go create/list/detail/publish, owner/role/privacy checks, invalid/expired signature and live reset-revocation smoke |

Implementation acceptance is distinct from review/merge. Check actual PR/check status before merging. No PR is automatically merged by these scripts or this implementation.

## Validation record

- Python/PostgreSQL: **67 passed**, zero failed. Includes migration/schema drift, persistence, single-use and multi-link reset concurrency, post-lock identity revocation, sanitized DB timeouts, strict JWT claim types, all four roles, privilege injection, current-user status, organizer access, password reset/revocation and shared response schema checks.
- Go: **15 top-level tests**, 31 including named subtests, passed with race detector; all five original Event tests retained. `go vet` and `gofmt` checks pass.
- Actual local Docker + SMTP and Python-to-Go HTTP smoke passed; includes forged/expired signatures, non-owner publication, attendee write denial, draft isolation and revocation reaching Go.
- OpenAPI validator, Spectral (no warnings/errors), regenerated API types and standalone TypeScript type check passed.
- Host Prism 4010 and container Prism on alternate loopback port 4011 returned a valid registration mock. Default container port is configurable with PRISM_PORT.
- Historical Auth commit `dfc9661` passed its original checks. The November completion PR adds real PostgreSQL concurrency/failure regressions and restarts Python in the SMTP smoke; current PR CI must be inspected on that exact head.
- One test-client upstream deprecation warning remains: Starlette recommends migrating its httpx-based test client integration to httpx2. Tests still pass; no warning is suppressed.

## Run and test

From repository root:

```sh
python3 scripts/init_local.py
docker compose up --build -d --wait python go
# API: http://localhost:8000/docs and /redoc; mail: http://localhost:8025
# Optional mock:
docker compose --profile tools up --build -d prism

python3 -m venv .venv
.venv/bin/pip install -r backend-python/requirements.lock
.venv/bin/pip install --no-deps --no-build-isolation -e backend-python
.venv/bin/python scripts/smoke_identity.py
.venv/bin/python scripts/smoke_integration.py

# Create disposable DB once, then set actual generated local password privately:
docker compose exec postgres createdb -U biletflow biletflow_test
export TEST_DATABASE_URL='postgresql+psycopg://biletflow:YOUR_LOCAL_PASSWORD@localhost:5432/biletflow_test'
.venv/bin/pytest backend-python/tests -v
.venv/bin/ruff check backend-python scripts
.venv/bin/ruff format --check backend-python scripts
npm ci --ignore-scripts
npm run api:lint
.venv/bin/python -m openapi_spec_validator docs/openapi.yaml
(cd backend-go && go vet ./... && go test -race ./... -v)
```

Never use the test URL against a database that contains real data; the fixture requires the database name to end in `_test` and truncates identity tables. Local credential/key files are ignored and must not be attached to PRs. To apply migrations manually: `docker compose run --rm migrate`. Do not remove the persistent volume unless intentionally discarding local data.

## Known dependencies and limitations

1. Madi’s Event persistence/PATCH/UUID/lifecycle implementation exists in open PR #6 but is not merged into current main. Main still uses the in-memory Go repository and loses Event records on Go restart. DELETE/unpublish remain later work. This does not block the implemented identity persistence or secured existing routes, but prevents calling the entire Event/MVP implementation complete.
2. SRS one-primary-backend recommendation versus the existing Python/Go division remains a documented team/instructor architecture decision. No duplicate implementation or forced redesign was introduced.
3. Six high-severity upstream audit entries remain in the development-only Prism/Postman/Faker chain after compatible fixes. Use only trusted local YAML; no public exposure/untrusted Postman imports. These Node packages are absent from the Python runtime. See api-contract.md.
4. Synchronous SMTP is functional and tested, but an accepted-email/failed-database-commit window may produce an unusable link. Production durable delivery, provider credentials/TLS, distributed abuse controls and deployment-specific encryption/monitoring require operational review. Development Mailpit is not a production mail provider.
5. One active RSA key pair is supported; rotation is coordinated through deployment/restart. No production finance, check-in/ticketing Weeks 5-12, frontend page rewrite or mobile feature was implemented.
6. Frontend PR #7 has since merged real Auth/Event pages and API proxy. `npm ci --ignore-scripts && npm run build` now passes on the current main baseline. This Aliar completion does not modify frontend or mobile source. The frontend install reports 6 dependency advisories (5 high, 1 critical); those require a frontend-owned dependency update, not an unreviewed backend-scope change.


## November 24 completion audit

Base: current `origin/main` `026c017`, including frontend PR #7. The earlier Aliar
PRs #2–#5 are merged. Madi PR #6 and mobile PR #8 remain separate open work;
this completion neither imports nor merges them.

The new regression tests reproduced a reset deadlock and organizer onboarding
that succeeded after disable/role change/revocation. Fixes use a single lock
order (User → ActionToken), check expiry after lock acquisition, and resolve live
identity/permissions while holding the account lock. Read-only identity lookup
remains unlocked. Login and reset-link creation serialize with account changes.
No schema change or new endpoint is needed.

JWT verification now rejects malformed/compact UUID identities, bad jti,
boolean/nonfinite dates and invalid roles/version values. Finite fractional
numeric dates remain accepted. The signing algorithm, key distribution,
issuer/audience and normal Python-issued JWTs are unchanged. PostgreSQL pool,
connection, statement and lock waits are bounded and failures map to sanitized
503 responses, already defined by OpenAPI. CI cannot silently skip missing-DB
tests. Compose forwards Python-specific CORS, frontend mail origin and DB limits.

Validation: 67 Python/PostgreSQL tests passed without skips; Ruff, shared OpenAPI
validator/Spectral and generated-type drift checks passed. Frontend production
build passed unchanged. The expanded actual SMTP smoke validates profile/user
updates and persistence through Python restart; Go HTTP smoke validates actual
Python JWTs, roles, ownership, forged/expired signature rejection and password-reset
revocation. Exact final remote CI/review status is in the delivery PR and reply.

All 16 assigned implementation tasks are complete within Weeks 1–4. PR approval
and merge remain separate human decisions. Production mail provider credentials,
TLS/deployment operations and the existing one-backend SRS deviation are explicit
operational/team boundaries, not unimplemented identity endpoints.


## Scope of the later user request

This is the historical Weeks 1–4 acceptance record, not an assertion that the full
12-week assignment was complete. The user subsequently required Aliar through
Week 12 and selected local Docker HTTPS. The new platform/delivery/release work
is assessed separately in the full task matrix. In particular, the old synchronous
SMTP window and client-only logout policy are superseded by the encrypted outbox
and per-session revocation implementation; 67 is the historical baseline count.
