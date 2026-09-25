-- name: GetUserPasswordByUserID :one
SELECT * FROM user_passwords WHERE user_id = $1 LIMIT 1;

-- name: CreateUserPassword :one
INSERT INTO user_passwords (user_id, password_digest)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateUserPasswordDigest :execrows
UPDATE user_passwords
SET password_digest = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: DeleteUserPasswordByUserID :exec
DELETE FROM user_passwords WHERE user_id = $1;
