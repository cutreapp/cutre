-- name: GetLiveUserSessionWithUserByTokenDigest :one
-- ログイン中のセッションと、その持ち主のユーザーを1度のJOINで引く。
-- リクエストごとに通る経路のため、セッションとユーザーで2往復しないようにする。
-- 期限切れのセッションと退会したユーザーはここで落とし、呼び出し側には未ログインとして見せる。
SELECT sqlc.embed(user_sessions), sqlc.embed(users)
FROM user_sessions
INNER JOIN users ON users.id = user_sessions.user_id
WHERE user_sessions.token_digest = $1
  AND user_sessions.expires_at > NOW()
  AND users.deleted_at IS NULL
LIMIT 1;

-- name: CreateUserSession :one
INSERT INTO user_sessions (user_id, token_digest, expires_at, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ExtendUserSession :exec
-- 有効期限と最終利用時刻を進める。
UPDATE user_sessions
SET expires_at = $2, last_seen_at = $3, updated_at = NOW()
WHERE id = $1;

-- name: DeleteUserSessionByTokenDigest :exec
DELETE FROM user_sessions WHERE token_digest = $1;

-- name: DeleteUserSessionsByUserID :exec
DELETE FROM user_sessions WHERE user_id = $1;

-- name: DeleteExpiredUserSessions :exec
-- 指定した時刻までに有効期限が切れたセッションを削除する。
DELETE FROM user_sessions WHERE expires_at <= $1;
