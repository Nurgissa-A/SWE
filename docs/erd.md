# Identity and Event data model

Python migrations implement the four `identity_` tables below. Go's migrations (`backend-go/internal/platform/migrations`) implement the business tables: events, ticket types, staff assignments, activations, orders, order items, tickets, payments, refunds, promo campaigns and redemptions, check-in records, support cases and messages, notifications, reports, settings and the append-only audit log. Go references identity users by UUID without foreign keys into `identity_` tables; the dotted links below are those references.

```mermaid
erDiagram
  identity_users ||--o| identity_organizer_profiles : owns
  identity_users ||--o{ identity_action_tokens : receives
  identity_action_tokens ||--o| identity_email_deliveries : delivery_receipt
  identity_users ||..o{ Event : "organizerId reference"
  Event ||--o{ TicketType : has
  Event ||--o{ StaffAssignment : has
  identity_users ||..o{ StaffAssignment : "event-specific permission"
  Event ||--o{ Order : has
  Order ||--o{ Ticket : issues
  Event ||--o{ PromoCampaign : runs
  Order ||--o{ Payment : "simulated"
  Ticket ||--o{ CheckInRecord : scanned
  Event ||--o{ SupportCase : about
  identity_users ||..o{ Order : "buyer reference"
  identity_users {
    uuid id PK
    varchar email UK "normalized lowercase"
    varchar password_hash "Argon2id only"
    varchar role "attendee organizer event_admin platform_admin"
    varchar locale "kk ru en"
    boolean email_verified
    boolean is_active
    integer token_version
    timestamptz created_at
  }
  identity_organizer_profiles {
    uuid id PK
    uuid user_id FK,UK
    varchar contact_email
    varchar contact_phone
    varchar payout_account_placeholder "demo labels only"
  }
  identity_action_tokens {
    uuid id PK
    uuid user_id FK
    varchar token_hash UK "SHA256 of 256-bit random secret"
    varchar purpose
    timestamptz expires_at
    timestamptz used_at
  }
  identity_email_deliveries {
    uuid id PK
    uuid action_token_id FK,UK
    timestamptz sent_at
    integer attempts
  }
```

A user has one coarse platform role as in the original API schema. Event Admin is not Platform Admin and receives no global event-management permission. Scoped staff assignments (`staff_assignments`) grant check-in for one event only. Go stores immutable Python user UUIDs as references, never copies credentials or writes identity tables. Go Event IDs are now PostgreSQL UUIDs, matching the OpenAPI format.

Constraints: unique normalized email, one profile per user, restricted deletion of a user with a profile, indexed action-token owner/expiry, unique hashed token, valid role/purpose and nonnegative token version. Timestamps are timezone-aware UTC. Verification/reset tokens are row-locked and consumed in the same transaction as the user update, preventing concurrent reuse.
