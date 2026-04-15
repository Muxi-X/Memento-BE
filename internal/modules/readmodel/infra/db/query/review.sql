-- Review page readmodel

-- name: ListMyReviewDateStats :many
WITH dates AS (
  SELECT
    wu.biz_date,
    MIN(wu.official_keyword_id::text)::uuid AS keyword_id,
    COUNT(*)::int AS my_upload_count,
    COALESCE(SUM(wu.image_count), 0)::int AS my_image_count
  FROM work_uploads wu
  WHERE wu.author_user_id = $1
    AND wu.context_type = 'official_today'
    AND wu.deleted_at IS NULL
  GROUP BY wu.biz_date
  ORDER BY wu.biz_date DESC
  LIMIT $2
)
SELECT
  d.biz_date,
  ok.id AS keyword_id,
  ok.text,
  ok.category,
  ok.is_active,
  ok.display_order,
  d.my_upload_count,
  d.my_image_count
FROM dates d
JOIN official_keywords ok ON ok.id = d.keyword_id
ORDER BY d.biz_date DESC;

-- name: CountMyReviewParticipationDays :one
SELECT COUNT(DISTINCT wu.biz_date)::bigint AS count
FROM work_uploads wu
WHERE wu.author_user_id = $1
  AND wu.context_type = 'official_today'
  AND wu.deleted_at IS NULL;

-- name: CountMyReviewImageTotal :one
SELECT COALESCE(SUM(wu.image_count), 0)::bigint AS count
FROM work_uploads wu
WHERE wu.author_user_id = $1
  AND wu.context_type = 'official_today'
  AND wu.deleted_at IS NULL;

-- name: ListMyReviewKeywordCounts :many
WITH keyword_counts AS (
  SELECT
    wu.official_keyword_id AS keyword_id,
    COUNT(*)::int AS my_upload_count,
    COALESCE(SUM(wu.image_count), 0)::int AS my_image_count
  FROM work_uploads wu
  WHERE wu.author_user_id = $1
    AND wu.context_type = 'official_today'
    AND wu.deleted_at IS NULL
  GROUP BY wu.official_keyword_id
)
SELECT
  ok.id AS keyword_id,
  ok.text,
  ok.category,
  ok.is_active,
  ok.display_order,
  kc.my_upload_count,
  kc.my_image_count
FROM keyword_counts kc
JOIN official_keywords ok ON ok.id = kc.keyword_id
ORDER BY ok.display_order ASC, ok.id ASC;
