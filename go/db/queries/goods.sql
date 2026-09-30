-- name: GetGoodsByID :one
-- 状態を問わずに引く。削除したグッズや、削除したカテゴリー・イベントのグッズを存在しないものとして扱うかは呼び出し側が決める。
SELECT * FROM goods WHERE id = $1 LIMIT 1;

-- name: ListGoodsByIDs :many
-- 一覧に出すアイテムのグッズをまとめて引く。GetByIDと同じく状態を問わない。
SELECT * FROM goods WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ListUndeletedGoodsByEventCategoryID :many
-- 管理画面のカテゴリーのグッズ。公開中とアーカイブしたものを並び順に並べ、同じ並び順はUUIDv7のidで並べ直す。
SELECT * FROM goods
WHERE event_category_id = $1
  AND status <> 'deleted'
ORDER BY position, id;

-- name: CreateGoods :one
INSERT INTO goods (event_category_id, name, position)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateGoods :execrows
-- 編集のフォームが持ち回った版 (lock_version) と一致するときだけ更新し、版を1つ上げる。
-- 削除したグッズは更新しない。
UPDATE goods
SET name = $2,
    position = $3,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND lock_version = $4
  AND status <> 'deleted';

-- name: ArchiveGoods :execrows
-- 公開中のグッズだけをアーカイブする。版を上げ、アーカイブの前に開いた編集のフォームから上書きさせない。
UPDATE goods
SET status = 'archived',
    archive_message = $2,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status = 'published'
  AND lock_version = $3;

-- name: UnarchiveGoods :execrows
-- アーカイブしたグッズだけを公開に戻し、理由を空にする。
UPDATE goods
SET status = 'published',
    archive_message = NULL,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status = 'archived'
  AND lock_version = $2;

-- name: DeleteGoods :execrows
-- 行は消さずに削除した状態にする。物理削除はあとからまとめて行う。
UPDATE goods
SET status = 'deleted',
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status <> 'deleted'
  AND lock_version = $2;

-- name: ListPublishedGoodsByEventCategoryID :many
-- ユーザー向けのカテゴリーのグッズ。公開中のものを並び順に並べる。カテゴリーとイベントが公開中かは呼び出し側が確かめる。
SELECT * FROM goods
WHERE event_category_id = $1
  AND status = 'published'
ORDER BY position, id;

-- name: LockGoodsByID :one
-- アイテムの追加とグッズの状態変更を直列化する。状態はロック取得後に別の文で読み直す。
SELECT id FROM goods WHERE id = $1 FOR NO KEY UPDATE;

-- name: CountPublishedGoodsGroupByEventID :many
-- ユーザー向けのイベントの一覧に出す、イベントごとの公開中のグッズの種類の数。公開中のカテゴリーのグッズだけを数える。
SELECT event_categories.event_id, COUNT(*) AS goods_count
FROM goods
JOIN event_categories ON event_categories.id = goods.event_category_id
WHERE goods.status = 'published'
  AND event_categories.status = 'published'
GROUP BY event_categories.event_id;

-- name: CountPublishedGoodsByEventIDGroupByEventCategoryID :many
-- ユーザー向けのイベントのカテゴリーに出す、カテゴリーごとの公開中のグッズの種類の数。
SELECT goods.event_category_id, COUNT(*) AS goods_count
FROM goods
JOIN event_categories ON event_categories.id = goods.event_category_id
WHERE event_categories.event_id = $1
  AND goods.status = 'published'
GROUP BY goods.event_category_id;
