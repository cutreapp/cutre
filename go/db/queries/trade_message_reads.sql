-- name: GetTradeMessageReadByTradeIDAndUserID :one
-- ユーザーが交換のメッセージをどこまで読んだか。
SELECT * FROM trade_message_reads
WHERE trade_id = $1
  AND user_id = $2
LIMIT 1;

-- name: UpsertTradeMessageRead :exec
-- ユーザーが交換のメッセージを、(送信時刻, ID) の位置まで読んだことを記録する。
-- 古いページを開き直したときなどに戻さないよう、読んだ位置は進めるだけにする。
INSERT INTO trade_message_reads (trade_id, user_id, last_read_at, last_read_message_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (trade_id, user_id) DO UPDATE
SET last_read_at = EXCLUDED.last_read_at,
    last_read_message_id = EXCLUDED.last_read_message_id,
    updated_at = NOW()
WHERE (trade_message_reads.last_read_at, trade_message_reads.last_read_message_id)
    < (EXCLUDED.last_read_at, EXCLUDED.last_read_message_id);
