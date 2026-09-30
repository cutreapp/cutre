-- migrate:up

-- trade_event_kindは交換の出来事の種類。申し込み・取り下げ・承認・お断り・交換できた (1人が押した)・
-- 交換できなかった・やめたの7つ。
CREATE TYPE trade_event_kind AS ENUM ('proposed', 'withdrawn', 'approved', 'declined', 'completed', 'failed', 'cancelled');

-- trade_eventsは交換の出来事を持つ。交換のページの「これまでの流れ」と、メッセージの流れに入るお知らせに使う。
-- actor_user_idはその出来事を起こしたユーザー。
--
-- reasonは、お断り・交換できなかった・やめたで選んだ理由の選択肢のキー。選択肢は文言と一緒に変わりやすいため、
-- ENUMにせず文字列で持ち、許す値はバリデーターで決める。理由を選ばない出来事では空にする。
--
-- trade_idの外部キーは、交換の行とともに消すためON DELETE CASCADEにする。
-- actor_user_idの外部キーは、退会しても匿名化したusersの行とともに残すためON DELETE RESTRICTにする。
CREATE TABLE trade_events (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    trade_id uuid NOT NULL REFERENCES trades (id) ON DELETE CASCADE,
    actor_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    kind trade_event_kind NOT NULL,
    reason VARCHAR,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 交換の出来事を時刻順に引くためのインデックス。外部キーのインデックスも兼ねる。
CREATE INDEX ON trade_events (trade_id, created_at);
CREATE INDEX ON trade_events (actor_user_id);

-- migrate:down

DROP TABLE IF EXISTS trade_events;

DROP TYPE IF EXISTS trade_event_kind;
