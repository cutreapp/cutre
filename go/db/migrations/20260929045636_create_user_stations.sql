-- migrate:up

-- user_stationsは、ユーザーが交換場所に選んだ駅を持つ。1行が「このユーザーは、この駅で会える」を表す。
--
-- 交換場所の都道府県は、選んだ駅の都道府県として導く。駅を選ばずに都道府県だけを持つことはしない。
--
-- user_idの外部キーは、ユーザーの行を消すときに交換場所も一緒に消すためON DELETE CASCADEにする。
-- 退会ではusersの行を匿名化して残すため、交換場所は退会の処理で明示的に消す。
-- station_idの外部キーは、交換場所から参照されている駅を物理削除させないためON DELETE RESTRICTにする。
CREATE TABLE user_stations (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    station_id uuid NOT NULL REFERENCES stations (id) ON DELETE RESTRICT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    -- 同じ駅を2つ選ばせない。ユーザーの交換場所を引くためのインデックスも兼ねる。
    UNIQUE (user_id, station_id)
);

-- 外部キーと、駅の削除の前にその駅を選んでいる交換場所を探すためのインデックス。
CREATE INDEX ON user_stations (station_id);

-- migrate:down

DROP TABLE IF EXISTS user_stations;
