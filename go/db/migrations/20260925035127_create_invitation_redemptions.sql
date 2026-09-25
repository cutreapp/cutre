-- migrate:up

-- invitation_redemptionsは招待の使用の記録を持つ。1行が「この招待でこのユーザーが登録した」を表す。
-- 1つの招待を複数人が使えるため、使用の記録を招待の行から分ける。
--
-- user_idのUNIQUEにより、1人が複数の招待で登録した記録を作らない。
-- どちらの外部キーも退会後に招待の経路を辿るために残す記録のため、ON DELETE RESTRICTにして、
-- 匿名化して残すはずのusersの行と、記録の元になった招待を誤って物理削除できないようにする。
CREATE TABLE invitation_redemptions (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    invitation_id uuid NOT NULL REFERENCES invitations (id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (user_id)
);

-- 招待ごと・招待者ごとに使用の件数を数えるときに、invitationsと結合するためのインデックス。
CREATE INDEX ON invitation_redemptions (invitation_id);

INSERT INTO invitation_redemptions (invitation_id, user_id, created_at, updated_at)
SELECT id, used_by_user_id, used_at, used_at
FROM invitations
WHERE used_by_user_id IS NOT NULL;

ALTER TABLE invitations
    DROP CONSTRAINT invitations_check,
    DROP COLUMN used_by_user_id,
    DROP COLUMN used_at;

-- 招待者ごとに使用の件数を数えるときに、取り消した招待も含めて招待者の招待を引くためのインデックス。
CREATE INDEX ON invitations (inviter_user_id);

-- 取り消していない招待を招待者ごとに1本に限る。
-- 招待者の無い (管理者が発行した) 招待はNULL同士が重複とみなされないため、何本でも持てる。
-- アカウントの作成は使う招待の行をロックして招待者単位に直列化しており、この制約がその前提になる。
CREATE UNIQUE INDEX invitations_inviter_user_id_unrevoked_key ON invitations (inviter_user_id) WHERE revoked_at IS NULL;

-- migrate:down

DROP INDEX IF EXISTS invitations_inviter_user_id_unrevoked_key;
DROP INDEX IF EXISTS invitations_inviter_user_id_idx;

ALTER TABLE invitations
    ADD COLUMN used_by_user_id uuid REFERENCES users (id) ON DELETE RESTRICT,
    ADD COLUMN used_at TIMESTAMP WITH TIME ZONE,
    ADD CONSTRAINT invitations_used_by_user_id_key UNIQUE (used_by_user_id),
    ADD CONSTRAINT invitations_check CHECK ((used_by_user_id IS NULL) = (used_at IS NULL));

-- 1つの招待に使用の記録が複数あるときは、戻す先の列が1組しか無いため、最初の記録だけを戻す。
UPDATE invitations
SET used_by_user_id = first_redemptions.user_id,
    used_at = first_redemptions.created_at
FROM (
    SELECT DISTINCT ON (invitation_id) invitation_id, user_id, created_at
    FROM invitation_redemptions
    ORDER BY invitation_id, created_at
) AS first_redemptions
WHERE invitations.id = first_redemptions.invitation_id;

DROP TABLE IF EXISTS invitation_redemptions;
