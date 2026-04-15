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
