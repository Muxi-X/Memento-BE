-- Prompts

-- name: DrawRandomPrompt :one
SELECT *
FROM official_keyword_prompts
WHERE keyword_id = $1
  AND kind = $2
  AND is_active = true
  AND EXISTS (
    SELECT 1
    FROM official_keywords ok
    WHERE ok.id = official_keyword_prompts.keyword_id
      AND ok.is_active = true
  )
ORDER BY random()
LIMIT 1;

-- name: LockUser :one
SELECT id
FROM users
WHERE id = $1
FOR UPDATE;

-- name: GetDailyPrompt :one
SELECT *
FROM user_daily_prompts
WHERE user_id = $1
  AND biz_date = $2;

-- name: InsertDailyPrompt :one
INSERT INTO user_daily_prompts (
  user_id, biz_date, keyword_id, prompt_id, kind, content_snapshot, selected_at
) VALUES (
  sqlc.arg(user_id)::uuid,
  sqlc.arg(biz_date)::date,
  sqlc.arg(keyword_id)::uuid,
  sqlc.arg(prompt_id)::uuid,
  sqlc.arg(kind)::prompt_kind,
  sqlc.arg(content_snapshot)::text,
  sqlc.arg(selected_at)::timestamptz
)
ON CONFLICT (user_id, biz_date) DO NOTHING
RETURNING *;

-- name: UpsertPrompt :one
INSERT INTO official_keyword_prompts (id, keyword_id, kind, content, display_order, is_active)
VALUES (
  sqlc.arg(id)::uuid,
  sqlc.arg(keyword_id)::uuid,
  sqlc.arg(kind)::prompt_kind,
  sqlc.arg(content)::varchar,
  COALESCE(sqlc.narg(display_order)::int, (SELECT COALESCE(MAX(okp.display_order), 0) + 1 FROM official_keyword_prompts okp WHERE okp.keyword_id = sqlc.arg(keyword_id)::uuid)),
  sqlc.arg(is_active)::boolean
)
ON CONFLICT (id) DO UPDATE
SET keyword_id = EXCLUDED.keyword_id,
    kind = EXCLUDED.kind,
    content = EXCLUDED.content,
    display_order = EXCLUDED.display_order,
    is_active = EXCLUDED.is_active
RETURNING *;

-- name: ListPromptsByKeyword :many
SELECT *
FROM official_keyword_prompts
WHERE keyword_id = $1
ORDER BY kind ASC, display_order ASC, id ASC;
