-- migrate:up

-- invitationsは招待リンクを持つ。1行が1回だけ使える招待で、Cutreに登録するにはこれが要る。
--
-- inviter_user_idは招待したユーザーで、NULLは管理者がCLIから発行した招待を表す。
-- used_by_user_idは招待で登録したユーザーで、UNIQUEにより1人が複数の招待を消費した記録を作らない。
-- どちらも退会後に招待の経路を辿るために残す記録のため、外部キーはON DELETE RESTRICTにして、
-- 匿名化して残すはずのusersの行を誤って物理削除できないようにする。
--
-- tokenは招待リンク (/i/{token}) に載せる値で、発行した後もURLとQRコードを再表示できるよう平文で持つ。
-- 盗まれても登録できるだけで、招待者の記録・期限・取り消しで被害を抑えられる。
--
-- 招待が使える状態 (未使用・期限内・未取り消し) かは、used_at・expires_at・revoked_atの組み合わせで決まる。
-- used_by_user_idとused_atは登録と同時に埋まるため、片方だけが埋まった行をCHECKで作れないようにする。
CREATE TABLE invitations (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    inviter_user_id uuid REFERENCES users (id) ON DELETE RESTRICT,
    token VARCHAR NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_by_user_id uuid REFERENCES users (id) ON DELETE RESTRICT,
    used_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (token),
    UNIQUE (used_by_user_id),
    CHECK ((used_by_user_id IS NULL) = (used_at IS NULL))
);

-- migrate:down

DROP TABLE IF EXISTS invitations;
