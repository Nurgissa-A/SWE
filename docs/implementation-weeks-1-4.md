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

- Python/PostgreSQL: **50 passed**, zero failed. Includes migration/schema drift, persistence, single-use concurrency, all four roles, privilege injection, current-user status, organizer access, password reset/revocation and shared response schema checks.
- Go: **15 top-level tests**, 31 including named subtests, passed with race detector; all five original Event tests retained. `go vet` and `gofmt` checks pass.
- Actual local Docker + SMTP and Python-to-Go HTTP smoke passed; includes forged/expired signatures, non-owner publication, attendee write denial, draft isolation and revocation reaching Go.
- OpenAPI validator, Spectral (no warnings/errors), regenerated API types and standalone TypeScript type check passed.
- Host Prism 4010 and container Prism on alternate loopback port 4011 returned a valid registration mock. Default container port is configurable with PRISM_PORT.
- Exact corrected Auth commit `dfc9661` passed all three GitHub jobs in run 40513531032. Original Auth formatting failure is fixed by committed root Ruff configuration. Final integration checks must be inspected on the Go PR, not inferred from this earlier run.
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

1. Madi still owns PostgreSQL Event persistence, PATCH/DELETE/unpublish, broader Event validation and Event UUID-format alignment. Event records remain in memory and disappear on Go restart. This does not block the implemented identity persistence or secured existing routes, but prevents calling the entire Event/MVP implementation complete.
2. SRS one-primary-backend recommendation versus the existing Python/Go division remains a documented team/instructor architecture decision. No duplicate implementation or forced redesign was introduced.
3. Six high-severity upstream audit entries remain in the development-only Prism/Postman/Faker chain after compatible fixes. Use only trusted local YAML; no public exposure/untrusted Postman imports. These Node packages are absent from the Python runtime. See api-contract.md.
4. Synchronous SMTP is functional and tested, but an accepted-email/failed-database-commit window may produce an unusable link. Production durable delivery, provider credentials/TLS, distributed abuse controls and deployment-specific encryption/monitoring require operational review. Development Mailpit is not a production mail provider.
5. One active RSA key pair is supported; rotation is coordinated through deployment/restart. No production finance, check-in/ticketing Weeks 5-12, frontend page rewrite or mobile feature was implemented.
6. Frontend page.tsx was a comment-only file before this work; no functional frontend build is claimed. Only its generated API types changed and were type-checked; page/component logic and mobile files remain untouched.
