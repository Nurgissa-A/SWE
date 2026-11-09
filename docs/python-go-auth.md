# Python and Go authentication integration

## Trust and route boundary

Python on 8000 issues tokens; the frontend/API client sends the existing `Authorization: Bearer <token>` header to Go on 8080. Python signs **RS256** using its private RSA key. Go receives only the public PEM key. Both services enforce the same `JWT_ISSUER`, `JWT_AUDIENCE` and `JWT_KEY_ID` values. The public `/.well-known/jwks.json` endpoint is also available for clients, but Go deliberately uses the deployed pinned public PEM, not arbitrary token-controlled key URLs.

Required claims: UUID `sub` and `jti`, known role snapshot, integer `ver`, `iss`, `aud`, `iat`, `nbf`, `exp`. Go pins RS256 and rejects unknown kid, missing claims, wrong signature/issuer/audience, future nbf/iat and expired tokens. It then calls the trusted configured Python `/api/users/me` with the same bearer token and a two-second timeout. Python validates current DB status and token version; Go checks the returned subject and verification status and uses its **current database role**. This prevents disabled/deleted/revoked users and old roles from remaining authorized until JWT expiry. Network or identity failure fails closed (503), while invalid/revoked identity is 401. Redirects are not followed.

## Existing Event route authorization

- POST `/api/events`: authenticated organizer only; owner ID comes from live principal, never request JSON. Unknown fields/forged `organizerId` and trailing JSON are rejected.
- GET `/api/events`: bearer required as in the shared contract. Organizers see their own records plus publicly published events; other roles see publicly published events only.
- GET `/api/events/{id}`: owner may see their own private/draft record; anonymous visitors may see public/unlisted **published** details. Private/draft records return 404 to other users. An invalid supplied bearer is rejected, not downgraded to anonymous.
- PATCH `/api/events/{id}` and POST `/api/events/{id}/publish`: verified organizer and exact event owner only, enforced independently in the service. Row-locked draft mutations persist atomically. Event Admin and Platform Admin do not automatically gain organizer write permission.

Event Admin staff assignments/check-in remain future Go business work; this role alone authorizes no arbitrary Event writes. DELETE/unpublish remain planned. Event persistence, canonical UUIDs, PATCH and publication validation are implemented in Madi's Weeks 1–4 work.

## Startup and key operations

```sh
python3 scripts/init_local.py
docker compose up --build -d --wait python go
.venv/bin/python scripts/smoke_identity.py
.venv/bin/python scripts/smoke_integration.py
```

Or run Go locally with the same values, from `backend-go/`:

```sh
export JWT_PUBLIC_KEY_FILE=../.secrets/jwt-public.pem
export JWT_ISSUER=biletflow-identity
export JWT_AUDIENCE=biletflow-api
export JWT_KEY_ID=biletflow-local-1
export IDENTITY_URL=http://localhost:8000
# Set DATABASE_URL securely to the existing PostgreSQL database first.
# Apply Python Alembic before this Go migration.
go run ./cmd/migrate up
go run ./cmd/api
```

Use the supported Go 1.26 toolchain used by CI and the pinned image. `go.mod` now requires Go 1.25 for pinned pgx v5.11.0; CI and Docker use 1.26. Missing/invalid key or verifier configuration aborts startup. Go never mounts the private key. Public key, private key and kid rotation requires a coordinated deployment/restart; this MVP supports one active key pair and does not claim transparent multi-key rollover. Provision production keys through the deployment secret mechanism. Demo inter-service HTTP stays inside the private Docker network; use trusted TLS/service authentication for separate hosts.

## Tests and limitations

`go vet ./...`, `go test -race ./... -v` and the CI live Compose job validate signature/expiry/role/owner denial. The smoke journey obtains actual Python tokens through registration + SMTP email verification + login, onboards two unrelated organizers, creates/publishes an event with the authenticated owner, blocks non-owner modification and attendee writes, checks draft visibility, rejects forged/expired tokens and confirms password-reset revocation reaches Go immediately.

The smoke commands create clearly named disposable demo users/events in the local demo database. They print neither passwords nor tokens. Go PostgreSQL tests use a separate disposable `*_go_test` database. The smoke patches and publishes a persisted event, validates actual Event response schemas, restarts Go and confirms identical data. Python identity/key material also persists. This completes the five required Event operations, not the later ticket/order/payment MVP.

## Madi Weeks 1–4 persistence update

The five implemented Event routes now use PostgreSQL and service-level
permissions, including PATCH. Python RS256/live-identity verification is reused
unchanged. Organizer subject UUID references the existing organizer profile's
`user_id` in the same database. Go mounts only the public key. PATCH and publish
hold a row lock through validation/commit. See [Go README](../backend-go/README.md)
for current routes, schema ownership and run/test commands. DELETE and unpublish
remain planned. The HTTP interop smoke now validates response schemas and actual
PostgreSQL persistence across a Go process restart.
