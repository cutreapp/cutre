-- migrate:up

-- trade_messagesは、交換の2人が交換ごとに送り合うメッセージを持つ。申し込みのひとことが1通目になる。
--
-- retracted_atは送った人が取り消した時刻。取り消しても本文は消さずに残す。
-- 利用者から問題の報告を受けたときに、運営が取り消したものを含めて確認できるようにするため。
-- 取り消したメッセージの本文は画面に出さない。
--
-- trade_idの外部キーは、交換の行とともに消すためON DELETE CASCADEにする。
-- sender_user_idの外部キーは、退会しても匿名化したusersの行とともに残すためON DELETE RESTRICTにする。
CREATE TABLE trade_messages (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    trade_id uuid NOT NULL REFERENCES trades (id) ON DELETE CASCADE,
    sender_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    body VARCHAR NOT NULL,
    retracted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 交換のメッセージを時刻順に引くためのインデックス。外部キーのインデックスも兼ねる。
CREATE INDEX ON trade_messages (trade_id, created_at);
CREATE INDEX ON trade_messages (sender_user_id);

-- migrate:down

DROP TABLE IF EXISTS trade_messages;
