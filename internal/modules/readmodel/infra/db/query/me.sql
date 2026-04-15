-- Me page readmodel

-- name: GetMeHomeSummary :one
SELECT
  up.user_id,
  up.nickname,
  ma.original_object_key AS avatar_object_key,
  COALESCE((
    SELECT COUNT(wui.id)::int
    FROM work_uploads wu
    JOIN work_upload_images wui
      ON wui.upload_id = wu.id
     AND wui.deleted_at IS NULL
    WHERE wu.author_user_id = up.user_id
      AND wu.context_type = 'official_today'
      AND wu.deleted_at IS NULL
  ), 0)::int AS official_image_count,
  COALESCE((
    SELECT COUNT(wui.id)::int
    FROM work_uploads wu
    JOIN work_upload_images wui
      ON wui.upload_id = wu.id
     AND wui.deleted_at IS NULL
    JOIN custom_keywords ck
      ON ck.id = wu.custom_keyword_id
     AND ck.owner_user_id = up.user_id
     AND ck.deleted_at IS NULL
    WHERE wu.author_user_id = up.user_id
      AND wu.context_type = 'custom_keyword'
      AND wu.deleted_at IS NULL
  ), 0)::int AS custom_image_count,
  COALESCE((
    SELECT COUNT(*)::int
    FROM notifications n
    WHERE n.recipient_user_id = up.user_id
      AND n.read_at IS NULL
  ), 0)::int AS unread_notification_count
FROM user_profiles up
LEFT JOIN media_assets ma
  ON ma.id = up.current_avatar_asset_id
 AND ma.deleted_at IS NULL
WHERE up.user_id = $1;

-- name: ListMeHomeCustomKeywords :many
WITH keyword_stats AS (
  SELECT
    wu.custom_keyword_id,
    COUNT(wui.id)::int AS image_count
  FROM work_uploads wu
  JOIN work_upload_images wui
    ON wui.upload_id = wu.id
   AND wui.deleted_at IS NULL
  JOIN custom_keywords owned
    ON owned.id = wu.custom_keyword_id
   AND owned.owner_user_id = $1
   AND owned.deleted_at IS NULL
  WHERE wu.author_user_id = $1
    AND wu.context_type = 'custom_keyword'
    AND wu.deleted_at IS NULL
    AND wu.custom_keyword_id IS NOT NULL
  GROUP BY wu.custom_keyword_id
)
SELECT
  ck.id,
  ck.text,
  NULLIF(ck.target_image_count, 0)::int AS target_image_count,
  COALESCE(ks.image_count, 0)::int AS total_image_count,
  COALESCE(ks.image_count, 0)::int AS my_image_count,
  cover_asset.id AS cover_image_id,
  cover_asset.original_object_key AS cover_object_key
FROM custom_keywords ck
LEFT JOIN keyword_stats ks
  ON ks.custom_keyword_id = ck.id
LEFT JOIN media_assets cover_asset
  ON cover_asset.id = ck.cover_asset_id
 AND cover_asset.deleted_at IS NULL
WHERE ck.owner_user_id = $1
  AND ck.deleted_at IS NULL
ORDER BY ck.created_at DESC, ck.id DESC;
