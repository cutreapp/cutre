-- name: CreateMessageConsent :one
INSERT INTO message_consents (user_id, version, agreed_at)
VALUES ($1, $2, NOW())
RETURNING *;

-- name: GetLatestMessageConsentByUserID :one
-- 同じ時刻の行が並んだときも1行に決まるよう、UUIDv7のidで並べ直す。
SELECT * FROM message_consents
WHERE user_id = $1
ORDER BY agreed_at DESC, id DESC
LIMIT 1;

-- name: WithdrawMessageConsentsByUserID :execrows
-- やめていない同意をすべてやめる。版が古い同意も含め、有効な同意が残らないようにする。
UPDATE message_consents
SET withdrawn_at = NOW(), updated_at = NOW()
WHERE user_id = $1 AND withdrawn_at IS NULL;
