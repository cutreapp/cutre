-- name: CreateTrade :one
-- 申し込みで、返事待ち (pending) の交換を作る。
INSERT INTO trades (proposer_user_id, receiver_user_id)
VALUES ($1, $2)
RETURNING *;

-- name: ExistsInProgressTradeByUserID :one
-- ユーザーが申し込んだか申し込まれた、進行中 (返事待ち・マッチ成立) の交換があるか。
SELECT EXISTS (
    SELECT 1 FROM trades
    WHERE (proposer_user_id = $1 OR receiver_user_id = $1)
      AND status IN ('pending', 'matched')
);

-- name: CountInProgressTradesByUserID :one
-- ユーザーが申し込んだか申し込まれた、進行中 (返事待ち・マッチ成立) の交換の数。
SELECT COUNT(*) FROM trades
WHERE (proposer_user_id = $1 OR receiver_user_id = $1)
  AND status IN ('pending', 'matched');

-- name: GetTradeByID :one
-- 交換を引く。交換の2人以外に見せないかは呼び出し側が決める。
SELECT * FROM trades WHERE id = $1 LIMIT 1;

-- name: ListInProgressTradesByUserID :many
-- ユーザーが申し込んだか申し込まれた、進行中 (返事待ち・マッチ成立) の交換。新しく申し込まれたものから並べる。
SELECT * FROM trades
WHERE (proposer_user_id = sqlc.arg(user_id) OR receiver_user_id = sqlc.arg(user_id))
  AND status IN ('pending', 'matched')
ORDER BY created_at DESC, id DESC;

-- name: CountAwaitingTradesByUserID :one
-- ユーザーの返事や確認を待っている交換の数。
-- 申し込まれて返事をしていない交換と、マッチ成立のあとに相手だけが「交換できた」を押した交換を数える。
SELECT COUNT(*) FROM trades
WHERE (receiver_user_id = sqlc.arg(user_id) AND status = 'pending')
   OR (proposer_user_id = sqlc.arg(user_id) AND status = 'matched' AND proposer_completed_at IS NULL AND receiver_completed_at IS NOT NULL)
   OR (receiver_user_id = sqlc.arg(user_id) AND status = 'matched' AND receiver_completed_at IS NULL AND proposer_completed_at IS NOT NULL);

-- name: WithdrawTrade :execrows
-- 申し込んだ人が、返事待ちの交換を取り下げる。
-- 返事待ちであることを条件に含め、承認やお断りと同時に押されても二重に進めない。
UPDATE trades
SET status = 'withdrawn',
    ended_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND proposer_user_id = $2
  AND status = 'pending';

-- name: ApproveTrade :execrows
-- 申し込まれた人が、返事待ちの交換を承認し、マッチ成立にする。
-- 返事待ちであることを条件に含め、取り下げやお断りと同時に押されても二重に進めない。
UPDATE trades
SET status = 'matched',
    matched_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND receiver_user_id = $2
  AND status = 'pending';

-- name: DeclineTrade :execrows
-- 申し込まれた人が、返事待ちの交換をお断りする。
-- 返事待ちであることを条件に含め、取り下げや承認と同時に押されても二重に進めない。
UPDATE trades
SET status = 'declined',
    ended_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND receiver_user_id = $2
  AND status = 'pending';

-- name: ListTradesByUserIDOrderByLatestMessage :many
-- ユーザーが申し込んだか申し込まれた交換すべて (終わったものを含む)。
-- 最新のメッセージが新しいものから並べ、メッセージの無い交換は申し込まれた時刻で並べる。
SELECT trades.* FROM trades
LEFT JOIN LATERAL (
    SELECT MAX(trade_messages.created_at) AS latest_message_at
    FROM trade_messages
    WHERE trade_messages.trade_id = trades.id
) AS latest ON TRUE
WHERE trades.proposer_user_id = sqlc.arg(user_id) OR trades.receiver_user_id = sqlc.arg(user_id)
ORDER BY COALESCE(latest.latest_message_at, trades.created_at) DESC, trades.id DESC;

-- name: CompleteTrade :one
-- 交換の2人のうち user_id が、マッチ成立の交換で「交換できた」を押す。
-- 相手がすでに押していれば、同じ更新で交換を「交換できた」の段階で終える。
-- マッチ成立で、まだ押していないことを条件に含め、2人が同時に押しても、あとの更新が先に押された時刻を見て1回だけ終える。
-- 更新の条件で自分の時刻が空であることを確かめているため、SETで見る時刻のどちらかが入っていれば、それは相手が押した時刻になる。
UPDATE trades
SET proposer_completed_at = CASE WHEN proposer_user_id = sqlc.arg(user_id) THEN NOW() ELSE proposer_completed_at END,
    receiver_completed_at = CASE WHEN receiver_user_id = sqlc.arg(user_id) THEN NOW() ELSE receiver_completed_at END,
    status = CASE WHEN proposer_completed_at IS NOT NULL OR receiver_completed_at IS NOT NULL THEN 'completed'::trade_status ELSE status END,
    ended_at = CASE WHEN proposer_completed_at IS NOT NULL OR receiver_completed_at IS NOT NULL THEN NOW() ELSE ended_at END,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = 'matched'
  AND ((proposer_user_id = sqlc.arg(user_id) AND proposer_completed_at IS NULL)
    OR (receiver_user_id = sqlc.arg(user_id) AND receiver_completed_at IS NULL))
RETURNING *;

-- name: FailTrade :execrows
-- 交換の2人のうち user_id が、マッチ成立の交換を「交換できなかった」の段階で終える。
-- どちらかが「交換できた」を押していても記録できる。マッチ成立であることを条件に含め、2人が同時に押しても、2人そろって「交換できた」になるのと同時に押されても二重に進めない。
UPDATE trades
SET status = 'failed',
    ended_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND (proposer_user_id = sqlc.arg(user_id) OR receiver_user_id = sqlc.arg(user_id))
  AND status = 'matched';

-- name: CancelTrade :execrows
-- 交換の2人のうち user_id が、マッチ成立の交換をやめる。
-- マッチ成立で、どちらも「交換できた」を押していないことを条件に含め、「交換できた」と同時に押されても、押したあとの交換をやめない。
UPDATE trades
SET status = 'cancelled',
    ended_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND (proposer_user_id = sqlc.arg(user_id) OR receiver_user_id = sqlc.arg(user_id))
  AND status = 'matched'
  AND proposer_completed_at IS NULL
  AND receiver_completed_at IS NULL;

-- name: ListEndedTradesByUserID :many
-- ユーザーが申し込んだか申し込まれた、終わった (取り下げ・お断り・交換できた・交換できなかった・やめた) 交換。
-- 終わった時刻が新しいものから並べる。
SELECT * FROM trades
WHERE (proposer_user_id = sqlc.arg(user_id) OR receiver_user_id = sqlc.arg(user_id))
  AND status IN ('withdrawn', 'declined', 'completed', 'failed', 'cancelled')
ORDER BY ended_at DESC, id DESC;

-- name: CountEndedTradesByUserIDGroupByStatus :many
-- ユーザーが申し込んだか申し込まれた、終わった交換の数を段階ごとに数える。1件も無い段階の行は返さない。
SELECT status, COUNT(*) AS count FROM trades
WHERE (proposer_user_id = sqlc.arg(user_id) OR receiver_user_id = sqlc.arg(user_id))
  AND status IN ('withdrawn', 'declined', 'completed', 'failed', 'cancelled')
GROUP BY status;
