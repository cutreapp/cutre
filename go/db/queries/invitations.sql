-- name: CreateInvitation :one
INSERT INTO invitations (inviter_user_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetInvitationByToken :one
SELECT * FROM invitations WHERE token = $1 LIMIT 1;

-- name: GetInvitationByID :one
SELECT * FROM invitations WHERE id = $1 LIMIT 1;

-- name: GetInvitationByIDForShare :one
SELECT * FROM invitations WHERE id = $1 FOR SHARE;

-- name: GetInvitationByIDForUpdate :one
SELECT * FROM invitations WHERE id = $1 FOR UPDATE;

-- name: GetUnrevokedInvitationByInviterUserID :one
SELECT * FROM invitations WHERE inviter_user_id = $1 AND revoked_at IS NULL LIMIT 1;

-- name: CreateInvitationUnlessUnrevokedExists :one
-- 招待者が取り消していない招待を既に持つときは挿入せず、行を返さない。
-- 同じ招待者が同時に招待を作ろうとしたとき、部分ユニークインデックスの違反をエラーにせず、先に作られた招待に揃えるため。
INSERT INTO invitations (inviter_user_id, token, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (inviter_user_id) WHERE revoked_at IS NULL DO NOTHING
RETURNING *;

-- name: RevokeInvitation :exec
UPDATE invitations SET revoked_at = NOW(), updated_at = NOW() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeUnrevokedInvitationsByInviterUserID :exec
-- 退会した招待者の取り消していない招待を取り消す。
UPDATE invitations SET revoked_at = NOW(), updated_at = NOW() WHERE inviter_user_id = $1 AND revoked_at IS NULL;
