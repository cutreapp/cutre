-- migrate:up

-- goods_statusはグッズの状態。イベントと同じく公開 (published)・アーカイブ (archived)・削除 (deleted) の3つ。
CREATE TYPE goods_status AS ENUM ('published', 'archived', 'deleted');

-- goodsは賞の中のグッズ (例: 「くまの子」) のマスタ。ユーザーはグッズを譲れる・ほしいのリストに入れる。
--
-- 並び順・アーカイブ・削除・lock_versionの扱いは賞と同じ。
-- 削除はアイテムからの参照が無いときに限るため、あとから賞を物理削除するときは配下のグッズごと消す (ON DELETE CASCADE)。
CREATE TABLE goods (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    event_category_id uuid NOT NULL REFERENCES event_categories (id) ON DELETE CASCADE,
    name VARCHAR NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    status goods_status NOT NULL DEFAULT 'published',
    archive_message VARCHAR,
    lock_version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 管理画面のグッズの一覧 (賞ごとに、公開中とアーカイブしたものを並び順に) と、外部キーのためのインデックス。
CREATE INDEX ON goods (event_category_id, position);

-- ユーザー向けの賞のグッズ (公開中のものを並び順に) のための部分インデックス。
CREATE INDEX ON goods (event_category_id, position) WHERE status = 'published';

-- migrate:down

DROP TABLE IF EXISTS goods;

DROP TYPE IF EXISTS goods_status;
