-- migrate:up

-- user_sessionsはCookieに紐づくログイン中のセッションを持つ。
-- 1行が1つのセッションで、Cookieが運ぶトークンのダイジェストで引く。
--
-- token_digestに平文ではなくSHA-256のダイジェストを入れるのは、データベースの内容が
-- 漏れてもそのままログインに使えるトークンが露出しないようにするため。
-- トークンは高エントロピーの乱数のため、ソルトや低速ハッシュは要らず、
-- 完全一致で引ける決定的なダイジェストが適する。
--
-- expires_atはサーバー側の有効期限で、アクセスのたびに延長する。
-- Cookieの寿命だけに任せると、利用者の手元で期限を伸ばせてしまう。
-- last_seen_atは最後にセッションを使った時刻で、延長のUPDATEを間引く判定に使う。
-- signed_in_atはログインした時刻で、新規作成の行ではcreated_atと一致する。
--
-- user_idの外部キーをON DELETE CASCADEにするのは、セッションが独立したライフサイクルを
-- 持たない従属データであり、ユーザーの行と一緒に消えるべきものであるため。
-- 同じ列のインデックスは、ユーザー単位でセッションを引いて失効させる操作
-- (退会・全端末からのログアウト) とカスケードの削除が全表走査にならないようにする。
CREATE TABLE user_sessions (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_digest VARCHAR NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    ip_address VARCHAR NOT NULL,
    user_agent VARCHAR NOT NULL,
    signed_in_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (token_digest)
);

CREATE INDEX ON user_sessions (user_id);

-- migrate:down

DROP TABLE IF EXISTS user_sessions;
