-- name: CreateUserTwoFactorRecoveryCode :exec
INSERT INTO user_two_factor_recovery_codes (user_id, code_digest)
VALUES ($1, $2);

-- name: CountUnusedUserTwoFactorRecoveryCodes :one
SELECT COUNT(*) FROM user_two_factor_recovery_codes WHERE user_id = $1 AND used_at IS NULL;

-- name: UseUserTwoFactorRecoveryCode :execrows
-- 未使用のコードだけを使用済みにし、更新できたかどうかでコードを使えたかを判定する。
UPDATE user_two_factor_recovery_codes
SET used_at = NOW(), updated_at = NOW()
WHERE user_id = $1 AND code_digest = $2 AND used_at IS NULL;

-- name: DeleteUserTwoFactorRecoveryCodesByUserID :exec
DELETE FROM user_two_factor_recovery_codes WHERE user_id = $1;
