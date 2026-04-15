-- Official page readmodel

-- name: GetOfficialHomeDay :one
SELECT
  dka.biz_date,
  ok.id AS keyword_id,
  ok.text,
  ok.category,
  ok.is_active,
  ok.display_order,
  COALESCE(dks.participant_user_count, 0) AS participant_user_count
FROM daily_keyword_assignments dka
JOIN official_keywords ok ON ok.id = dka.keyword_id
LEFT JOIN daily_keyword_stats dks ON dks.biz_date = dka.biz_date
WHERE dka.biz_date = $1;
