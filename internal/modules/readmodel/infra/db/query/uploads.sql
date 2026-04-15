-- Shared upload readmodel queries

-- name: GetOfficialKeyword :one
SELECT
  id,
  text,
  category,
  is_active,
  display_order
FROM official_keywords
WHERE id = $1;

-- name: ListPublicUploadsByDateLatest :many
SELECT
  wu.id,
  wu.biz_date,
  wu.official_keyword_id AS keyword_id,
  cover_image.id AS cover_image_id,
  COALESCE(NULLIF(btrim(cover_content.title), ''), NULLIF(btrim(cover_content.note), '')) AS display_text,
  CASE WHEN cover_content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS cover_has_audio,
  cover_content.audio_duration_ms AS cover_audio_duration_ms,
  cover_asset.original_object_key AS cover_object_key,
  wu.image_count,
  wu.reaction_inspired_count,
  wu.reaction_resonated_count,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images cover_image
  ON cover_image.upload_id = wu.id
 AND cover_image.image_asset_id = wu.cover_asset_id
 AND cover_image.deleted_at IS NULL
LEFT JOIN work_upload_image_contents cover_content
  ON cover_content.work_upload_image_id = cover_image.id
JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.context_type = 'official_today'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL
  AND wu.biz_date = $1
ORDER BY wu.published_at DESC, wu.id DESC
LIMIT $2;

-- name: ListPublicUploadsByDateRandom :many
SELECT
  wu.id,
  wu.biz_date,
  wu.official_keyword_id AS keyword_id,
  cover_image.id AS cover_image_id,
  COALESCE(NULLIF(btrim(cover_content.title), ''), NULLIF(btrim(cover_content.note), '')) AS display_text,
  CASE WHEN cover_content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS cover_has_audio,
  cover_content.audio_duration_ms AS cover_audio_duration_ms,
  cover_asset.original_object_key AS cover_object_key,
  wu.image_count,
  wu.reaction_inspired_count,
  wu.reaction_resonated_count,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images cover_image
  ON cover_image.upload_id = wu.id
 AND cover_image.image_asset_id = wu.cover_asset_id
 AND cover_image.deleted_at IS NULL
LEFT JOIN work_upload_image_contents cover_content
  ON cover_content.work_upload_image_id = cover_image.id
JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.context_type = 'official_today'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL
  AND wu.biz_date = sqlc.arg(biz_date)::date
ORDER BY
  CASE WHEN wu.rand_key < sqlc.arg(seed)::double precision THEN 1 ELSE 0 END ASC,
  wu.rand_key ASC,
  wu.id ASC
LIMIT sqlc.arg(limit_count)::int;

-- name: ListPublicUploadsByKeywordLatest :many
SELECT
  wu.id,
  wu.biz_date,
  wu.official_keyword_id AS keyword_id,
  cover_image.id AS cover_image_id,
  COALESCE(NULLIF(btrim(cover_content.title), ''), NULLIF(btrim(cover_content.note), '')) AS display_text,
  CASE WHEN cover_content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS cover_has_audio,
  cover_content.audio_duration_ms AS cover_audio_duration_ms,
  cover_asset.original_object_key AS cover_object_key,
  wu.image_count,
  wu.reaction_inspired_count,
  wu.reaction_resonated_count,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images cover_image
  ON cover_image.upload_id = wu.id
 AND cover_image.image_asset_id = wu.cover_asset_id
 AND cover_image.deleted_at IS NULL
LEFT JOIN work_upload_image_contents cover_content
  ON cover_content.work_upload_image_id = cover_image.id
JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.context_type = 'official_today'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL
  AND wu.official_keyword_id = $1
ORDER BY wu.published_at DESC, wu.id DESC
LIMIT $2;

-- name: ListPublicUploadsByKeywordRandom :many
SELECT
  wu.id,
  wu.biz_date,
  wu.official_keyword_id AS keyword_id,
  cover_image.id AS cover_image_id,
  COALESCE(NULLIF(btrim(cover_content.title), ''), NULLIF(btrim(cover_content.note), '')) AS display_text,
  CASE WHEN cover_content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS cover_has_audio,
  cover_content.audio_duration_ms AS cover_audio_duration_ms,
  cover_asset.original_object_key AS cover_object_key,
  wu.image_count,
  wu.reaction_inspired_count,
  wu.reaction_resonated_count,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images cover_image
  ON cover_image.upload_id = wu.id
 AND cover_image.image_asset_id = wu.cover_asset_id
 AND cover_image.deleted_at IS NULL
LEFT JOIN work_upload_image_contents cover_content
  ON cover_content.work_upload_image_id = cover_image.id
JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.context_type = 'official_today'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL
  AND wu.official_keyword_id = sqlc.arg(keyword_id)::uuid
ORDER BY
  CASE WHEN wu.rand_key < sqlc.arg(seed)::double precision THEN 1 ELSE 0 END ASC,
  wu.rand_key ASC,
  wu.id ASC
LIMIT sqlc.arg(limit_count)::int;

