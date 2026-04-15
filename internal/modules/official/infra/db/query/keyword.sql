-- Official keywords + assignments + daily stats

-- name: GetOfficialKeywordByID :one
SELECT *
FROM official_keywords
WHERE id = $1;

-- name: GetOfficialKeywordByText :one
SELECT *
FROM official_keywords
WHERE text = $1;

-- name: ListActiveOfficialKeywords :many
SELECT *
FROM official_keywords
WHERE is_active = true
ORDER BY display_order ASC, id ASC;

-- name: UpsertOfficialKeyword :one
INSERT INTO official_keywords (
  id, text, category, is_active, display_order
) VALUES (
  sqlc.arg(id)::uuid,
  sqlc.arg(text)::varchar,
  sqlc.arg(category)::keyword_category,
  sqlc.arg(is_active)::boolean,
  COALESCE(sqlc.narg(display_order)::int, (SELECT COALESCE(MAX(ok.display_order), 0) + 1 FROM official_keywords ok))
)
ON CONFLICT (id) DO UPDATE
SET text = EXCLUDED.text,
    category = EXCLUDED.category,
    is_active = EXCLUDED.is_active,
    display_order = EXCLUDED.display_order
RETURNING *;

-- name: DeactivateOfficialKeyword :one
UPDATE official_keywords
SET is_active = false
WHERE id = $1
RETURNING *;

-- name: GetDailyKeywordAssignment :one
SELECT *
FROM daily_keyword_assignments
WHERE biz_date = $1;

-- name: UpsertDailyKeywordAssignment :one
INSERT INTO daily_keyword_assignments (
  biz_date, keyword_id
) VALUES (
  sqlc.arg(biz_date)::date,
  sqlc.arg(keyword_id)::uuid
)
ON CONFLICT (biz_date) DO UPDATE
SET keyword_id = EXCLUDED.keyword_id
RETURNING *;

-- name: GetKeywordForDateWithStats :one
SELECT
  dka.biz_date,
  ok.id AS keyword_id,
  ok.text,
  ok.category,
  ok.is_active,
  ok.display_order,
  COALESCE(dks.participant_user_count, 0) AS participant_user_count,
  COALESCE(dks.upload_count, 0) AS upload_count,
  COALESCE(dks.image_count, 0) AS image_count
FROM daily_keyword_assignments dka
JOIN official_keywords ok ON ok.id = dka.keyword_id
LEFT JOIN daily_keyword_stats dks ON dks.biz_date = dka.biz_date
WHERE dka.biz_date = $1;

-- name: GetDailyKeywordStat :one
SELECT *
FROM daily_keyword_stats
WHERE biz_date = $1;

-- name: UpsertDailyKeywordStat :one
INSERT INTO daily_keyword_stats (
  biz_date, participant_user_count, upload_count, image_count
) VALUES (
  sqlc.arg(biz_date)::date,
  sqlc.arg(participant_user_count)::int,
  sqlc.arg(upload_count)::int,
  sqlc.arg(image_count)::int
)
ON CONFLICT (biz_date) DO UPDATE
SET participant_user_count = EXCLUDED.participant_user_count,
    upload_count = EXCLUDED.upload_count,
    image_count = EXCLUDED.image_count
RETURNING *;

-- name: RecomputeDailyKeywordStatsFromUploads :one
WITH agg AS (
  SELECT
    sqlc.arg(biz_date)::date AS biz_date,
    COUNT(DISTINCT wu.author_user_id)::int AS participant_user_count,
    COUNT(DISTINCT wu.id)::int AS upload_count,
    COUNT(wui.id)::int AS image_count
  FROM work_uploads wu
  LEFT JOIN work_upload_images wui
    ON wui.upload_id = wu.id
   AND wui.deleted_at IS NULL
  WHERE wu.context_type = 'official_today'
    AND wu.biz_date = sqlc.arg(biz_date)::date
    AND wu.deleted_at IS NULL
)
INSERT INTO daily_keyword_stats (
  biz_date, participant_user_count, upload_count, image_count
)
SELECT biz_date, participant_user_count, upload_count, image_count
FROM agg
ON CONFLICT (biz_date) DO UPDATE
SET participant_user_count = EXCLUDED.participant_user_count,
    upload_count = EXCLUDED.upload_count,
    image_count = EXCLUDED.image_count
RETURNING *;
