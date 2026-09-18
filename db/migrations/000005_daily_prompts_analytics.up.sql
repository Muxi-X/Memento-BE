CREATE TABLE user_daily_prompts (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  biz_date date NOT NULL,
  keyword_id uuid NOT NULL,
  prompt_id uuid NOT NULL,
  kind prompt_kind NOT NULL,
  content_snapshot text NOT NULL CHECK (content_snapshot <> ''),
  selected_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, biz_date)
);

CREATE INDEX idx_user_daily_prompts_biz_date
  ON user_daily_prompts (biz_date);

CREATE TABLE analytics_events (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_id uuid NOT NULL,
  event_name text NOT NULL CHECK (event_name IN ('prompt_entry_click', 'prompt_kind_entry_click')),
  kind prompt_kind NULL,
  source text NOT NULL CHECK (source = 'today'),
  schema_version smallint NOT NULL CHECK (schema_version = 1),
  occurred_at timestamptz NOT NULL,
  selection_state text NOT NULL CHECK (selection_state IN ('unknown', 'unselected', 'selected')),
  context_biz_date date NULL,
  keyword_id uuid NULL,
  received_at timestamptz NOT NULL,
  biz_date date NOT NULL,
  request_id text NOT NULL,
  PRIMARY KEY (user_id, event_id),
  CONSTRAINT analytics_events_combination_check CHECK (
    (event_name = 'prompt_entry_click' AND kind IS NULL)
    OR
    (event_name = 'prompt_kind_entry_click' AND kind IS NOT NULL AND selection_state = 'unselected')
  )
);

CREATE INDEX idx_analytics_events_biz_date_event_kind
  ON analytics_events (biz_date, event_name, kind);
