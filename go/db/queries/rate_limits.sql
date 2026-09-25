-- name: IncrementRateLimit :one
-- 指定したキーと時間枠のカウンターを1つ増やし、増やしたあとの行を返す。
-- 行が無ければ1で作る。UPSERTにすることで、同時に届いた試行の加算を取りこぼさない。
INSERT INTO rate_limits (key, window_start)
VALUES ($1, $2)
ON CONFLICT (key, window_start)
DO UPDATE SET count = rate_limits.count + 1, updated_at = NOW()
RETURNING *;

-- name: DeleteRateLimitsOlderThan :exec
-- 指定した時刻より前に始まった時間枠の行を削除する。
DELETE FROM rate_limits WHERE window_start < $1;
