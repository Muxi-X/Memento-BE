-- Media assets and variants (current schema only)

-- name: CreateMediaAsset :one
INSERT INTO media_assets (
  owner_user_id,
  media_kind,
  mime_type,
  original_object_key,
  byte_size,
  width,
  height,
  duration_ms,
  status
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetMediaAssetByID :one
SELECT *
FROM media_assets
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetMediaAssetByIDForUpdate :one
SELECT *
FROM media_assets
WHERE id = $1
  AND deleted_at IS NULL
  FOR UPDATE;

-- name: UpdateMediaAsset :one
UPDATE media_assets
SET width = COALESCE(sqlc.narg(width)::int, width),
    height = COALESCE(sqlc.narg(height)::int, height),
    duration_ms = COALESCE(sqlc.narg(duration_ms)::int, duration_ms),
    status = sqlc.arg(status)::media_asset_status,
    deleted_at = COALESCE(sqlc.narg(deleted_at)::timestamptz, deleted_at)
WHERE id = sqlc.arg(id)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- name: TransitionMediaAssetStatus :one
UPDATE media_assets
SET width = COALESCE(sqlc.narg(width)::int, width),
    height = COALESCE(sqlc.narg(height)::int, height),
    duration_ms = COALESCE(sqlc.narg(duration_ms)::int, duration_ms),
    status = sqlc.arg(next_status)::media_asset_status
WHERE id = sqlc.arg(id)::uuid
  AND status = sqlc.arg(current_status)::media_asset_status
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMediaAsset :one
UPDATE media_assets
SET status = 'deleted'::media_asset_status,
    deleted_at = sqlc.arg(deleted_at)::timestamptz
WHERE id = sqlc.arg(id)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- name: UpsertMediaVariant :one
INSERT INTO media_variants (
  asset_id,
  variant_name,
  object_key,
  width,
  height,
  status
) VALUES (
  sqlc.arg(asset_id)::uuid,
  sqlc.arg(variant_name)::text,
  sqlc.arg(object_key)::text,
  sqlc.narg(width)::int,
  sqlc.narg(height)::int,
  sqlc.arg(status)::media_variant_status
)
ON CONFLICT (asset_id, variant_name) DO UPDATE
SET object_key = EXCLUDED.object_key,
    width = COALESCE(EXCLUDED.width, media_variants.width),
    height = COALESCE(EXCLUDED.height, media_variants.height),
    status = EXCLUDED.status
RETURNING *;

-- name: GetMediaVariantByAssetIDAndName :one
SELECT *
FROM media_variants
WHERE asset_id = $1
  AND variant_name = $2;

-- name: GetMediaVariantByAssetIDAndNameForUpdate :one
SELECT *
FROM media_variants
WHERE asset_id = $1
  AND variant_name = $2
FOR UPDATE;

-- name: ListMediaVariantsByAssetID :many
SELECT *
FROM media_variants
WHERE asset_id = $1
ORDER BY variant_name ASC, id ASC;

-- name: UpdateMediaVariant :one
UPDATE media_variants
SET object_key = COALESCE(sqlc.narg(object_key)::text, object_key),
    width = COALESCE(sqlc.narg(width)::int, width),
    height = COALESCE(sqlc.narg(height)::int, height),
    status = sqlc.arg(status)::media_variant_status
WHERE asset_id = sqlc.arg(asset_id)::uuid
  AND variant_name = sqlc.arg(variant_name)::text
RETURNING *;

-- name: TransitionMediaVariantStatus :one
UPDATE media_variants
SET width = COALESCE(sqlc.narg(width)::int, width),
    height = COALESCE(sqlc.narg(height)::int, height),
    status = sqlc.arg(next_status)::media_variant_status
WHERE asset_id = sqlc.arg(asset_id)::uuid
  AND variant_name = sqlc.arg(variant_name)::text
  AND status = sqlc.arg(current_status)::media_variant_status
RETURNING *;

-- name: CountMediaAssetLiveReferences :one
SELECT (
  COALESCE((
    SELECT COUNT(*)::bigint
    FROM user_profiles up
    WHERE up.current_avatar_asset_id = sqlc.arg(asset_id)::uuid
  ), 0)
  +
  COALESCE((
    SELECT COUNT(*)::bigint
    FROM avatar_upload_sessions aus
    WHERE aus.image_asset_id = sqlc.arg(asset_id)::uuid
      AND aus.status IN ('created', 'presigned', 'uploaded', 'committing')
  ), 0)
  +
  COALESCE((
    SELECT COUNT(*)::bigint
    FROM publish_session_items psi
    JOIN publish_sessions ps
      ON ps.id = psi.session_id
    WHERE (psi.image_asset_id = sqlc.arg(asset_id)::uuid OR psi.audio_asset_id = sqlc.arg(asset_id)::uuid)
      AND ps.status = 'created'
  ), 0)
  +
  COALESCE((
    SELECT COUNT(*)::bigint
    FROM work_uploads wu
    WHERE wu.cover_asset_id = sqlc.arg(asset_id)::uuid
      AND wu.deleted_at IS NULL
  ), 0)
  +
  COALESCE((
    SELECT COUNT(*)::bigint
    FROM work_upload_images wui
    WHERE wui.image_asset_id = sqlc.arg(asset_id)::uuid
      AND wui.deleted_at IS NULL
  ), 0)
  +
  COALESCE((
    SELECT COUNT(*)::bigint
    FROM work_upload_image_contents wuic
    WHERE wuic.audio_asset_id = sqlc.arg(asset_id)::uuid
  ), 0)
  +
  COALESCE((
    SELECT COUNT(*)::bigint
    FROM custom_keywords ck
    WHERE ck.cover_asset_id = sqlc.arg(asset_id)::uuid
      AND ck.deleted_at IS NULL
  ), 0)
)::bigint AS ref_count;
