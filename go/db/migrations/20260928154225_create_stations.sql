-- migrate:up

-- station_statusは駅の状態。イベントと同じく公開 (published)・アーカイブ (archived)・削除 (deleted) の3つ。
CREATE TYPE station_status AS ENUM ('published', 'archived', 'deleted');

-- stationsは交換場所に選べる、都道府県ごとの主要駅のマスタ。
--
-- 都道府県はテーブルにせず、JIS X 0401の都道府県コード (1〜47) で持つ。名前はコードの定数から翻訳して出す。
-- 並び順は都道府県の中での順で、管理画面で position に数値を入力して決め、同じ値が並んだときはUUIDv7のidで並べる。
-- アーカイブ・削除・lock_versionの扱いはイベントと同じ。
CREATE TABLE stations (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    prefecture_code SMALLINT NOT NULL CHECK (prefecture_code BETWEEN 1 AND 47),
    name VARCHAR NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    status station_status NOT NULL DEFAULT 'published',
    archive_message VARCHAR,
    lock_version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 管理画面の駅の一覧 (都道府県ごとに、公開中とアーカイブしたものを並び順に) のためのインデックス。
CREATE INDEX ON stations (prefecture_code, position);

-- ユーザー向けの駅の選択肢 (都道府県ごとに、公開中のものを並び順に) のための部分インデックス。
CREATE INDEX ON stations (prefecture_code, position) WHERE status = 'published';

-- migrate:down

DROP TABLE IF EXISTS stations;

DROP TYPE IF EXISTS station_status;
