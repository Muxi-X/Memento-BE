CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

DO $$ BEGIN
  CREATE TYPE keyword_category AS ENUM ('emotion', 'color', 'shape', 'time', 'abstract');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE prompt_kind AS ENUM ('intuition', 'structure', 'concept');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE reaction_type AS ENUM ('inspired', 'resonated');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE notification_type AS ENUM ('reaction_received');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE media_kind AS ENUM ('image', 'audio');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE media_asset_status AS ENUM ('pending_upload', 'uploaded', 'processing', 'ready', 'failed', 'deleted');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE media_variant_status AS ENUM ('pending', 'processing', 'ready', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE publish_context_type AS ENUM ('official_today', 'custom_keyword');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE publish_session_status AS ENUM ('created', 'committed', 'canceled', 'expired');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE publish_session_item_status AS ENUM ('pending_upload', 'uploaded');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE work_upload_visibility_status AS ENUM ('processing', 'visible', 'hidden', 'deleted');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE avatar_upload_session_status AS ENUM ('created', 'presigned', 'uploaded', 'committing', 'completed', 'failed', 'canceled', 'expired');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE custom_keyword_cover_mode AS ENUM ('manual', 'auto_latest');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE custom_keyword_status AS ENUM ('active', 'inactive', 'deleted');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE job_status AS ENUM ('queued', 'running', 'finished', 'dead');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  status smallint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_email_identities (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  email citext NOT NULL UNIQUE,
  email_verified boolean NOT NULL DEFAULT false,
  password_hash text NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE email_action_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  purpose smallint NOT NULL CHECK (purpose IN (1, 2)),
  email citext NOT NULL,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE official_keywords (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  text varchar(50) NOT NULL,
  category keyword_category NOT NULL,
  is_active boolean NOT NULL DEFAULT true,
  display_order integer NOT NULL DEFAULT 0 CHECK (display_order >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_official_keywords_text UNIQUE (text)
);

CREATE TABLE official_keyword_prompts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  keyword_id uuid NOT NULL REFERENCES official_keywords(id) ON DELETE CASCADE,
  kind prompt_kind NOT NULL,
  content varchar(200) NOT NULL,
  display_order integer NOT NULL DEFAULT 0 CHECK (display_order >= 0),
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_official_keyword_prompts_unique UNIQUE (keyword_id, kind, content)
);

CREATE TABLE daily_keyword_assignments (
  biz_date date PRIMARY KEY,
  keyword_id uuid NOT NULL REFERENCES official_keywords(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE daily_keyword_stats (
  biz_date date PRIMARY KEY,
  participant_user_count integer NOT NULL DEFAULT 0 CHECK (participant_user_count >= 0),
  upload_count integer NOT NULL DEFAULT 0 CHECK (upload_count >= 0),
  image_count integer NOT NULL DEFAULT 0 CHECK (image_count >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE media_assets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  media_kind media_kind NOT NULL,
  mime_type text NOT NULL,
  original_object_key text NOT NULL UNIQUE,
  byte_size bigint NOT NULL CHECK (byte_size > 0),
  width integer NULL CHECK (width IS NULL OR width > 0),
  height integer NULL CHECK (height IS NULL OR height > 0),
  duration_ms integer NULL CHECK (duration_ms IS NULL OR duration_ms >= 0),
  status media_asset_status NOT NULL DEFAULT 'pending_upload',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz NULL
);

CREATE TABLE media_variants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id uuid NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
  variant_name text NOT NULL,
  object_key text NOT NULL UNIQUE,
  width integer NULL CHECK (width IS NULL OR width > 0),
  height integer NULL CHECK (height IS NULL OR height > 0),
  status media_variant_status NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_media_variants_asset_variant UNIQUE (asset_id, variant_name)
);

CREATE TABLE user_profiles (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  nickname varchar(40) NOT NULL DEFAULT 'User',
  bio varchar(500) NOT NULL DEFAULT '',
  current_avatar_asset_id uuid NULL REFERENCES media_assets(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT chk_user_profiles_nickname_not_blank CHECK (char_length(btrim(nickname)) > 0)
);

CREATE TABLE user_settings (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  public_pool_enabled boolean NOT NULL DEFAULT true,
  privacy_version integer NOT NULL DEFAULT 0 CHECK (privacy_version >= 0),
  privacy_updated_at timestamptz NOT NULL DEFAULT now(),
  reaction_notification_enabled boolean NOT NULL DEFAULT true,
  creation_reminder_enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE avatar_upload_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  image_asset_id uuid NULL REFERENCES media_assets(id) ON DELETE SET NULL,
  status avatar_upload_session_status NOT NULL DEFAULT 'created',
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE custom_keywords (
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

CREATE TABLE work_uploads (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  author_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  context_type publish_context_type NOT NULL,
  official_keyword_id uuid NULL REFERENCES official_keywords(id),
  custom_keyword_id uuid NULL REFERENCES custom_keywords(id),
  biz_date date NULL,
  visibility_status work_upload_visibility_status NOT NULL DEFAULT 'processing',
  cover_asset_id uuid NOT NULL REFERENCES media_assets(id),
  image_count integer NOT NULL DEFAULT 0 CHECK (image_count >= 0),
  reaction_inspired_count integer NOT NULL DEFAULT 0 CHECK (reaction_inspired_count >= 0),
  reaction_resonated_count integer NOT NULL DEFAULT 0 CHECK (reaction_resonated_count >= 0),
  rand_key double precision NOT NULL DEFAULT random(),
  published_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz NULL,
  CONSTRAINT chk_work_uploads_context CHECK (
    (context_type = 'official_today' AND official_keyword_id IS NOT NULL AND biz_date IS NOT NULL AND custom_keyword_id IS NULL) OR
    (context_type = 'custom_keyword' AND custom_keyword_id IS NOT NULL AND official_keyword_id IS NULL)
  )
);

CREATE TABLE publish_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  context_type publish_context_type NOT NULL,
  official_keyword_id uuid NULL REFERENCES official_keywords(id),
  custom_keyword_id uuid NULL REFERENCES custom_keywords(id),
  biz_date date NULL,
  status publish_session_status NOT NULL DEFAULT 'created',
  expires_at timestamptz NOT NULL,
  published_upload_id uuid NULL REFERENCES work_uploads(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT chk_publish_sessions_context CHECK (
    (context_type = 'official_today' AND official_keyword_id IS NOT NULL AND biz_date IS NOT NULL AND custom_keyword_id IS NULL) OR
    (context_type = 'custom_keyword' AND custom_keyword_id IS NOT NULL AND official_keyword_id IS NULL)
  )
);

CREATE TABLE publish_session_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id uuid NOT NULL REFERENCES publish_sessions(id) ON DELETE CASCADE,
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_image_id varchar(100) NOT NULL,
  image_asset_id uuid NOT NULL REFERENCES media_assets(id),
  audio_asset_id uuid NULL REFERENCES media_assets(id),
  display_order integer NULL CHECK (display_order IS NULL OR display_order > 0),
  is_cover boolean NOT NULL DEFAULT false,
  title varchar(80) NULL,
  note varchar(500) NULL,
  status publish_session_item_status NOT NULL DEFAULT 'pending_upload',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_publish_session_items_client_image UNIQUE (session_id, client_image_id)
);

CREATE TABLE work_upload_images (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  upload_id uuid NOT NULL REFERENCES work_uploads(id) ON DELETE CASCADE,
  image_asset_id uuid NOT NULL REFERENCES media_assets(id),
  display_order integer NOT NULL CHECK (display_order > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz NULL,
  CONSTRAINT uq_work_upload_images_upload_asset UNIQUE (upload_id, image_asset_id)
);

CREATE TABLE work_upload_image_contents (
  work_upload_image_id uuid PRIMARY KEY REFERENCES work_upload_images(id) ON DELETE CASCADE,
  title varchar(80) NULL,
  note varchar(500) NULL,
  audio_asset_id uuid NULL REFERENCES media_assets(id),
  audio_duration_ms integer NULL CHECK (audio_duration_ms IS NULL OR audio_duration_ms >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE work_upload_reactions (
  upload_id uuid NOT NULL REFERENCES work_uploads(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type reaction_type NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (upload_id, user_id, type)
);

CREATE TABLE notifications (
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

CREATE TABLE jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_type text NOT NULL,
  dedupe_key text NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  status job_status NOT NULL DEFAULT 'queued',
  max_attempts integer NOT NULL DEFAULT 10 CHECK (max_attempts > 0),
  available_at timestamptz NOT NULL DEFAULT now(),
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  last_error text NULL,
  locked_at timestamptz NULL,
  finished_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_user_email_identities_updated_at BEFORE UPDATE ON user_email_identities FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_official_keywords_updated_at BEFORE UPDATE ON official_keywords FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_official_keyword_prompts_updated_at BEFORE UPDATE ON official_keyword_prompts FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_daily_keyword_assignments_updated_at BEFORE UPDATE ON daily_keyword_assignments FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_daily_keyword_stats_updated_at BEFORE UPDATE ON daily_keyword_stats FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_media_assets_updated_at BEFORE UPDATE ON media_assets FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_media_variants_updated_at BEFORE UPDATE ON media_variants FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_user_profiles_updated_at BEFORE UPDATE ON user_profiles FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_user_settings_updated_at BEFORE UPDATE ON user_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_avatar_upload_sessions_updated_at BEFORE UPDATE ON avatar_upload_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_custom_keywords_updated_at BEFORE UPDATE ON custom_keywords FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_work_uploads_updated_at BEFORE UPDATE ON work_uploads FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_publish_sessions_updated_at BEFORE UPDATE ON publish_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_publish_session_items_updated_at BEFORE UPDATE ON publish_session_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_work_upload_images_updated_at BEFORE UPDATE ON work_upload_images FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_work_upload_image_contents_updated_at BEFORE UPDATE ON work_upload_image_contents FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_jobs_updated_at BEFORE UPDATE ON jobs FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_refresh_tokens_user_expires_at ON refresh_tokens (user_id, expires_at DESC);
CREATE INDEX idx_email_action_sessions_email_purpose ON email_action_sessions (email, purpose, created_at DESC);
CREATE INDEX idx_official_keywords_display_order ON official_keywords (display_order ASC, id ASC);
CREATE INDEX idx_official_keyword_prompts_keyword_kind_active ON official_keyword_prompts (keyword_id, kind, is_active, display_order ASC, id ASC);
CREATE INDEX idx_daily_keyword_assignments_keyword_id ON daily_keyword_assignments (keyword_id);
CREATE INDEX idx_media_assets_owner_created_at ON media_assets (owner_user_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_media_variants_variant_status_updated ON media_variants (variant_name, status, updated_at DESC);
CREATE INDEX idx_avatar_upload_sessions_user_created_at ON avatar_upload_sessions (user_id, created_at DESC, id DESC);
CREATE INDEX idx_custom_keywords_owner_updated_at ON custom_keywords (owner_user_id, updated_at DESC, id DESC) WHERE deleted_at IS NULL;

CREATE INDEX idx_work_uploads_visibility_biz_date_published
  ON work_uploads (visibility_status, biz_date, published_at DESC, id DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_work_uploads_official_keyword_visibility_published
  ON work_uploads (official_keyword_id, visibility_status, published_at DESC, id DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_work_uploads_author_published
  ON work_uploads (author_user_id, published_at DESC, id DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_work_uploads_visibility_rand
  ON work_uploads (visibility_status, rand_key, id)
  WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uq_publish_session_items_session_display_order
  ON publish_session_items (session_id, display_order)
  WHERE display_order IS NOT NULL;

CREATE INDEX idx_publish_session_items_session_created
  ON publish_session_items (session_id, created_at ASC, id ASC);

CREATE UNIQUE INDEX uq_work_upload_images_upload_display_order
  ON work_upload_images (upload_id, display_order)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_work_upload_images_upload_display_order
  ON work_upload_images (upload_id, display_order ASC, id ASC)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_notifications_recipient_created
  ON notifications (recipient_user_id, created_at DESC, id DESC);

CREATE INDEX idx_notifications_recipient_read_created
  ON notifications (recipient_user_id, read_at, created_at DESC);

CREATE INDEX idx_work_upload_reactions_user_created
  ON work_upload_reactions (user_id, created_at DESC);

CREATE INDEX idx_jobs_status_available_at
  ON jobs (status, available_at, id);

CREATE INDEX idx_jobs_status_locked_at
  ON jobs (status, locked_at);

CREATE UNIQUE INDEX uq_jobs_active_dedupe
  ON jobs (job_type, dedupe_key)
  WHERE dedupe_key IS NOT NULL AND status IN ('queued', 'running');
