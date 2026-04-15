-- Custom keywords

-- name: CreateCustomKeyword :one
INSERT INTO custom_keywords (
  owner_user_id,
  text,
  target_image_count,
  cover_mode,
  status
) VALUES (
  sqlc.arg(owner_user_id)::uuid,
  sqlc.arg(text)::varchar,
  COALESCE(sqlc.narg(target_image_count)::int, 0),
  'auto_latest'::custom_keyword_cover_mode,
  'active'::custom_keyword_status
)
RETURNING *;

-- name: GetCustomKeywordByIDForUser :one
SELECT *
FROM custom_keywords
WHERE id = sqlc.arg(id)::uuid
  AND owner_user_id = sqlc.arg(owner_user_id)::uuid
  AND deleted_at IS NULL;

-- name: UpdateCustomKeywordForUser :one
UPDATE custom_keywords
SET
  text = COALESCE(sqlc.narg(text)::varchar, text),
  target_image_count = COALESCE(sqlc.narg(target_image_count)::int, target_image_count),
  status = COALESCE(sqlc.narg(status)::custom_keyword_status, status)
WHERE id = sqlc.arg(id)::uuid
  AND owner_user_id = sqlc.arg(owner_user_id)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCustomKeywordForUser :execrows
UPDATE custom_keywords
SET deleted_at = now(),
    status = 'deleted'::custom_keyword_status
WHERE id = sqlc.arg(id)::uuid
  AND owner_user_id = sqlc.arg(owner_user_id)::uuid
  AND deleted_at IS NULL;

-- name: ListCustomKeywordSummariesForUser :many
WITH keyword_stats AS (
  SELECT
    wu.custom_keyword_id,
    COUNT(wui.id)::int AS total_image_count
  FROM work_uploads wu
  JOIN work_upload_images wui
    ON wui.upload_id = wu.id
   AND wui.deleted_at IS NULL
  WHERE wu.author_user_id = sqlc.arg(owner_user_id)::uuid
    AND wu.context_type = 'custom_keyword'
    AND wu.visibility_status = 'visible'
    AND wu.deleted_at IS NULL
    AND wu.custom_keyword_id IS NOT NULL
  GROUP BY wu.custom_keyword_id
)
SELECT
  ck.*,
  COALESCE(ks.total_image_count, 0)::int AS total_image_count
FROM custom_keywords ck
LEFT JOIN keyword_stats ks
  ON ks.custom_keyword_id = ck.id
WHERE ck.owner_user_id = sqlc.arg(owner_user_id)::uuid
  AND ck.deleted_at IS NULL
ORDER BY ck.created_at DESC, ck.id DESC;

-- name: CountVisibleCustomKeywordImages :one
SELECT COALESCE(COUNT(wui.id), 0)::int
FROM work_uploads wu
JOIN work_upload_images wui
  ON wui.upload_id = wu.id
 AND wui.deleted_at IS NULL
WHERE wu.author_user_id = sqlc.arg(owner_user_id)::uuid
  AND wu.custom_keyword_id = sqlc.arg(keyword_id)::uuid
  AND wu.context_type = 'custom_keyword'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL;

-- name: ResolveCustomKeywordImageAsset :one
SELECT wui.image_asset_id
FROM work_upload_images wui
JOIN work_uploads wu
  ON wu.id = wui.upload_id
WHERE wui.id = sqlc.arg(image_id)::uuid
  AND wu.author_user_id = sqlc.arg(owner_user_id)::uuid
  AND wu.custom_keyword_id = sqlc.arg(keyword_id)::uuid
  AND wu.context_type = 'custom_keyword'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL
  AND wui.deleted_at IS NULL;

-- name: SetCustomKeywordManualCover :one
UPDATE custom_keywords
SET cover_asset_id = sqlc.arg(asset_id)::uuid,
    cover_mode = 'manual'::custom_keyword_cover_mode
WHERE id = sqlc.arg(id)::uuid
  AND owner_user_id = sqlc.arg(owner_user_id)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- name: ClearCustomKeywordCover :one
UPDATE custom_keywords
SET cover_asset_id = NULL,
    cover_mode = 'auto_latest'::custom_keyword_cover_mode
WHERE id = sqlc.arg(id)::uuid
  AND owner_user_id = sqlc.arg(owner_user_id)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- name: GetCustomKeywordCoverAsset :one
SELECT
  id,
  original_object_key,
  width,
  height
FROM media_assets
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetLatestCustomKeywordCoverAsset :one
SELECT
  ma.id,
  ma.original_object_key,
  ma.width,
  ma.height
FROM work_uploads wu
JOIN work_upload_images wui
  ON wui.upload_id = wu.id
 AND wui.deleted_at IS NULL
JOIN media_assets ma
  ON ma.id = wui.image_asset_id
 AND ma.deleted_at IS NULL
WHERE wu.author_user_id = sqlc.arg(owner_user_id)::uuid
  AND wu.custom_keyword_id = sqlc.arg(keyword_id)::uuid
  AND wu.context_type = 'custom_keyword'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL
ORDER BY wu.published_at DESC, wu.id DESC, wui.display_order ASC, wui.id ASC
LIMIT 1;

-- name: ListCustomKeywordImages :many
SELECT
  wui.id,
  wui.image_asset_id,
  ma.original_object_key,
  ma.width,
  ma.height,
  wui.display_order,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images wui
  ON wui.upload_id = wu.id
 AND wui.deleted_at IS NULL
JOIN media_assets ma
  ON ma.id = wui.image_asset_id
 AND ma.deleted_at IS NULL
WHERE wu.author_user_id = sqlc.arg(owner_user_id)::uuid
  AND wu.custom_keyword_id = sqlc.arg(keyword_id)::uuid
  AND wu.context_type = 'custom_keyword'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL
ORDER BY wu.published_at DESC, wu.id DESC, wui.display_order ASC, wui.id ASC
LIMIT sqlc.arg(row_limit)::int;

-- name: GetCustomKeywordImageDetail :one
SELECT
  wui.id,
  wu.custom_keyword_id,
  wui.image_asset_id,
  ma.original_object_key,
  ma.width,
  ma.height,
  wui.display_order,
  wuic.title,
  wuic.note,
  (wuic.audio_asset_id IS NOT NULL) AS has_audio,
  wuic.audio_duration_ms,
  audio.original_object_key AS audio_object_key,
  wu.published_at AS created_at
FROM work_upload_images wui
JOIN work_uploads wu
  ON wu.id = wui.upload_id
JOIN media_assets ma
  ON ma.id = wui.image_asset_id
 AND ma.deleted_at IS NULL
LEFT JOIN work_upload_image_contents wuic
  ON wuic.work_upload_image_id = wui.id
LEFT JOIN media_assets audio
  ON audio.id = wuic.audio_asset_id
 AND audio.deleted_at IS NULL
WHERE wui.id = sqlc.arg(image_id)::uuid
  AND wu.author_user_id = sqlc.arg(owner_user_id)::uuid
  AND wu.context_type = 'custom_keyword'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL
  AND wui.deleted_at IS NULL;
