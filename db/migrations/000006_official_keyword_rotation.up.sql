CREATE TABLE official_keyword_rotation_state (
  singleton boolean PRIMARY KEY DEFAULT TRUE CHECK (singleton),
  initialized boolean NOT NULL DEFAULT FALSE,
  effective_date date NULL,
  last_planned_date date NULL,
  next_queue_position bigint NOT NULL DEFAULT 1 CHECK (next_queue_position > 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT official_keyword_rotation_state_dates_check CHECK (
    (initialized AND effective_date IS NOT NULL AND last_planned_date IS NOT NULL)
    OR
    (NOT initialized AND effective_date IS NULL AND last_planned_date IS NULL)
  )
);

-- The state intentionally starts uninitialized. Application startup supplies the
-- confirmed S = next Asia/Shanghai business date and persists it exactly once.
INSERT INTO official_keyword_rotation_state (singleton)
VALUES (TRUE);

CREATE TABLE official_keyword_rotation_queue (
  keyword_id uuid PRIMARY KEY REFERENCES official_keywords(id) ON DELETE CASCADE,
  queue_position bigint NOT NULL UNIQUE CHECK (queue_position > 0),
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Preserve the activity known at cutover (or before a new keyword's first
-- activation). Together with applied changes this explains the live catalog.
CREATE TABLE official_keyword_catalog_baseline (
  keyword_id uuid PRIMARY KEY REFERENCES official_keywords(id),
  initial_is_active boolean NOT NULL
);

CREATE TABLE official_keyword_changes (
  id bigserial PRIMARY KEY,
  keyword_id uuid NOT NULL REFERENCES official_keywords(id) ON DELETE CASCADE,
  action text NOT NULL CHECK (action IN ('activate', 'deactivate')),
  effective_date date NOT NULL,
  applied_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_official_keyword_changes_pending_effective
  ON official_keyword_changes (effective_date, id)
  WHERE applied_at IS NULL;

CREATE INDEX idx_official_keyword_changes_applied_keyword
  ON official_keyword_changes (keyword_id, effective_date DESC, id DESC)
  WHERE applied_at IS NOT NULL;
