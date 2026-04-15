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
