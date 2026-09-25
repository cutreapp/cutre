-- migrate:up

-- password_reset_tokensは、パスワードリセットのメールに載せるリンクの使い捨てトークンを持つ。
-- 1行が1通のリセットのメールで、平文のトークンはメールの中にしか無い。
--
-- token_digestに平文ではなくSHA-256のダイジェストを入れる理由はuser_sessionsと同じで、
-- データベースの内容が漏れても、そのままパスワードを変えられるトークンが露出しないようにするため。
-- expires_atはリンクが使える期限。
--
-- 使ったトークンは行ごと消すため、使用済みを表す列は持たない。
-- 新しいリセットを申請したときも、そのユーザーの古いトークンを消してから作る。
-- 1人のユーザーが持つ行は常に高々1行になるため、期限切れの行を定期的に消すジョブは要らない。
--
-- user_idの外部キーをON DELETE CASCADEにするのは、トークンがユーザーに従属するデータのため。
-- user_idの一意制約は1人のユーザーに残るトークンを高々1行に保ち、
-- ユーザー単位の削除とカスケードの削除にも使う。
CREATE TABLE password_reset_tokens (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_digest VARCHAR NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (token_digest),
    UNIQUE (user_id)
);

-- migrate:down

DROP TABLE IF EXISTS password_reset_tokens;