-- name: GetPublicUploadCard :one
SELECT
  wu.id,
  wu.biz_date,
  wu.official_keyword_id AS keyword_id,
  cover_image.id AS cover_image_id,
  COALESCE(NULLIF(btrim(cover_content.title), ''), NULLIF(btrim(cover_content.note), '')) AS display_text,
  CASE WHEN cover_content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS cover_has_audio,
  cover_content.audio_duration_ms AS cover_audio_duration_ms,
  cover_asset.original_object_key AS cover_object_key,
  wu.image_count,
  wu.reaction_inspired_count,
  wu.reaction_resonated_count,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images cover_image
  ON cover_image.upload_id = wu.id
 AND cover_image.image_asset_id = wu.cover_asset_id
 AND cover_image.deleted_at IS NULL
LEFT JOIN work_upload_image_contents cover_content
  ON cover_content.work_upload_image_id = cover_image.id
JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.id = $1
  AND wu.context_type = 'official_today'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL;

-- name: GetMyReviewUploadCard :one
SELECT
  wu.id,
  wu.biz_date,
  wu.official_keyword_id AS keyword_id,
  cover_image.id AS cover_image_id,
  COALESCE(NULLIF(btrim(cover_content.title), ''), NULLIF(btrim(cover_content.note), '')) AS display_text,
  CASE WHEN cover_content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS cover_has_audio,
  cover_content.audio_duration_ms AS cover_audio_duration_ms,
  cover_asset.original_object_key AS cover_object_key,
  wu.image_count,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images cover_image
  ON cover_image.upload_id = wu.id
 AND cover_image.image_asset_id = wu.cover_asset_id
 AND cover_image.deleted_at IS NULL
LEFT JOIN work_upload_image_contents cover_content
  ON cover_content.work_upload_image_id = cover_image.id
JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.id = $1
  AND wu.author_user_id = $2
  AND wu.context_type = 'official_today'
  AND wu.deleted_at IS NULL;

-- name: ListMyReviewUploadsByDate :many
SELECT
  wu.id,
  wu.biz_date,
  wu.official_keyword_id AS keyword_id,
  cover_image.id AS cover_image_id,
  COALESCE(NULLIF(btrim(cover_content.title), ''), NULLIF(btrim(cover_content.note), '')) AS display_text,
  CASE WHEN cover_content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS cover_has_audio,
  cover_content.audio_duration_ms AS cover_audio_duration_ms,
  cover_asset.original_object_key AS cover_object_key,
  wu.image_count,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images cover_image
  ON cover_image.upload_id = wu.id
 AND cover_image.image_asset_id = wu.cover_asset_id
 AND cover_image.deleted_at IS NULL
LEFT JOIN work_upload_image_contents cover_content
  ON cover_content.work_upload_image_id = cover_image.id
JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.author_user_id = $1
  AND wu.context_type = 'official_today'
  AND wu.biz_date = $2
  AND wu.deleted_at IS NULL
ORDER BY wu.published_at DESC, wu.id DESC
LIMIT $3;

-- name: ListMyReviewUploadsByKeyword :many
SELECT
  wu.id,
  wu.biz_date,
  wu.official_keyword_id AS keyword_id,
  cover_image.id AS cover_image_id,
  COALESCE(NULLIF(btrim(cover_content.title), ''), NULLIF(btrim(cover_content.note), '')) AS display_text,
  CASE WHEN cover_content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS cover_has_audio,
  cover_content.audio_duration_ms AS cover_audio_duration_ms,
  cover_asset.original_object_key AS cover_object_key,
  wu.image_count,
  wu.published_at AS created_at
FROM work_uploads wu
JOIN work_upload_images cover_image
  ON cover_image.upload_id = wu.id
 AND cover_image.image_asset_id = wu.cover_asset_id
 AND cover_image.deleted_at IS NULL
LEFT JOIN work_upload_image_contents cover_content
  ON cover_content.work_upload_image_id = cover_image.id
JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.author_user_id = $1
  AND wu.context_type = 'official_today'
  AND wu.official_keyword_id = $2
  AND wu.deleted_at IS NULL
ORDER BY wu.published_at DESC, wu.id DESC
LIMIT $3;

-- name: ListUploadImages :many
SELECT
  wui.id,
  wui.image_asset_id,
  image_asset.original_object_key AS original_object_key,
  image_asset.width AS original_width,
  image_asset.height AS original_height,
  wui.display_order,
  content.title,
  content.note,
  CASE WHEN content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END AS has_audio,
  content.audio_duration_ms,
  audio_asset.original_object_key AS audio_object_key,
  wui.created_at
FROM work_upload_images wui
JOIN media_assets image_asset
  ON image_asset.id = wui.image_asset_id
 AND image_asset.deleted_at IS NULL
LEFT JOIN work_upload_image_contents content
  ON content.work_upload_image_id = wui.id
LEFT JOIN media_assets audio_asset
  ON audio_asset.id = content.audio_asset_id
 AND audio_asset.deleted_at IS NULL
WHERE wui.upload_id = $1
  AND wui.deleted_at IS NULL
ORDER BY wui.display_order ASC, wui.id ASC;

-- name: ListMyReactionTypesByUploadIDs :many
SELECT
  upload_id,
  type
FROM work_upload_reactions
WHERE user_id = $1
  AND upload_id = ANY($2::uuid[])
ORDER BY upload_id ASC, type ASC;
