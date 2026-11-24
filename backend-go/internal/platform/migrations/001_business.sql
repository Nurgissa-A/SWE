-- Go-owned business schema (SRS §6 core data entities).
-- Users live in the Python identity service; this schema stores their UUIDs as
-- plain references (no foreign keys into identity_* tables, see docs/architecture.md ADR 002).
-- Money is integer KZT (tenge). Timestamps are timestamptz (UTC).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Last-seen snapshot of identity data, refreshed from Python /api/users/me on every
-- authenticated Go request. Used for names/emails on notifications and staff lists.
CREATE TABLE user_directory (
  user_id      uuid PRIMARY KEY,
  email        text NOT NULL,
  display_name text,
  locale       text NOT NULL DEFAULT 'ru' CHECK (locale IN ('kk','ru','en')),
  role         text NOT NULL,
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_directory_email_idx ON user_directory(lower(email));

-- Simulated organizer payment setup (SRS §3.2, §4.5): identity check and payout account.
-- The 12-digit IIN is checked for format only and never stored; the IBAN is stored masked.
CREATE TABLE organizer_payment_setups (
  organizer_id         uuid PRIMARY KEY,
  legal_name           text,
  identity_verified_at timestamptz,
  bank_name            text,
  account_holder       text,
  iban_masked          text,
  payout_verified_at   timestamptz,
  is_simulated         boolean NOT NULL DEFAULT true,
  updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE events (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organizer_id           uuid NOT NULL,
  title                  text NOT NULL,
  description            text,
  category               text,
  image_url              text,
  venue_id               uuid,
  venue_name             text,
  venue_address          text,
  city                   text,
  starts_at              timestamptz NOT NULL,
  ends_at                timestamptz,
  timezone               text NOT NULL DEFAULT 'Asia/Almaty',
  visibility             text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','unlisted','private')),
  access_token           text NOT NULL DEFAULT encode(gen_random_bytes(12),'hex'),
  status                 text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','cancelled')),
  capacity               int CHECK (capacity IS NULL OR capacity > 0),
  registration_opens_at  timestamptz,
  registration_closes_at timestamptz,
  refund_policy          text NOT NULL DEFAULT 'No refunds unless the event is cancelled.',
  paid_sales_status      text NOT NULL DEFAULT 'inactive' CHECK (paid_sales_status IN ('inactive','active','suspended')),
  suspended_at           timestamptz,
  suspension_reason      text,
  duplicated_from        uuid REFERENCES events(id),
  published_at           timestamptz,
  cancelled_at           timestamptz,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at IS NULL OR ends_at > starts_at)
);
CREATE INDEX events_organizer_idx ON events(organizer_id);
CREATE INDEX events_public_idx ON events(status, visibility, starts_at);

CREATE TABLE ticket_types (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id       uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  name           text NOT NULL,
  description    text NOT NULL DEFAULT '',
  price_kzt      int NOT NULL DEFAULT 0 CHECK (price_kzt >= 0),
  quantity       int NOT NULL CHECK (quantity >= 0),
  sales_start_at timestamptz,
  sales_end_at   timestamptz,
  max_per_order  int NOT NULL DEFAULT 10 CHECK (max_per_order > 0),
  hidden         boolean NOT NULL DEFAULT false,
  sort_order     int NOT NULL DEFAULT 0,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ticket_types_event_idx ON ticket_types(event_id);

CREATE TABLE event_activations (
  event_id          uuid PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
  fee_kzt           int,
  fee_payment_id    uuid,
  fee_paid_at       timestamptz,
  terms_accepted_at timestamptz,
  activated_at      timestamptz,
  activated_by      uuid
);

-- Event Admin is an event-scoped permission (SRS §2.3), not a platform role.
CREATE TABLE staff_assignments (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id   uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL,
  can_undo   boolean NOT NULL DEFAULT true,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (event_id, user_id)
);
CREATE INDEX staff_assignments_user_idx ON staff_assignments(user_id);

CREATE TABLE promo_campaigns (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id          uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  name              text NOT NULL,
  code              text NOT NULL,
  link_token        text NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(12),'hex'),
  discount_type     text NOT NULL CHECK (discount_type IN ('percent','fixed')),
  discount_value    int NOT NULL CHECK (discount_value > 0),
  valid_from        timestamptz,
  valid_until       timestamptz,
  max_redemptions   int CHECK (max_redemptions IS NULL OR max_redemptions > 0),
  redemptions_count int NOT NULL DEFAULT 0,
  ticket_type_ids   uuid[],
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  link_visits       int NOT NULL DEFAULT 0,
  created_by        uuid NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (event_id, code),
  CHECK (discount_type <> 'percent' OR discount_value <= 100),
  CHECK (max_redemptions IS NULL OR redemptions_count <= max_redemptions),
  CHECK (redemptions_count >= 0)
);

CREATE SEQUENCE order_number_seq START 100001;
CREATE TABLE orders (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  number            text NOT NULL UNIQUE DEFAULT ('BF-' || nextval('order_number_seq')),
  event_id          uuid NOT NULL REFERENCES events(id),
  user_id           uuid NOT NULL,
  status            text NOT NULL CHECK (status IN ('pending','completed','failed','expired','cancelled','refunded')),
  buyer_name        text NOT NULL,
  buyer_email       text NOT NULL,
  buyer_phone       text,
  subtotal_kzt      int NOT NULL,
  discount_kzt      int NOT NULL DEFAULT 0,
  total_kzt         int NOT NULL,
  fee_kzt           int NOT NULL DEFAULT 0,
  promo_campaign_id uuid REFERENCES promo_campaigns(id),
  attendee_details  jsonb NOT NULL DEFAULT '[]',
  expires_at        timestamptz,
  paid_at           timestamptz,
  cancelled_at      timestamptz,
  refunded_at       timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  CHECK (total_kzt >= 0 AND discount_kzt >= 0)
);
CREATE INDEX orders_event_idx ON orders(event_id, status);
CREATE INDEX orders_user_idx ON orders(user_id);

CREATE TABLE order_items (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id       uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  ticket_type_id uuid NOT NULL REFERENCES ticket_types(id),
  quantity       int NOT NULL CHECK (quantity > 0),
  unit_price_kzt int NOT NULL,
  discount_kzt   int NOT NULL DEFAULT 0,
  -- Reserved for assigned seating (bonus feature, SRS §13.4); unused in the MVP.
  seat_section   text, seat_row text, seat_number text
);

CREATE TABLE tickets (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code           text NOT NULL UNIQUE,
  order_id       uuid NOT NULL REFERENCES orders(id),
  order_item_id  uuid NOT NULL REFERENCES order_items(id),
  event_id       uuid NOT NULL REFERENCES events(id),
  ticket_type_id uuid NOT NULL REFERENCES ticket_types(id),
  user_id        uuid NOT NULL,
  attendee_name  text NOT NULL,
  attendee_email text NOT NULL,
  status         text NOT NULL DEFAULT 'valid' CHECK (status IN ('valid','checked_in','cancelled','refunded')),
  issued_at      timestamptz NOT NULL DEFAULT clock_timestamp(), -- per-row time keeps attendee order stable
  checked_in_at  timestamptz,
  checked_in_by  uuid
);
CREATE INDEX tickets_event_idx ON tickets(event_id, status);
CREATE INDEX tickets_user_idx ON tickets(user_id);

CREATE TABLE payments (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind           text NOT NULL CHECK (kind IN ('ticket','activation_fee')),
  order_id       uuid REFERENCES orders(id),
  event_id       uuid NOT NULL REFERENCES events(id),
  payer_id       uuid NOT NULL,
  amount_kzt     int NOT NULL,
  fee_kzt        int NOT NULL DEFAULT 0,
  status         text NOT NULL CHECK (status IN ('succeeded','failed')),
  failure_reason text,
  provider       text NOT NULL DEFAULT 'biletflow-simulator',
  provider_ref   text NOT NULL,
  card_last4     text,
  is_simulated   boolean NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refunds (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id     uuid NOT NULL REFERENCES orders(id),
  amount_kzt   int NOT NULL,
  reason       text NOT NULL,
  initiated_by uuid NOT NULL,
  is_simulated boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE promo_redemptions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id  uuid NOT NULL REFERENCES promo_campaigns(id),
  order_id     uuid NOT NULL UNIQUE REFERENCES orders(id),
  discount_kzt int NOT NULL,
  status       text NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved','confirmed','released')),
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE check_in_records (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id  uuid NOT NULL REFERENCES tickets(id),
  event_id   uuid NOT NULL REFERENCES events(id),
  staff_id   uuid NOT NULL,
  action     text NOT NULL CHECK (action IN ('check_in','undo')),
  method     text NOT NULL DEFAULT 'qr' CHECK (method IN ('qr','manual')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE SEQUENCE support_case_seq START 1001;
CREATE TABLE support_cases (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  number       int NOT NULL UNIQUE DEFAULT nextval('support_case_seq'),
  kind         text NOT NULL CHECK (kind IN ('attendee','organizer')),
  requester_id uuid NOT NULL,
  category     text NOT NULL CHECK (category IN ('ticket_delivery','payment','refund','event_information','check_in','account','activation','technical')),
  subject      text NOT NULL,
  status       text NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','waiting_for_customer','resolved')),
  event_id     uuid REFERENCES events(id),
  order_id     uuid REFERENCES orders(id),
  ticket_id    uuid REFERENCES tickets(id),
  assigned_to  uuid,
  escalated    boolean NOT NULL DEFAULT false,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  resolved_at  timestamptz,
  CHECK (kind = 'organizer' OR event_id IS NOT NULL)
);

CREATE TABLE support_messages (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id    uuid NOT NULL REFERENCES support_cases(id) ON DELETE CASCADE,
  author_id  uuid NOT NULL,
  body       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Notifications store a type plus parameters; clients and emails render them in the
-- recipient's language (Kazakh, Russian or English).
CREATE TABLE notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL,
  type       text NOT NULL,
  params     jsonb NOT NULL DEFAULT '{}',
  link       text,
  read_at    timestamptz,
  -- Transactional outbox: the row commits with the business change, and a worker sends
  -- the e-mail afterwards, so a rolled-back change never e-mails anyone.
  email_wanted   boolean NOT NULL DEFAULT false,
  emailed_at     timestamptz,
  email_attempts int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications(user_id, created_at DESC);
CREATE INDEX notifications_outbox_idx ON notifications(created_at) WHERE email_wanted AND emailed_at IS NULL;

CREATE TABLE event_reports (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id    uuid NOT NULL REFERENCES events(id),
  reporter_id uuid NOT NULL,
  reason      text NOT NULL,
  status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open','actioned','dismissed')),
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE platform_settings (
  key   text PRIMARY KEY,
  value jsonb NOT NULL
);
INSERT INTO platform_settings(key, value) VALUES
  ('activation_fee_kzt', '5000'),
  ('processing_fee_percent', '2.5'),
  ('hold_minutes', '10');

-- event_id has no foreign key on purpose: history must outlive a deleted draft.
CREATE TABLE audit_logs (
  id          bigserial PRIMARY KEY,
  event_id    uuid,
  actor_id    uuid,
  action      text NOT NULL,
  entity_type text NOT NULL,
  entity_id   text,
  description text NOT NULL,
  metadata    jsonb NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_event_idx ON audit_logs(event_id, created_at DESC);

-- Audit entries can never be edited or deleted, even by application bugs (SRS §4.9, §4.16).
CREATE FUNCTION audit_logs_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'audit_logs is append-only';
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER audit_logs_no_update BEFORE UPDATE OR DELETE ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION audit_logs_immutable();

-- Live inventory per ticket type. Reserved = unexpired checkout holds; sold = issued,
-- still-valid tickets (refunded or cancelled tickets free their place again).
CREATE VIEW ticket_type_stock AS
SELECT tt.id AS ticket_type_id, tt.event_id, tt.quantity,
  (SELECT coalesce(sum(oi.quantity),0) FROM order_items oi JOIN orders o ON o.id = oi.order_id
     WHERE oi.ticket_type_id = tt.id AND o.status = 'pending' AND o.expires_at > now())::int AS reserved,
  (SELECT count(*) FROM tickets t WHERE t.ticket_type_id = tt.id AND t.status IN ('valid','checked_in'))::int AS sold,
  (SELECT count(*) FROM tickets t WHERE t.ticket_type_id = tt.id AND t.status = 'refunded')::int AS refunded,
  (SELECT count(*) FROM tickets t WHERE t.ticket_type_id = tt.id AND t.status = 'checked_in')::int AS checked_in
FROM ticket_types tt;
