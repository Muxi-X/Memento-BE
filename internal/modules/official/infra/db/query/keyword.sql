-- Official keywords + assignments + daily stats + rotation

-- name: GetOfficialKeywordByID :one
SELECT *
FROM official_keywords
WHERE id = $1;

-- name: GetOfficialKeywordByText :one
SELECT *
FROM official_keywords
WHERE text = $1;

-- name: ListOfficialKeywords :many
SELECT *
FROM official_keywords
ORDER BY display_order ASC, id ASC;

-- name: ListActiveOfficialKeywords :many
SELECT *
FROM official_keywords
WHERE is_active = true
ORDER BY display_order ASC, id ASC;

-- name: InsertOfficialKeyword :one
INSERT INTO official_keywords (
  id, text, category, is_active, display_order
) VALUES (
  sqlc.arg(id)::uuid,
  sqlc.arg(text)::varchar,
  sqlc.arg(category)::keyword_category,
  sqlc.arg(is_active)::boolean,
  COALESCE(sqlc.narg(display_order)::int, (SELECT COALESCE(MAX(ok.display_order), 0) + 1 FROM official_keywords ok))
)
RETURNING *;

-- name: UpdateOfficialKeyword :one
UPDATE official_keywords
SET text = sqlc.arg(text)::varchar,
    category = sqlc.arg(category)::keyword_category,
    display_order = sqlc.arg(display_order)::int
WHERE id = sqlc.arg(id)::uuid
RETURNING *;

-- name: GetDailyKeywordAssignment :one
SELECT *
FROM daily_keyword_assignments
WHERE biz_date = $1;

-- name: GetLastDailyKeywordAssignmentBefore :one
SELECT *
FROM daily_keyword_assignments
WHERE biz_date < $1
ORDER BY biz_date DESC
LIMIT 1;

-- name: HasDailyKeywordAssignmentOnOrAfter :one
SELECT EXISTS (
  SELECT 1
  FROM daily_keyword_assignments
  WHERE biz_date >= $1
);

-- name: InsertDailyKeywordAssignment :one
INSERT INTO daily_keyword_assignments (
  biz_date, keyword_id
) VALUES (
  sqlc.arg(biz_date)::date,
  sqlc.arg(keyword_id)::uuid
)
ON CONFLICT (biz_date) DO NOTHING
RETURNING *;

-- name: ListDailyKeywordAssignmentsBetween :many
SELECT *
FROM daily_keyword_assignments
WHERE biz_date >= sqlc.arg(start_date)::date
  AND biz_date <= sqlc.arg(end_date)::date
ORDER BY biz_date ASC;

-- name: LockOfficialKeywordRotationState :one
SELECT *
FROM official_keyword_rotation_state
WHERE singleton = TRUE
FOR UPDATE;

-- name: InitializeOfficialKeywordRotation :one
UPDATE official_keyword_rotation_state
SET initialized = TRUE,
    effective_date = sqlc.arg(effective_date)::date,
    last_planned_date = sqlc.arg(effective_date)::date - 1,
    next_queue_position = 1,
    updated_at = now()
WHERE singleton = TRUE
  AND initialized = FALSE
RETURNING *;

-- name: InsertOfficialKeywordRotationQueueItem :exec
INSERT INTO official_keyword_rotation_queue (
  keyword_id, queue_position
)
VALUES (
  sqlc.arg(keyword_id)::uuid,
  sqlc.arg(queue_position)::bigint
);

-- name: SyncOfficialKeywordRotationQueueTail :one
UPDATE official_keyword_rotation_state
SET next_queue_position = COALESCE(
      (SELECT MAX(queue_position) + 1 FROM official_keyword_rotation_queue),
      1
    ),
    updated_at = now()
WHERE singleton = TRUE
RETURNING *;

-- name: ListOfficialKeywordRotationQueue :many
SELECT
  q.keyword_id,
  q.queue_position,
  ok.is_active
FROM official_keyword_rotation_queue q
JOIN official_keywords ok ON ok.id = q.keyword_id
ORDER BY q.queue_position ASC;

-- name: MoveOfficialKeywordToRotationTail :one
WITH next_position AS (
  UPDATE official_keyword_rotation_state
  SET next_queue_position = next_queue_position + 1,
      updated_at = now()
  WHERE singleton = TRUE
  RETURNING next_queue_position - 1 AS queue_position
)
UPDATE official_keyword_rotation_queue q
SET queue_position = next_position.queue_position
FROM next_position
WHERE q.keyword_id = sqlc.arg(keyword_id)::uuid
RETURNING q.keyword_id, q.queue_position;

-- name: MoveOrInsertOfficialKeywordAtRotationTail :one
WITH next_position AS (
  UPDATE official_keyword_rotation_state
  SET next_queue_position = next_queue_position + 1,
      updated_at = now()
  WHERE singleton = TRUE
  RETURNING next_queue_position - 1 AS queue_position
)
INSERT INTO official_keyword_rotation_queue (keyword_id, queue_position)
SELECT sqlc.arg(keyword_id)::uuid, next_position.queue_position
FROM next_position
ON CONFLICT (keyword_id) DO UPDATE
SET queue_position = EXCLUDED.queue_position
RETURNING keyword_id, queue_position;

-- name: AdvanceOfficialKeywordRotationDate :one
UPDATE official_keyword_rotation_state
SET last_planned_date = sqlc.arg(biz_date)::date,
    updated_at = now()
WHERE singleton = TRUE
RETURNING *;

-- name: InsertOfficialKeywordChange :one
INSERT INTO official_keyword_changes (
  keyword_id, action, effective_date
) VALUES (
  sqlc.arg(keyword_id)::uuid,
  sqlc.arg(action)::text,
  sqlc.arg(effective_date)::date
)
RETURNING *;

-- name: SnapshotOfficialKeywordCatalog :exec
INSERT INTO official_keyword_catalog_baseline (keyword_id, initial_is_active)
SELECT id, is_active FROM official_keywords;

-- name: RegisterOfficialKeywordBaseline :exec
INSERT INTO official_keyword_catalog_baseline (keyword_id, initial_is_active)
VALUES (sqlc.arg(keyword_id)::uuid, FALSE);

-- name: HasOfficialKeywordCatalogMismatch :one
SELECT EXISTS (
  SELECT 1
  FROM official_keywords ok
  FULL JOIN official_keyword_catalog_baseline baseline ON baseline.keyword_id = ok.id
  LEFT JOIN LATERAL (
    SELECT change.action
    FROM official_keyword_changes change
    WHERE change.keyword_id = baseline.keyword_id
      AND change.applied_at IS NOT NULL
    ORDER BY change.effective_date DESC, change.id DESC
    LIMIT 1
  ) applied ON TRUE
  WHERE ok.id IS NULL
     OR baseline.keyword_id IS NULL
     OR ok.is_active IS DISTINCT FROM COALESCE(applied.action = 'activate', baseline.initial_is_active)
);

-- name: ListPendingOfficialKeywordChangesThrough :many
SELECT *
FROM official_keyword_changes
WHERE applied_at IS NULL
  AND effective_date <= sqlc.arg(biz_date)::date
ORDER BY effective_date ASC, id ASC;

-- name: SetOfficialKeywordActive :exec
UPDATE official_keywords
SET is_active = sqlc.arg(is_active)::boolean
WHERE id = sqlc.arg(keyword_id)::uuid;

-- name: MarkOfficialKeywordChangeApplied :exec
UPDATE official_keyword_changes
SET applied_at = now()
WHERE id = sqlc.arg(id)::bigint
  AND applied_at IS NULL;

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
