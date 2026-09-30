-- migrate:up

-- trade_messagesの (trade_id, id) を、trade_message_readsの複合外部キーから参照するために一意にする。
-- idが主キーのため制約としては冗長だが、読んだメッセージが同じ交換のものであることをDBで保証するために要る。
CREATE UNIQUE INDEX ON trade_messages (trade_id, id);

-- trade_message_readsは、交換の2人それぞれが、交換のメッセージをどこまで読んだかを持つ。
-- 相手のメッセージのうち、読んだ位置 (last_read_at, last_read_message_id) より後に並ぶものを未読として数える。
-- 交換のメッセージのページを開いたときに、そのとき出したメッセージのうち最新のものの位置まで進める。
--
-- trade_idの外部キーは、交換の行とともに消すためON DELETE CASCADEにする。
-- user_idの外部キーは、退会しても匿名化したusersの行とともに残すためON DELETE RESTRICTにする。
-- last_read_message_idは同じ送信時刻のメッセージを区別する。
-- 読んだメッセージが同じ交換のものであることを保証するため、(trade_id, last_read_message_id) の複合外部キーにする。
-- 参照するメッセージは交換の行とともに消えるため、ON DELETE CASCADEにする。
CREATE TABLE trade_message_reads (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    trade_id uuid NOT NULL REFERENCES trades (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    last_read_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_read_message_id uuid NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (trade_id, last_read_message_id) REFERENCES trade_messages (trade_id, id) ON DELETE CASCADE
);

-- 交換とユーザーの組で1行にする。外部キー (trade_id と、複合外部キーの先頭のtrade_id) のインデックスも兼ねる。
CREATE UNIQUE INDEX ON trade_message_reads (trade_id, user_id);
CREATE INDEX ON trade_message_reads (user_id);

-- migrate:down

DROP TABLE IF EXISTS trade_message_reads;
DROP INDEX IF EXISTS trade_messages_trade_id_id_idx;
