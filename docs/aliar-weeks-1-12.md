# Aliar Weeks 1–12: implementation and acceptance matrix

Scope: all Python/architecture/infrastructure responsibilities in the retained
12-week allocation, with local Docker HTTPS chosen by the user. The user explicitly
excluded implementing Madi's Go business work. “Implemented” below describes Aliar's
code or operator deliverable; it does not assert peer approval, merged PRs, or final
whole-team acceptance. Original Week 1–4 delivery is recorded in
[the earlier handover](implementation-weeks-1-4.md).

| Task | Aliar delivery | External acceptance/dependency |
|---|---|---|
| W01-AL1 | Monorepo setup, generated environment, README | None for local setup |
| W01-AL2 | FastAPI health/readiness, Docker and PostgreSQL | None |
| W01-AL3 | Identity/business ownership and initial API inventory | Team agreement/review remains human |
| W02-AL1 | PostgreSQL, ERD and separate migration/runtime ownership | Madi owns business migrations |
| W02-AL2 | Shared OpenAPI Auth/Event/error/bearer schemas | Planned Go routes explicitly marked |
| W02-AL3 | Swagger/ReDoc, Prism/Spectral/types, Python/Go/web/release CI | Approval/merge separate |
| W03-AL1 | Argon2 registration, unique email, expiring single-use verification, Mailpit | None locally |
| W03-AL2 | Login/current user, strict RS256 and verified accounts | None locally |
| W03-AL3 | JWT guide, cross-service positive/negative tests | Existing Go integration exercised |
| W04-AL1 | Four roles, protected profile/contact/demo payout placeholder | No public privileged-role assignment |
| W04-AL2 | Live Go/Python JWT and owner/role denial tests | Event staff assignment is Go-owned |
| W04-AL3 | Real integration smoke, error/session contract | Client/peer review remains external |
| W04-AL4 | Owner-authorized image API, validated static inputs, private durable local WebP storage | Clients must use returned image URL |
| W05-AL1 | Persisted per-session logout; reset invalidates all old sessions | None locally |
| W05-AL2 | Reusable SMTP adapter, durable encrypted outbox, leases/retry/failure records | Real SMTP provider outside local scope |
| W05-AL3 | Ticket/order/activation/notification schemas and ownership boundaries | Go implements planned business APIs |
| W06-AL1 | Idempotent authenticated order/ticket notification consumer | Go commits/publishes business outbox events; actual checkout absent |
| W06-AL2 | Internal organizer/profile/settings API; simulated activation fee/version | Go consumes authoritative fee and implements activation |
| W06-AL3 | Real Auth/Event/SMTP smoke, HTTPS and private secrets | Checkout smoke blocked on Go checkout |
| W07-AL1 | Fixed-origin ticket links, owner-scoped notification feed | Go must enforce actual ticket/PDF owner/staff access; client ticket view required |
| W07-AL2 | Retry visibility, request IDs across services, campaign security contract | Go signs/verifies campaign tokens and enforces budgets server-side |
| W07-AL3 | Support context/permissions/idempotency/polling contract | Web owners consume API; Go staff/context endpoints |
| W08-AL1 | Persistent cases/messages/categories/assignment/four states | None for platform support; contextual Go dependencies below |
| W08-AL2 | Requester/staff/admin scope, live Go authority, retained history | Order/ticket/staff Go endpoints pending; fixtures test fail-closed behavior |
| W08-AL3 | In-app/mail fanout, bounded cursors, retry-safe submissions | Web polling UI remains client-owned |
| W09-AL1 | Admin search/suspension/reactivation/settings/escalation/audit | None for Python API |
| W09-AL2 | Authenticated Go moderation adapters; shared fee store; live revocation | Go admin/event/order/payment/refund endpoints absent: adapters return unavailable |
| W09-AL3 | Support/admin/business templates and this traceability assessment | Go business producers and human feature-freeze decision |
| W10-AL1 | Clean Docker/release CI, CORS/token/revocation/security suite | Whole-team purchase/admission suite still pending |
| W10-AL2 | Secret isolation, database ACLs, hashing, persisted abuse budgets/body limits | Operator protects host/key backups |
| W10-AL3 | Ordered migrations, private DB/assets backup and disposable restore tools | Local evidence recorded separately |
| W11-AL1 | Pinned HTTPS image set, OCI source provenance, readiness and one-shot migrations | Local release candidate, not public hosting |
| W11-AL2 | Architecture/API/setup/operations/restore/compatible rollback runbook | Review/approval remains external |
| W11-AL3 | Operator clean-install/restore/rollback exercise and smoke tooling | Independent teammate rehearsal must still be performed |
| W12-AL1 | Exact-commit source archive, image/checksum manifest and release tag | Component candidate; no final whole-team release assertion |
| W12-AL2 | Documentation linked to actual schemas/scripts/build checks | Reviewer accepts documentation |
| W12-AL3 | Executable local demo and secret-free handover/runbook | Live team/instructor demonstration is human acceptance |

## Explicit Madi handoff

1. Merge/review the existing persistent Events PR separately; main currently uses
   an in-memory Event scaffold, so Go restarts lose those Events. Python retains
   its own data and image blobs. Read-only 32-hex Event-ID compatibility is temporary.
2. Implement the marked planned ticket types/orders/checkout/payment/tickets/PDF/
   activation/refund/payout routes. Return canonical UUID event/organizer/user/order
   links and authorize each caller; Python never writes their business tables.
3. Publish committed business actions to `/internal/notifications` using the opaque
   internal credential, stable sourceId and simulation=true for financial messages.
   Consumer uniqueness prevents duplicate jobs; producer outbox retry is Go-owned.
4. Implement `/api/events/{id}/staff/me` with current scoped permissions and Go admin
   search/event suspension. Missing/denied context fails closed; no mock permission
   or fabricated moderation report is returned in production.
5. Ticket delivery email contains a resource link, never an embedded admission QR or
   campaign credential. Go must check the owning attendee/authorized event staff on
   ticket/PDF retrieval. Campaign credentials must be purpose-separated from admission,
   signed, expiry/event/budget constrained, replay controlled and never accepted as
   client-supplied price authority. No campaign signer is claimed in Python.

API details: [Python platform](python-platform.md), [shared contract](openapi.yaml),
[JWT integration](python-go-auth.md), [architecture](architecture.md), [ERD](erd.md).
Operator delivery: [local HTTPS and recovery](local-release.md).
