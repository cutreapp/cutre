-- migrate:up

-- trade_statusは交換の段階。返事待ち (pending) から、取り下げ (withdrawn)・お断り (declined)・
-- マッチ成立 (matched) のいずれかへ進み、マッチ成立から交換できた (completed)・交換できなかった (failed)・
-- やめた (cancelled) のいずれかで終わる。
-- 片方だけが「交換できた」を押した状態は段階を増やさず、*_completed_at の片方だけが入っていることで表す。
CREATE TYPE trade_status AS ENUM ('pending', 'withdrawn', 'declined', 'matched', 'completed', 'failed', 'cancelled');

-- tradesは、ユーザー (申し込んだ人) がほかのユーザー (申し込まれた人) に申し込んだ交換を持つ。
-- 交換の品は trade_items、これまでの流れは trade_events、2人のやり取りは trade_messages が持つ。
--
-- proposer_completed_at・receiver_completed_at は、それぞれが「交換できた」を押した時刻。
-- matched_at は承認した時刻、ended_at は交換が終わった (取り下げ・お断りを含む) 時刻。
--
-- usersへの外部キーは、退会しても匿名化したusersの行とともに交換の記録を残すためON DELETE RESTRICTにする。
CREATE TABLE trades (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    proposer_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    receiver_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status trade_status NOT NULL DEFAULT 'pending',
    proposer_completed_at TIMESTAMP WITH TIME ZONE,
    receiver_completed_at TIMESTAMP WITH TIME ZONE,
    matched_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (proposer_user_id <> receiver_user_id)
);

-- ユーザーが申し込んだ・申し込まれた交換を、段階で絞って引くためのインデックス。外部キーのインデックスも兼ねる。
CREATE INDEX ON trades (proposer_user_id, status);
CREATE INDEX ON trades (receiver_user_id, status);

-- migrate:down

DROP TABLE IF EXISTS trades;

DROP TYPE IF EXISTS trade_status;
