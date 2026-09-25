-- migrate:up

-- rate_limitsは固定ウィンドウのレート制限のカウンターを持つ。
-- 1行が「あるキーの、ある時間枠」で、その枠の中で試行が起きるたびにcountが増える。
--
-- keyには用途・単位の種類・値を連結して入れる (sign_in:ip:203.0.113.10など)。
-- 用途ごとにカウンターを分けつつ、数える対象が増えても列を足さずに済むよう1つの文字列にする。
-- window_startは時間枠の開始時刻で、現在時刻を枠の長さで切り捨てて決める。
--
-- (key, window_start) のUNIQUEは、同時に届いた試行がそれぞれ行を作ることを防ぐ。
-- カウントはこの制約に対するUPSERTで増やすため、加算がデータベース側で直列化される。
-- window_startのインデックスは、古い行をまとめて消す定期ジョブのためのもの。
CREATE TABLE rate_limits (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    key VARCHAR NOT NULL,
    window_start TIMESTAMP WITH TIME ZONE NOT NULL,
    count INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (key, window_start)
);

CREATE INDEX ON rate_limits (window_start);

-- migrate:down

DROP TABLE IF EXISTS rate_limits;
