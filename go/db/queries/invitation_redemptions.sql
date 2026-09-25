-- name: CreateInvitationRedemption :one
INSERT INTO invitation_redemptions (invitation_id, user_id)
VALUES ($1, $2)
RETURNING *;

-- name: CountInvitationRedemptionsByInvitationID :one
SELECT COUNT(*) FROM invitation_redemptions WHERE invitation_id = $1;

-- name: CountInvitationRedemptionsByInviterUserID :one
-- 取り消した招待や期限の切れた招待の使用も含めて、招待者のすべての招待の使用を数える。
SELECT COUNT(*)
FROM invitation_redemptions
INNER JOIN invitations ON invitations.id = invitation_redemptions.invitation_id
WHERE invitations.inviter_user_id = $1;

-- name: ListInvitationRedemptionsWithUsersByInviterUserID :many
-- 取り消した招待や期限の切れた招待の使用も含めて、招待者のすべての招待の使用を、登録した人と合わせて新しい順に返す。
-- 退会した人も匿名化したusersの行が残るため内部結合で引き、退会したかどうかは呼び出し側がdeleted_atで見分ける。
SELECT
    invitation_redemptions.*,
    users.atname AS user_atname,
    users.deleted_at AS user_deleted_at
FROM invitation_redemptions
INNER JOIN invitations ON invitations.id = invitation_redemptions.invitation_id
INNER JOIN users ON users.id = invitation_redemptions.user_id
WHERE invitations.inviter_user_id = $1
ORDER BY invitation_redemptions.created_at DESC, invitation_redemptions.id DESC;
