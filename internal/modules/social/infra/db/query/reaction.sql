-- Reactions / notifications

-- name: GetVisibleReactionTarget :one
SELECT
  wu.id AS upload_id,
  wu.author_user_id,
  us.reaction_notification_enabled,
  cover_asset.original_object_key AS target_cover_object_key
FROM work_uploads wu
JOIN user_settings us
  ON us.user_id = wu.author_user_id
LEFT JOIN media_assets cover_asset
  ON cover_asset.id = wu.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE wu.id = $1
  AND wu.context_type = 'official_today'
  AND wu.visibility_status = 'visible'
  AND wu.deleted_at IS NULL;

-- name: GetActorSnapshot :one
SELECT
  up.user_id,
  up.nickname,
  ma.original_object_key AS avatar_object_key
FROM user_profiles up
LEFT JOIN media_assets ma
  ON ma.id = up.current_avatar_asset_id
 AND ma.deleted_at IS NULL
WHERE up.user_id = $1;

-- name: InsertReaction :one
INSERT INTO work_upload_reactions (upload_id, user_id, type)
VALUES (
  sqlc.arg(upload_id)::uuid,
  sqlc.arg(user_id)::uuid,
  sqlc.arg(type)::reaction_type
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: DeleteReaction :one
DELETE FROM work_upload_reactions
WHERE upload_id = sqlc.arg(upload_id)::uuid
  AND user_id = sqlc.arg(user_id)::uuid
  AND type = sqlc.arg(type)::reaction_type
RETURNING *;

-- name: RecomputeReactionCounts :one
UPDATE work_uploads
SET reaction_inspired_count = (
      SELECT COUNT(*)::int
      FROM work_upload_reactions
      WHERE upload_id = sqlc.arg(upload_id)::uuid
        AND type = 'inspired'
    ),
    reaction_resonated_count = (
      SELECT COUNT(*)::int
      FROM work_upload_reactions
      WHERE upload_id = sqlc.arg(upload_id)::uuid
        AND type = 'resonated'
    )
WHERE id = sqlc.arg(upload_id)::uuid
RETURNING id, reaction_inspired_count, reaction_resonated_count;

-- name: CreateNotification :exec
INSERT INTO notifications (
  recipient_user_id,
  actor_user_id,
  actor_nickname_snapshot,
  actor_avatar_variant_key_snapshot,
  target_upload_id,
  target_upload_cover_variant_key_snapshot,
  type,
  reaction_type
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListNotifications :many
SELECT
  n.id,
  n.actor_avatar_variant_key_snapshot AS actor_avatar_object_key,
  n.target_upload_id,
  n.target_upload_cover_variant_key_snapshot AS target_cover_object_key,
  n.type,
  n.reaction_type,
  n.read_at,
  n.created_at
FROM notifications n
WHERE n.recipient_user_id = $1
ORDER BY n.created_at DESC, n.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET read_at = sqlc.arg(read_at)::timestamptz
WHERE recipient_user_id = sqlc.arg(recipient_user_id)::uuid
  AND read_at IS NULL;
