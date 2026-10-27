# Shared API contract and implementation status

`docs/openapi.yaml` is the shared contract. Existing `/api` paths, camelCase DTOs and login `{token,user}` response remain intact. Do not drop `/api` from client requests. There is no combined gateway yet: route Auth/User/Profile to Python 8000 and Events to Go 8080. Prism on 4010 mocks both; a mock response is not an implemented backend.

## Identity implementation

| Route | Behavior |
|---|---|
| POST /api/auth/register | 201 attendee only, normalized email, Argon2id hash and verification email; duplicate 409, invalid/extra fields 422, SMTP unavailable 503 |
| POST /api/auth/verify-email | 200 once; invalid, expired, reused or wrong-purpose token 400 |
| POST /api/auth/login | 200 token + user; invalid/inactive 401; unverified 403; no-store response |
| POST /api/auth/request-password-reset | Generic 200 for known/unknown addresses; configured mail adapter, no raw token in response |
| POST /api/auth/reset-password | Single-use 200; invalidates prior access/reset tokens; invalid/expired/reused 400 |
| GET /api/users/me | Live identity; missing, invalid, revoked, inactive/deleted/unverified user 401 |
| PATCH /api/users/me | Own displayName only; role/id injection rejected |
| POST /api/organizer-profile | Verified attendee/organizer creates own profile and becomes organizer; repeated create 409; admin roles forbidden |
| GET /api/organizer-profile | Organizer sees own record only; missing 404, wrong role 403 |
| PATCH /api/organizer-profile | Replaces own contact fields; optional fields default to null; contactEmail required |

All identity errors use `{error:{code,message}}` and never echo credentials or validation input. HTTP 401 includes `WWW-Authenticate: Bearer`. Empty success bodies for verification/reset retain existing 200 semantics.

## Explicit contract changes in v0.2

- Added identity error payloads/statuses and required response fields without removing existing response properties.
- Added organizer profile PATCH; existing create/read paths remain. POST is explicitly create-only, returning 409 for duplicates.
- Password/email/token/display-name bounds and extra-field rejection are documented; public roles/identities cannot be injected.
- Login requires verified email. `payoutAccountPlaceholder` now accepts **only** `demo:` labels to prevent collection of real payout details. This is a deliberate input validation tightening: consumers must use a simulation label or null.
- Servers now identify their actual route subsets rather than incorrectly claiming the frontend port serves every API.
- Frontend generated types are regenerated from this contract; frontend page/component logic is untouched.

Go create/list/detail/PATCH/publish now persist to PostgreSQL and enforce authorization in the service. DELETE/unpublish remain planned. See [Go README](../backend-go/README.md).

Madi's Event contract update preserves `/api`, camelCase and array list responses.
PATCH adds optional `category` and `venueId` so incomplete physical-event drafts
can be completed. PATCH null/empty/unknown fields are rejected; omitted values
are unchanged. Title must be nonblank, capacity is a positive PostgreSQL integer,
and end follows start. Publish now requires venueId (physical-event SRS) and
returns 409 for a non-draft state. Clients that previously published without a
venue must supply one on create or PATCH. Event response fields always present
at runtime are now required in the schema, making generated TypeScript safer.
Actual HTTP smoke responses are validated against this schema, including after
service restart. No frontend pages/components were changed.

## Tools

Python: `http://localhost:8000/docs`, `/redoc`, `/openapi.json` (implemented identity routes only).

From root, Node 24.18+ and Python environment:

```sh
npm ci --ignore-scripts
npm run api:lint
.venv/bin/python -m openapi_spec_validator docs/openapi.yaml
npm run api:mock
npm run api:types
```

Prism binds to 127.0.0.1:4010 and serves the **shared** static contract, including planned Go operations. It has no real Auth, email or persistence. Use only trusted repository specs. Do not expose it publicly. The optional Compose tools profile supplies the same mock independently of host Node tooling.

When changing a contract, update examples/validation and generated types in the same PR; involve Madi and Nurgul before changing Event semantics. CI validates syntax, Spectral rules and actual identity response shapes against the shared contract.

### Tooling advisory boundary

The pinned development-only Prism dependency chain currently brings upstream `postman-collection` / Faker advisories. Compatible lodash/uuid fixes are overridden in the root lock, but six high-severity transitive audit entries remain in the mock tooling chain. There is no upstream compatible patched `http-spec` release at the time of implementation. No Prism/Node tooling is installed in the Python runtime image, and no untrusted Postman/spec inputs or public interfaces are supported. Run it only for this trusted local YAML on loopback/private Docker networking. Track the upstream upgrade; do not apply `npm audit fix --force` or incompatible major overrides merely to suppress the report.
