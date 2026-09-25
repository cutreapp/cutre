-- name: CreateEmailConfirmation :one
INSERT INTO email_confirmations (email, code, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUnconfirmedEmailConfirmationByID :one
SELECT * FROM email_confirmations
WHERE id = $1
  AND confirmed_at IS NULL
LIMIT 1;

-- name: VerifyEmailConfirmation :one
-- 入力されたコードを照合し、一致すれば確認済みに、違えば誤入力の回数を1つ増やす。
-- 照合と更新を1つの文で行い、行ロックで試行を直列化する。
-- 同時に送られた誤ったコードも1つずつ数えられ、上限を超えて照合されることはない。
-- 期限切れ・確認済み・上限に達した行は更新せず、行を返さない。
UPDATE email_confirmations
SET confirmed_at = CASE WHEN code = sqlc.arg(code)::text THEN NOW() END,
    failed_attempts_count = failed_attempts_count + CASE WHEN code = sqlc.arg(code)::text THEN 0 ELSE 1 END,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND confirmed_at IS NULL
  AND expires_at > NOW()
  AND failed_attempts_count < sqlc.arg(max_failed_attempts)::integer
RETURNING *;

-- name: GetConfirmedEmailConfirmationByID :one
SELECT * FROM email_confirmations
WHERE id = $1
  AND confirmed_at IS NOT NULL
LIMIT 1;

-- name: DeleteConfirmedEmailConfirmation :execrows
-- アカウントの作成に使った確認を消し、同じ確認で2つ目のアカウントを作れないようにする。
DELETE FROM email_confirmations
WHERE id = $1
  AND confirmed_at IS NOT NULL;

-- name: DeleteExpiredEmailConfirmations :exec
-- 指定した時刻までに有効期限が切れた確認を、確認済みかどうかによらず削除する。
DELETE FROM email_confirmations WHERE expires_at <= $1;

-- name: DeleteEmailConfirmationsByEmail :exec
-- 退会したユーザーのメールアドレスを残さないよう、そのアドレスへの確認を確認済みかどうかによらず削除する。
DELETE FROM email_confirmations WHERE email = $1;
