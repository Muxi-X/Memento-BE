-- name: InsertAnalyticsEvent :exec
INSERT INTO analytics_events (
  user_id,
  event_id,
  event_name,
  kind,
  source,
  schema_version,
  occurred_at,
  selection_state,
  context_biz_date,
  keyword_id,
  received_at,
  biz_date,
  request_id
) VALUES (
  sqlc.arg(user_id)::uuid,
  sqlc.arg(event_id)::uuid,
  sqlc.arg(event_name)::text,
  (sqlc.narg(kind)::text)::prompt_kind,
  sqlc.arg(source)::text,
  sqlc.arg(schema_version)::smallint,
  sqlc.arg(occurred_at)::timestamptz,
  sqlc.arg(selection_state)::text,
  sqlc.narg(context_biz_date)::date,
  sqlc.narg(keyword_id)::uuid,
  sqlc.arg(received_at)::timestamptz,
  sqlc.arg(biz_date)::date,
  sqlc.arg(request_id)::text
)
ON CONFLICT (user_id, event_id) DO NOTHING;

-- name: GetAnalyticsClickMetrics :many
SELECT
  'prompt_entry_click'::text AS event_name,
  ''::text AS kind,
  COUNT(*)::bigint AS click_count,
  COUNT(DISTINCT user_id)::bigint AS unique_user_count
FROM analytics_events
WHERE biz_date = sqlc.arg(biz_date)::date
  AND event_name = 'prompt_entry_click'
UNION ALL
SELECT
  'prompt_kind_entry_click'::text AS event_name,
  'intuition'::text AS kind,
  COUNT(*)::bigint AS click_count,
  COUNT(DISTINCT user_id)::bigint AS unique_user_count
FROM analytics_events
WHERE biz_date = sqlc.arg(biz_date)::date
  AND event_name = 'prompt_kind_entry_click'
  AND kind = 'intuition'
UNION ALL
SELECT
  'prompt_kind_entry_click'::text AS event_name,
  'structure'::text AS kind,
  COUNT(*)::bigint AS click_count,
  COUNT(DISTINCT user_id)::bigint AS unique_user_count
FROM analytics_events
WHERE biz_date = sqlc.arg(biz_date)::date
  AND event_name = 'prompt_kind_entry_click'
  AND kind = 'structure'
UNION ALL
SELECT
  'prompt_kind_entry_click'::text AS event_name,
  'concept'::text AS kind,
  COUNT(*)::bigint AS click_count,
  COUNT(DISTINCT user_id)::bigint AS unique_user_count
FROM analytics_events
WHERE biz_date = sqlc.arg(biz_date)::date
  AND event_name = 'prompt_kind_entry_click'
  AND kind = 'concept'
ORDER BY event_name ASC, kind ASC;

-- name: GetDailyPromptSuccessCount :one
SELECT COUNT(*)::bigint
FROM user_daily_prompts
WHERE biz_date = sqlc.arg(biz_date)::date;
