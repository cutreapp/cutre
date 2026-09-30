-- name: CreateTradeEvent :one
INSERT INTO trade_events (trade_id, actor_user_id, kind, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListTradeEventsByTradeID :many
-- 交換のページの「これまでの流れ」に出す、交換の出来事。起きた順に並べる。
SELECT * FROM trade_events
WHERE trade_id = $1
ORDER BY created_at, id;
