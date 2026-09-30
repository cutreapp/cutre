-- migrate:up

-- event_statusはイベントの状態。公開 (published)・アーカイブ (archived)・削除 (deleted) の3つで、Annict DBと同じ形にする。
-- 賞・グッズ・駅の状態も同じ値を持つが、テーブルごとに別のENUM型を作り、あるテーブルだけに値を足せるようにする。
CREATE TYPE event_status AS ENUM ('published', 'archived', 'deleted');

-- eventsは運営が管理画面で用意するイベント (例: 一番くじ) のマスタ。
--
-- 開催期間は暦日で持ち、終わりが決まっていないイベントは ends_on をNULLにする。
-- 名前は公式の表記を1つだけ持ち、英語の画面でもそのまま出す。
--
-- アーカイブは理由を archive_message に残して行い、元に戻すと published に戻して archive_message を空 (NULL) にする。
-- 削除は行を消さずに deleted にし、管理画面にも出さない。物理削除はあとから手作業かバッチでまとめて行う。
--
-- lock_versionは管理画面の編集の競合を見つけるための版で、更新のたびに1つ上げる。
-- 編集のフォームが持ち回った版と一致しないときは更新しない。
CREATE TABLE events (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    name VARCHAR NOT NULL,
    starts_on DATE NOT NULL,
    ends_on DATE,
    status event_status NOT NULL DEFAULT 'published',
    archive_message VARCHAR,
    lock_version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (ends_on IS NULL OR starts_on <= ends_on)
);

-- ユーザー向けのイベント一覧 (公開中のものを開始日の新しい順) のための部分インデックス。
CREATE INDEX ON events (starts_on DESC) WHERE status = 'published';

-- migrate:down

DROP TABLE IF EXISTS events;

DROP TYPE IF EXISTS event_status;
