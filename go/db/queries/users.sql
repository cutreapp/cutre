-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: LockUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByAtname :one
SELECT * FROM users WHERE atname = $1 AND deleted_at IS NULL LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (email, atname, locale, time_zone)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: WithdrawUser :execrows
-- 退会した時刻を入れ、メールアドレスとアットネームを匿名の値に置き換える。
-- ロケールとタイムゾーンも全員共通の値にし、退会前の属性を残さない。
-- 退会済みの行は更新せず、同じユーザーの退会が重なっても2度目は0行になる。
UPDATE users
SET deleted_at = NOW(),
    email = $2,
    atname = $3,
    locale = 'ja',
    time_zone = 'Etc/UTC',
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;
