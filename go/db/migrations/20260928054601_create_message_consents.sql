-- migrate:up

-- message_consentsは、メッセージの取り扱いへの同意の記録を持つ。1行が「この版の文面に、この日時に同意した」を表す。
-- 同意をやめたら行を消さずにwithdrawn_atを入れ、もう一度同意したら新しい行を作る。
-- 有効な同意は、ユーザーの最新の行が今の版で、withdrawn_atが空のときに限る。
--
-- versionは同意した文面の版で、文面と対にしてコードの定数で持つ。文面を変えたら版を上げる。
--
-- user_idの外部キーは、退会しても同意の記録を残すためON DELETE RESTRICTにして、
-- 匿名化して残すはずのusersの行を誤って物理削除できないようにする。
CREATE TABLE message_consents (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    version INTEGER NOT NULL,
    agreed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    withdrawn_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- ユーザーの最新の同意を引くためのインデックス。
CREATE INDEX ON message_consents (user_id, agreed_at);

-- migrate:down

DROP TABLE IF EXISTS message_consents;
