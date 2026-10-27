# BiletFlow architecture and decisions

## Implemented foundation

- Python FastAPI owns identity (`identity_users`, organizer profiles, action tokens and delivery receipts).
- PostgreSQL 16 supplies persistent identity storage; Alembic applies migrations before Python readiness passes.
- Go retains its handler/service/repository Event architecture and currently uses an in-memory repository.
- Next.js and mobile source are preserved. They are consumers, not trusted authorities for roles or organizer IDs.
- Docker Compose exposes Python 8000, Go 8080, Mailpit 8025/1025 and optional Prism 4010 on loopback. It does not pretend that port 8000 serves Go Event routes.

## ADR 001 — Existing two-backend deviation

Status: documented repository-direction decision; instructor/product scope approval remains open.

SRS v0.3 section 9 recommends one primary backend implementation. The agreed team allocation and existing repository instead separate Aliar's Python identity work from Madi's Go Event work. Retain these bounded responsibilities without duplicating Event/User functionality. This introduces two runtimes, cross-service JWT validation, deployment coordination and failure handling. No further service split is introduced. If the team/instructor rejects the exception, a separately approved migration/rebaseline is required; this implementation does not claim the discrepancy has disappeared.

## ADR 002 — Persistence ownership

Python creates only `identity_` tables in the existing database. The prefix avoids silently claiming a shared `users`/`events` namespace. Go owns its own future business migrations. Apply identity migrations first when business tables need identity references; coordinate the shared migration order before Go persistence lands. Event persistence, ticket relations and event-specific staff assignments remain Madi's responsibility. See [ERD](erd.md).

## ADR 003 — RS256 and live identity

Python alone holds the RSA private key. Tokens have fixed RS256, `kid`, `iss`, `aud`, UUID `sub`/`jti`, `iat`, `nbf`, `exp`, role snapshot and integer `ver`. TTL defaults to 15 minutes. Public key PEM and JWKS contain no signing secret. Python loads and validates the matching key pair at startup. Replace keys through the deployment secret mechanism, not Git.

Every protected Python request looks up the current user in PostgreSQL and rejects inactive/deleted/unverified users or changed token versions. RBAC uses the current database role, not a stale role claim. Organizer onboarding promotes a verified attendee to organizer for their own profile; administrator roles are never self-assigned. Operator role changes/disabling and password resets invalidate existing tokens by incrementing `token_version`. Go integration must verify the signature independently and confirm current identity through `/api/users/me` before authorization; identity failure must fail closed.

Web clients use the existing `token` response field; secure client-side storage/session design remains the frontend owner's responsibility. No refresh/logout endpoint is added to the contract: client sign-out clears its bearer token; operator/reset revocation is immediate via live identity, otherwise bearer tokens expire within the configured TTL.

## ADR 004 — Email delivery boundary

The injectable SMTP adapter supports STARTTLS and credentials. Mailpit is a local sink only. Identity action tokens contain 256 random bits and are stored hashed; raw tokens appear only in the email and tests. Synchronous SMTP failure returns 503 and rolls back the pending registration/reset, so the caller can retry. A narrow SMTP-accepted/database-commit failure window can produce an unusable emailed link; this is explicitly not a transactional delivery guarantee. A durable encrypted outbox/retry design is future work, not a claimed production feature. Do not deploy this academic adapter as a production delivery system without that review.

## Threat boundary and deployment

No password, bearer token, raw action token or request body is logged. Request validation errors omit input values. Argon2id hashes passwords. CORS is restricted to configured origins; hosts are allow-listed. Internal HTTP is restricted to the private Compose network for the demo; put both services behind trusted HTTPS ingress outside localhost. Key generation creates ignored files and a random local DB password. Restrict operator/DB/key access. Production SMTP/TLS, infrastructure encryption, distributed rate limiting and audit/monitoring requirements need deployment-specific configuration; none are presented as already operated production controls.
