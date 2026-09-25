-- migrate:up

-- email_confirmationsは、登録するメールアドレスを持っていることを確かめる確認コードを持つ。
-- 1行が1回の送信で、登録の途中でまだユーザーがいないため、ユーザーではなくメールアドレスに紐づける。
-- emailはusers.emailと同じく大文字小文字を区別しないcitextにする。
--
-- codeは6桁の数字で、平文で持つ。桁数が少なくハッシュにしても総当たりで戻せるため、保護にならない。
-- 代わりに有効期限 (expires_at) と、誤ったコードの入力回数 (failed_attempts_count) で推測を抑える。
-- confirmed_atはコードが一致した時刻で、確認するまではNULL。
--
-- 登録済みのメールアドレスで登録を始めたときも行を作る (コードは送らない)。
-- 画面の挙動を未登録のときと揃え、登録の有無を画面から推測できないようにするため。
CREATE TABLE email_confirmations (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    email public.citext NOT NULL,
    code VARCHAR NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    failed_attempts_count INTEGER NOT NULL DEFAULT 0,
    confirmed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- migrate:down

DROP TABLE IF EXISTS email_confirmations;
