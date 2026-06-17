-- Profile / settings

-- name: GetProfileSettings :one
SELECT
  up.nickname,
  uei.email,
  ma.original_object_key AS avatar_object_key,
  us.reaction_notification_enabled
FROM user_profiles up
JOIN user_settings us
  ON us.user_id = up.user_id
LEFT JOIN user_email_identities uei
  ON uei.user_id = up.user_id
LEFT JOIN media_assets ma
  ON ma.id = up.current_avatar_asset_id
 AND ma.deleted_at IS NULL
WHERE up.user_id = $1;

-- name: UpdateUserReactionNotifications :execrows
UPDATE user_settings
SET reaction_notification_enabled = $2
WHERE user_id = $1;

-- name: UpdateUserNickname :execrows
UPDATE user_profiles
SET nickname = $2
WHERE user_id = $1;

-- name: CreateAvatarUploadSession :one
INSERT INTO avatar_upload_sessions (
  user_id,
  status,
  expires_at
) VALUES (
  $1,
  'created',
  $2
)
RETURNING *;

-- name: GetAvatarUploadSessionForUpdate :one
SELECT *
FROM avatar_upload_sessions
WHERE id = $1
  AND user_id = $2
FOR UPDATE;

-- name: SetAvatarUploadSessionImage :one
UPDATE avatar_upload_sessions
SET image_asset_id = $3,
    status = 'presigned'
WHERE id = $1
  AND user_id = $2
RETURNING *;

-- name: CompleteAvatarUploadSession :one
UPDATE avatar_upload_sessions
SET status = 'completed'
WHERE id = $1
  AND user_id = $2
RETURNING *;

-- name: ExpireAvatarUploadSession :execrows
UPDATE avatar_upload_sessions
SET status = 'expired'
WHERE id = $1
  AND user_id = $2;

-- name: UpdateUserAvatarAsset :execrows
UPDATE user_profiles
SET current_avatar_asset_id = $2
WHERE user_id = $1;
