-- migrate:up

-- event_category_statusは賞の状態。イベントと同じく公開 (published)・アーカイブ (archived)・削除 (deleted) の3つ。
CREATE TYPE event_category_status AS ENUM ('published', 'archived', 'deleted');

-- event_categoriesはイベントの賞 (例: 「B賞 ラバーマスコット」) のマスタ。配下にグッズを持つ。
--
-- 並び順は管理画面で position に数値を入力して決め、同じ値が並んだときはUUIDv7のidで並べる。
-- アーカイブ・削除・lock_versionの扱いはイベントと同じ。
-- 削除はアイテムからの参照が無いときに限るため、あとからイベントを物理削除するときは配下の賞ごと消す (ON DELETE CASCADE)。
CREATE TABLE event_categories (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    event_id uuid NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    name VARCHAR NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    status event_category_status NOT NULL DEFAULT 'published',
    archive_message VARCHAR,
    lock_version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 管理画面の賞の一覧 (イベントごとに、公開中とアーカイブしたものを並び順に) と、外部キーのためのインデックス。
CREATE INDEX ON event_categories (event_id, position);

-- ユーザー向けのイベントの賞 (公開中のものを並び順に) のための部分インデックス。
CREATE INDEX ON event_categories (event_id, position) WHERE status = 'published';

-- migrate:down

DROP TABLE IF EXISTS event_categories;

DROP TYPE IF EXISTS event_category_status;
