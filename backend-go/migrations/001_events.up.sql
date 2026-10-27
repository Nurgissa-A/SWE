CREATE TABLE events.events (
 id uuid PRIMARY KEY,
 organizer_id uuid NOT NULL REFERENCES public.identity_organizer_profiles(user_id) ON DELETE RESTRICT,
 title text NOT NULL CHECK (length(btrim(title)) > 0),
 description text,
 category text,
 starts_at timestamptz NOT NULL,
 ends_at timestamptz,
 venue_id uuid CHECK (venue_id <> '00000000-0000-0000-0000-000000000000'::uuid),
 visibility text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','unlisted','private')),
 capacity integer CHECK (capacity > 0),
 status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','cancelled')),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK (ends_at IS NULL OR ends_at > starts_at),
 CHECK (status <> 'published' OR venue_id IS NOT NULL),
 CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid)
);
CREATE INDEX events_owner_status_created_idx ON events.events (organizer_id,status,created_at DESC,id);
CREATE INDEX events_public_created_idx ON events.events (created_at DESC,id) WHERE status='published' AND visibility='public';
