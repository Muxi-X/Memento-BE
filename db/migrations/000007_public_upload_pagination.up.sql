CREATE INDEX IF NOT EXISTS idx_work_uploads_public_date_rand
  ON work_uploads (biz_date, rand_key, id)
  WHERE context_type = 'official_today'
    AND visibility_status = 'visible' AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_work_uploads_public_keyword_rand
  ON work_uploads (official_keyword_id, rand_key, id)
  WHERE context_type = 'official_today'
    AND visibility_status = 'visible' AND deleted_at IS NULL;
