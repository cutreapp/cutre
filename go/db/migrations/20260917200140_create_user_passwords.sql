-- migrate:up

-- user_passwordsはパスワード認証の資格情報を持つ。
-- usersから切り離すのは、身元と認証手段を分けて、認証手段が増えても
-- usersの形が変わらないようにするため。
-- password_digestはbcryptハッシュで、平文は保存しない。
--
-- user_idにUNIQUEを張り、1ユーザーが持つパスワードを高々1つにする。
-- 外部キーをON DELETE CASCADEにするのは、パスワードが独立したライフサイクルを持たない
-- 従属データであり、ユーザーの行と一緒に消えるべきものであるため。
CREATE TABLE user_passwords (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    password_digest VARCHAR NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (user_id)
);

-- migrate:down

DROP TABLE IF EXISTS user_passwords;
