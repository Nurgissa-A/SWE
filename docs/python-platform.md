# Aliar platform API and delivery implementation

Python owns identity, session revocation, asynchronous support, notification
transport, platform settings and administration. Go owns Event, Order, Ticket,
payments/refunds, campaigns, staff assignments, admission and business reporting.
These responsibilities are not duplicated by Python tables or services.

## Account and delivery behavior

`POST /api/auth/logout` revokes the current JWT jti, returning 204. Other sessions
remain valid. A repeat request with the revoked bearer returns 401; clients clear
local credentials in either case. Reset and operator/suspension changes still
invalidate every old session through token_version. Go independently verifies
RS256 and asks Python `/api/users/me`, so logout/suspension applies immediately
there too. Revocations survive process restart; expired rows are purged by worker.

Registration returns 201 once the user, action token and encrypted delivery job
commit. SMTP failure no longer rolls back a valid account or loses its only email
link. Reset/resend requests return generic 200 and queue delivery. Public
`POST /api/auth/resend-verification` recovers an unverified account without a
privileged role assignment or disclosing unknown addresses. Direct SMTP is not
part of the registration transaction. Tests may explicitly enable eager delivery.

`python -m app.worker` claims due jobs with `FOR UPDATE SKIP LOCKED`, persists an
attempt and a 60-second lease, sends outside the DB transaction, then records its
result. Five attempts use bounded exponential delay; crashed leases are reclaimed.
Admin can inspect/retry pending/failed jobs. Used/expired action links and inactive
recipients are skipped. Encryption failures are visible without exposing payloads.
The separate Fernet key is outside Git; back it up separately from the database.

Enqueue uniqueness is source + event UUID + recipient + template, with a canonical
payload digest. Same action/data replays one job; conflicting data returns 409.
SMTP is at-least-once: a crash after accepted SMTP but before acknowledgement can
repeat an email, using the same Message-ID. No worker code creates or reruns a
Go order, payment, ticket or refund. Exactly-once SMTP delivery is not claimed.

`POST /internal/notifications` accepts only the opaque server-to-server token.
Go must commit its business action/outbox before publishing this envelope and
retry with the same sourceId. Supported templates cover order confirmation,
ticket delivery, payment failure, event update/cancellation, refund completion
and payout status. Financial messages are explicitly simulation-only. Link
construction uses canonical IDs on the configured frontend origin; no untrusted
URL, discount amount, QR token or arbitrary email body is accepted from clients.
The internal API is blocked at the public HTTPS gateway.

## Support API

- POST/GET `/api/support/cases`; GET/PATCH `/api/support/cases/{case_id}`.
- GET/POST `/api/support/cases/{case_id}/messages`.
- GET `/api/support/cases/{case_id}/history` with a bounded chronological cursor.
- GET `/api/notifications?after=0&limit=50`; PATCH notification `/{id}/read`.

Create/reply/change require a UUID `Idempotency-Key`; exact retries return the
original resource and conflicting action/payload reuse is 409. Messages have
monotonic per-case sequence cursors. Retained audit/messages have no edit/delete
API. States are open, in_progress, waiting_for_customer and resolved; requester
replies can reopen a resolved case. Categories follow the SRS.

Attendees require authorized Event/Order/Ticket context; organizers can open
platform-support cases. Python validates linked resources through Go with the
requester's verified bearer and bounded HTTP calls. Event/order/ticket/organizer
IDs must agree; attendee ownership is checked. Go may return organizerId on an
authorized order/ticket to support private-event cases without publishing the
Event page. Missing authorization or upstream errors fail closed, never fabricate
ownership. A read-only 32-hex Event-ID fallback supports the historical main Go
scaffold while Madi PR #6's UUID persistence change is pending.

Requester can see their own retained thread. Event support staff need current
Go permission for that exact event; Event Admin alone grants no global access.
Organizer ownership is checked through Go. Platform cases and escalations are
limited to requester and Platform Admin. Authorized staff can self-assign,
assign an appropriate owner/admin, set status and escalate. Case actions generate
idempotent in-app/email notifications without including private message bodies
in mail. Unrelated users receive 404, including for messages/history.

## Platform administration

Platform Admin can search users with bounded pagination, suspend/reactivate
accounts, change the one authoritative KZT minor-unit activation fee using
optimistic version checks, inspect immutable audit entries and manage delivery
retries. Self-suspension is blocked. Suspension increments token_version; old
JWTs remain invalid after reactivation. Operator role/enable/disable CLI actions
are retained with actorKind=operator and no fabricated user ID.

`/internal/platform-settings` and `/internal/organizer-profiles/{user_id}` supply
fee/version and verified simulation-profile data to an authenticated Go service.
The profile exposes only contact and demo: payout placeholder values. Fee default
is zero until a Platform Admin chooses a simulation policy; no invented real
activation charge is presented as product policy.

`/api/admin/business/{resource}` and event suspension forward the live caller's
bearer to the corresponding Go admin API. Until those Go APIs exist, these
adapters return 503; they do not return placeholder reports or write Go data.
Go owns authoritative business audit/reporting. Python support case/history APIs
can be read with the authorized caller's JWT for future timeline aggregation.

## Security and operation

Persisted keyed auth budgets bound login/register/reset/resend by endpoint,
normalized email digest and client IP digest; no raw email/IP is stored in limiter
keys. Exhaustion returns 429 and Retry-After. All bodies are bounded; CORS includes
Idempotency-Key and exposes request IDs. Python/Go normalize and propagate
request IDs without logging tokens, bodies, addresses or query strings.

Runtime DB roles are separate from migrations. Identity writes only Python-owned
tables; notifier reads only delivery-relevant user/token columns and cannot read
password hashes; Go cannot read identity credentials. Audit/message tables are
append-only for runtime identity credentials. Bootstrap/grant tasks run in order,
and application roles cannot create public-schema objects. See release runbook
for key preservation, migrations, health checks and backup/restore procedures.

## Real integration dependencies

Go Orders/Tickets/PDF, staff-me assignment, business admin/search/suspension,
checkout/payment/refund, campaigns and analytics are not implemented on current
main. Their shared OpenAPI operations are explicitly planned. Context fixture
tests verify Python's adapter/denial behavior, not completed Go business features.
Real HTTPS smoke verifies actual Go Event context and ownership, SMTP/support,
settings/suspension/audit and logout. Full purchase-to-ticket/PDF/check-in and
whole-team final MVP acceptance remain dependent on Madi and client/QA owners.

## Event images

PUT `/api/event-images/{event_id}` accepts raw PNG/JPEG/static WebP from the current
organizer owner verified through Go. Maximum input/output is 1 MiB, each side 4096
pixels and total 8 MP. Decoding rejects MIME mismatch, animation and malformed data;
normalization strips metadata into RGB WebP. Immutable content-addressed blobs live
outside Git with a 50 MiB local quota; the database stores only an audited Event
pointer. Repeating identical bytes preserves the pointer/update timestamp. GET
checks current Go Event visibility, rejects missing/symlink blobs and sends no-store
WebP with nosniff. Back up the database and image storage together; see recovery runbook.
