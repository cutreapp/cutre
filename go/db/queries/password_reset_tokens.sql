-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (user_id, token_digest, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeletePasswordResetTokensByUserID :exec
DELETE FROM password_reset_tokens WHERE user_id = $1;

-- name: GetLivePasswordResetTokenByTokenDigest :one
SELECT * FROM password_reset_tokens WHERE token_digest = $1 AND expires_at > NOW() LIMIT 1;

-- name: GetLivePasswordResetTokenByID :one
SELECT * FROM password_reset_tokens WHERE id = $1 AND expires_at > NOW() LIMIT 1;

-- name: DeleteLivePasswordResetToken :execrows
-- 期限内のトークンだけを消し、消せたかどうかでトークンを使えたかを判定する。
DELETE FROM password_reset_tokens WHERE id = $1 AND expires_at > NOW();
