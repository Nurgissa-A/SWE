# Identity and Event data model

Python migrations implement the four `identity_` tables below. Go's current `Event` model is in-memory; the Event, StaffAssignment, TicketType, Order and Ticket persistence relationships are **planned and Go-owned**. No Python migration creates them.

```mermaid
erDiagram
  identity_users ||--o| identity_organizer_profiles : owns
  identity_users ||--o{ identity_action_tokens : receives
  identity_action_tokens ||--o| identity_email_deliveries : delivery_receipt
  identity_users ||..o{ Event : "planned organizerId reference"
  Event ||..o{ TicketType : planned
  Event ||..o{ StaffAssignment : planned
  identity_users ||..o{ StaffAssignment : "planned event-specific permission"
  Event ||..o{ Order : planned
  Order ||..o{ Ticket : planned
  identity_users {
    uuid id PK
    varchar email UK "normalized lowercase"
    varchar password_hash "Argon2id only"
    varchar role "attendee organizer event_admin platform_admin"
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

A user has one coarse platform role as in the original API schema. Event Admin is not Platform Admin and receives no global event-management permission. Madi must implement scoped staff assignments for check-in later. Go stores immutable Python user UUIDs as references, never copies credentials or writes identity tables. Future Event migrations must select a compatible UUID representation: today's Go Event IDs are 32-character hexadecimal strings, a pre-existing mismatch with OpenAPI's UUID format that this identity PR does not silently rewrite.

Constraints: unique normalized email, one profile per user, restricted deletion of a user with a profile, indexed action-token owner/expiry, unique hashed token, valid role/purpose and nonnegative token version. Timestamps are timezone-aware UTC. Verification/reset tokens are row-locked and consumed in the same transaction as the user update, preventing concurrent reuse.
