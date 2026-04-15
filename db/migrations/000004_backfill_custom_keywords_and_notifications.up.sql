DO $$ BEGIN
  CREATE TYPE notification_type AS ENUM ('reaction_received');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE custom_keyword_cover_mode AS ENUM ('manual', 'auto_latest');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE custom_keyword_status AS ENUM ('active', 'inactive', 'deleted');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM pg_type t
    JOIN pg_enum e ON e.enumtypid = t.oid
    WHERE t.typname = 'custom_keyword_cover_mode'
      AND e.enumlabel = 'latest'
  ) AND NOT EXISTS (
    SELECT 1
    FROM pg_type t
    JOIN pg_enum e ON e.enumtypid = t.oid
    WHERE t.typname = 'custom_keyword_cover_mode'
      AND e.enumlabel = 'auto_latest'
  ) THEN
    ALTER TYPE custom_keyword_cover_mode RENAME VALUE 'latest' TO 'auto_latest';
  ELSIF EXISTS (
    SELECT 1
    FROM pg_type t
    WHERE t.typname = 'custom_keyword_cover_mode'
  ) AND NOT EXISTS (
    SELECT 1
    FROM pg_type t
    JOIN pg_enum e ON e.enumtypid = t.oid
    WHERE t.typname = 'custom_keyword_cover_mode'
      AND e.enumlabel = 'auto_latest'
  ) THEN
    ALTER TYPE custom_keyword_cover_mode ADD VALUE IF NOT EXISTS 'auto_latest';
  END IF;
END $$;

ALTER TYPE publish_context_type ADD VALUE IF NOT EXISTS 'custom_keyword';

ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS reaction_notification_enabled boolean;

UPDATE user_settings
SET reaction_notification_enabled = true
WHERE reaction_notification_enabled IS NULL;

ALTER TABLE user_settings
  ALTER COLUMN reaction_notification_enabled SET DEFAULT true;

ALTER TABLE user_settings
  ALTER COLUMN reaction_notification_enabled SET NOT NULL;

CREATE TABLE IF NOT EXISTS custom_keywords (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  text varchar(80) NOT NULL,
  target_image_count integer NOT NULL DEFAULT 0 CHECK (target_image_count >= 0),
  cover_asset_id uuid NULL REFERENCES media_assets(id) ON DELETE SET NULL,
  cover_mode custom_keyword_cover_mode NOT NULL DEFAULT 'auto_latest',
  status custom_keyword_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz NULL
);

ALTER TABLE custom_keywords
  ALTER COLUMN cover_mode SET DEFAULT 'auto_latest';

ALTER TABLE work_uploads
  ADD COLUMN IF NOT EXISTS custom_keyword_id uuid NULL REFERENCES custom_keywords(id);

ALTER TABLE publish_sessions
  ADD COLUMN IF NOT EXISTS custom_keyword_id uuid NULL REFERENCES custom_keywords(id);

ALTER TABLE work_uploads
  DROP CONSTRAINT IF EXISTS chk_work_uploads_context;

ALTER TABLE work_uploads
  ADD CONSTRAINT chk_work_uploads_context CHECK (
    (context_type = 'official_today' AND official_keyword_id IS NOT NULL AND biz_date IS NOT NULL AND custom_keyword_id IS NULL) OR
    (context_type = 'custom_keyword' AND custom_keyword_id IS NOT NULL AND official_keyword_id IS NULL)
  );

ALTER TABLE publish_sessions
  DROP CONSTRAINT IF EXISTS chk_publish_sessions_context;

ALTER TABLE publish_sessions
  ADD CONSTRAINT chk_publish_sessions_context CHECK (
    (context_type = 'official_today' AND official_keyword_id IS NOT NULL AND biz_date IS NOT NULL AND custom_keyword_id IS NULL) OR
    (context_type = 'custom_keyword' AND custom_keyword_id IS NOT NULL AND official_keyword_id IS NULL)
  );

CREATE TABLE IF NOT EXISTS notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  recipient_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_nickname_snapshot varchar(40) NOT NULL,
  actor_avatar_variant_key_snapshot text NULL,
  target_upload_id uuid NOT NULL REFERENCES work_uploads(id) ON DELETE CASCADE,
  target_upload_cover_variant_key_snapshot text NULL,
  type notification_type NOT NULL,
  reaction_type reaction_type NULL,
  read_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_trigger
    WHERE tgname = 'trg_custom_keywords_updated_at'
  ) THEN
    CREATE TRIGGER trg_custom_keywords_updated_at
    BEFORE UPDATE ON custom_keywords
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_custom_keywords_owner_updated_at
  ON custom_keywords (owner_user_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_recipient_created
  ON notifications (recipient_user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_notifications_recipient_read_created
  ON notifications (recipient_user_id, read_at, created_at DESC);
