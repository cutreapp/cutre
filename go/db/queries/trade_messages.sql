-- name: CreateTradeMessage :one
INSERT INTO trade_messages (trade_id, sender_user_id, body)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateTradeMessageInProgress :one
-- 進行中 (返事待ち・マッチ成立) の交換にだけ、メッセージを記録する。
-- 交換の行を共有ロックで読んで段階を条件に含め、交換を終える操作と同時でも、終わった交換に送らない。
INSERT INTO trade_messages (trade_id, sender_user_id, body)
SELECT trades.id, sqlc.arg(sender_user_id), sqlc.arg(body)
FROM trades
WHERE trades.id = sqlc.arg(trade_id)
  AND trades.status IN ('pending', 'matched')
FOR SHARE
RETURNING *;

-- name: ListTradeMessagesByTradeID :many
-- 交換のメッセージ。送った順に並べる。
SELECT * FROM trade_messages
WHERE trade_id = $1
ORDER BY created_at, id;

-- name: GetTradeMessageByID :one
-- 交換のメッセージを引く。見てよい人かは呼び出し側が決める。
SELECT * FROM trade_messages WHERE id = $1 LIMIT 1;

-- name: ListLatestTradeMessagesByTradeIDs :many
-- 交換ごとの最新のメッセージ。取り消したものも含む。
SELECT DISTINCT ON (trade_id) * FROM trade_messages
WHERE trade_id = ANY(sqlc.arg(trade_ids)::uuid[])
ORDER BY trade_id, created_at DESC, id DESC;

-- name: CountUnreadTradeMessagesByTradeIDs :many
-- ユーザーの交換ごとの未読のメッセージの数。未読が無い交換は行を返さない。
-- 相手が送ったメッセージのうち、ユーザーが最後に読んだ位置 (送信時刻, ID) より後に並ぶものを数える。取り消したものは数えない。
SELECT trade_messages.trade_id, COUNT(*) AS unread_count
FROM trade_messages
LEFT JOIN trade_message_reads
  ON trade_message_reads.trade_id = trade_messages.trade_id
 AND trade_message_reads.user_id = sqlc.arg(user_id)
WHERE trade_messages.trade_id = ANY(sqlc.arg(trade_ids)::uuid[])
  AND trade_messages.sender_user_id <> sqlc.arg(user_id)
  AND trade_messages.retracted_at IS NULL
  AND (trade_message_reads.last_read_at IS NULL OR (trade_messages.created_at, trade_messages.id) > (trade_message_reads.last_read_at, trade_message_reads.last_read_message_id))
GROUP BY trade_messages.trade_id;

-- name: CountUnreadTradeMessagesByUserID :one
-- ユーザーの交換すべての、未読のメッセージの数。数え方は CountUnreadTradeMessagesByTradeIDs と揃える。
SELECT COUNT(*)
FROM trade_messages
JOIN trades ON trades.id = trade_messages.trade_id
LEFT JOIN trade_message_reads
  ON trade_message_reads.trade_id = trade_messages.trade_id
 AND trade_message_reads.user_id = sqlc.arg(user_id)
WHERE (trades.proposer_user_id = sqlc.arg(user_id) OR trades.receiver_user_id = sqlc.arg(user_id))
  AND trade_messages.sender_user_id <> sqlc.arg(user_id)
  AND trade_messages.retracted_at IS NULL
  AND (trade_message_reads.last_read_at IS NULL OR (trade_messages.created_at, trade_messages.id) > (trade_message_reads.last_read_at, trade_message_reads.last_read_message_id));

-- name: RetractTradeMessage :execrows
-- 送った人が、メッセージを取り消す。本文は消さずに残す。
-- 取り消していないことを条件に含め、取り消した時刻を最初の1回のまま保つ。
UPDATE trade_messages
SET retracted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND sender_user_id = $2
  AND retracted_at IS NULL;
