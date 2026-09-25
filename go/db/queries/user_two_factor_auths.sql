-- name: UpsertPendingUserTwoFactorAuth :one
-- 登録の途中の行を作るか、秘密鍵を差し替える。有効にした行は書き換えず、そのときは行を返さない。
INSERT INTO user_two_factor_auths (user_id, secret_ciphertext)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE
SET secret_ciphertext = EXCLUDED.secret_ciphertext, last_used_step = 0, updated_at = NOW()
WHERE user_two_factor_auths.enabled_at IS NULL
RETURNING *;

-- name: GetUserTwoFactorAuthByUserID :one
SELECT * FROM user_two_factor_auths WHERE user_id = $1 LIMIT 1;

-- name: EnableUserTwoFactorAuth :execrows
-- 登録の途中の行だけを有効にし、照合したコードのステップを使用済みとして記録する。
-- 照合後に秘密鍵が差し替わっていた場合は、新しい秘密鍵の行を有効にしない。
UPDATE user_two_factor_auths
SET enabled_at = NOW(), last_used_step = @step, updated_at = NOW()
WHERE user_id = @user_id AND secret_ciphertext = @expected_secret_ciphertext
  AND enabled_at IS NULL AND last_used_step < @step;

-- name: UpdateUserTwoFactorAuthLastUsedStep :execrows
-- 記録済みのステップより新しいときだけ更新し、更新できたかどうかでコードの使い回しを判定する。
UPDATE user_two_factor_auths
SET last_used_step = @step, updated_at = NOW()
WHERE user_id = @user_id AND enabled_at IS NOT NULL AND last_used_step < @step;

-- name: UseMatchingUserTwoFactorAuthStep :execrows
-- 照合した設定に限ってタイムステップを記録し、設定の入れ替わりとコードの使い回しを拒む。
UPDATE user_two_factor_auths
SET last_used_step = @step, updated_at = NOW()
WHERE user_id = @user_id AND id = @expected_id
  AND secret_ciphertext = @expected_secret_ciphertext
  AND enabled_at IS NOT NULL AND last_used_step < @step;

-- name: DeleteMatchingUserTwoFactorAuth :execrows
-- 再認証した設定だけを削除し、照合後に設定が入れ替わったときは削除しない。
DELETE FROM user_two_factor_auths
WHERE user_id = @user_id AND id = @expected_id
  AND secret_ciphertext = @expected_secret_ciphertext AND enabled_at IS NOT NULL;

-- name: DeleteUserTwoFactorAuthByUserID :exec
DELETE FROM user_two_factor_auths WHERE user_id = $1;
