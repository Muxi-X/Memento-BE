-- Publishing: official publish flow

-- name: CreatePublishSession :one
INSERT INTO publish_sessions (
  owner_user_id,
  context_type,
  official_keyword_id,
  custom_keyword_id,
  biz_date,
  status,
  expires_at
) VALUES (
  sqlc.arg(owner_user_id)::uuid,
  sqlc.arg(context_type)::publish_context_type,
  sqlc.narg(official_keyword_id)::uuid,
  sqlc.narg(custom_keyword_id)::uuid,
  sqlc.narg(biz_date)::date,
  sqlc.arg(status)::publish_session_status,
  sqlc.arg(expires_at)::timestamptz
)
RETURNING *;

-- name: GetPublishSessionByIDForOwnerForUpdate :one
SELECT *
FROM publish_sessions
WHERE id = $1
  AND owner_user_id = $2
FOR UPDATE;

-- name: UpdatePublishSessionStateForOwner :one
UPDATE publish_sessions
SET status = sqlc.arg(status)::publish_session_status,
    published_upload_id = sqlc.narg(published_upload_id)::uuid
WHERE id = sqlc.arg(id)::uuid
  AND owner_user_id = sqlc.arg(owner_user_id)::uuid
RETURNING *;

-- name: UpsertPublishSessionItem :one
INSERT INTO publish_session_items (
  session_id,
  owner_user_id,
  client_image_id,
  image_asset_id,
  audio_asset_id,
  display_order,
  is_cover,
  title,
  note,
  status
) VALUES (
  sqlc.arg(session_id)::uuid,
  sqlc.arg(owner_user_id)::uuid,
  sqlc.arg(client_image_id)::varchar,
  sqlc.arg(image_asset_id)::uuid,
  sqlc.narg(audio_asset_id)::uuid,
  sqlc.narg(display_order)::int,
  sqlc.arg(is_cover)::boolean,
  sqlc.narg(title)::varchar,
  sqlc.narg(note)::varchar,
  sqlc.arg(status)::publish_session_item_status
)
ON CONFLICT (session_id, client_image_id) DO UPDATE
SET image_asset_id = EXCLUDED.image_asset_id,
    audio_asset_id = EXCLUDED.audio_asset_id,
    display_order = EXCLUDED.display_order,
    is_cover = EXCLUDED.is_cover,
    title = EXCLUDED.title,
    note = EXCLUDED.note,
    status = EXCLUDED.status
RETURNING *;

-- name: ListPublishSessionItemsBySessionForOwnerForUpdate :many
SELECT *
FROM publish_session_items
WHERE session_id = $1
  AND owner_user_id = $2
ORDER BY created_at ASC, id ASC
FOR UPDATE;

-- name: ClearPublishSessionItemCoverForOwner :execrows
UPDATE publish_session_items
SET is_cover = false
WHERE session_id = $1
  AND owner_user_id = $2
  AND is_cover = true;

-- name: GetDailyKeywordAssignmentByBizDate :one
SELECT *
FROM daily_keyword_assignments
WHERE biz_date = $1;

-- name: GetMediaAssetLiteByID :one
SELECT id, owner_user_id, media_kind, duration_ms, status
FROM media_assets
WHERE id = $1
  AND deleted_at IS NULL;

-- name: CreateWorkUpload :one
INSERT INTO work_uploads (
  author_user_id,
  context_type,
  official_keyword_id,
  custom_keyword_id,
  biz_date,
  visibility_status,
  cover_asset_id,
  image_count,
  published_at
) VALUES (
  sqlc.arg(author_user_id)::uuid,
  sqlc.arg(context_type)::publish_context_type,
  sqlc.narg(official_keyword_id)::uuid,
  sqlc.narg(custom_keyword_id)::uuid,
  sqlc.narg(biz_date)::date,
  sqlc.arg(visibility_status)::work_upload_visibility_status,
  sqlc.arg(cover_asset_id)::uuid,
  sqlc.arg(image_count)::int,
  sqlc.arg(published_at)::timestamptz
)
RETURNING *;

-- name: CreateWorkUploadImage :one
INSERT INTO work_upload_images (
  upload_id,
  image_asset_id,
  display_order
) VALUES (
  $1, $2, $3
)
RETURNING *;

-- name: UpsertWorkUploadImageContent :one
INSERT INTO work_upload_image_contents (
  work_upload_image_id,
  title,
  note,
  audio_asset_id,
  audio_duration_ms
) VALUES (
  sqlc.arg(work_upload_image_id)::uuid,
  sqlc.narg(title)::varchar,
  sqlc.narg(note)::varchar,
  sqlc.narg(audio_asset_id)::uuid,
  sqlc.narg(audio_duration_ms)::int
)
ON CONFLICT (work_upload_image_id) DO UPDATE
SET title = EXCLUDED.title,
    note = EXCLUDED.note,
    audio_asset_id = EXCLUDED.audio_asset_id,
    audio_duration_ms = EXCLUDED.audio_duration_ms
RETURNING *;
