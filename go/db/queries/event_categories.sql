-- name: GetEventCategoryByID :one
-- 状態を問わずに引く。削除したカテゴリーや、削除したイベントのカテゴリーを存在しないものとして扱うかは呼び出し側が決める。
SELECT * FROM event_categories WHERE id = $1 LIMIT 1;

-- name: ListEventCategoriesByIDs :many
-- 一覧に出すアイテムのカテゴリーをまとめて引く。GetByIDと同じく状態を問わない。
SELECT * FROM event_categories WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ListUndeletedEventCategoriesByEventID :many
-- 管理画面のイベントのカテゴリー。公開中とアーカイブしたものを並び順に並べ、同じ並び順はUUIDv7のidで並べ直す。
SELECT * FROM event_categories
WHERE event_id = $1
  AND status <> 'deleted'
ORDER BY position, id;

-- name: CreateEventCategory :one
INSERT INTO event_categories (event_id, name, position)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateEventCategory :execrows
-- 編集のフォームが持ち回った版 (lock_version) と一致するときだけ更新し、版を1つ上げる。
-- 削除したカテゴリーは更新しない。
UPDATE event_categories
SET name = $2,
    position = $3,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND lock_version = $4
  AND status <> 'deleted';

-- name: ArchiveEventCategory :execrows
-- 公開中のカテゴリーだけをアーカイブする。版を上げ、アーカイブの前に開いた編集のフォームから上書きさせない。
UPDATE event_categories
SET status = 'archived',
    archive_message = $2,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status = 'published'
  AND lock_version = $3;

-- name: UnarchiveEventCategory :execrows
-- アーカイブしたカテゴリーだけを公開に戻し、理由を空にする。
UPDATE event_categories
SET status = 'published',
    archive_message = NULL,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status = 'archived'
  AND lock_version = $2;

-- name: DeleteEventCategory :execrows
-- 行は消さずに削除した状態にする。物理削除はあとからまとめて行う。
UPDATE event_categories
SET status = 'deleted',
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status <> 'deleted'
  AND lock_version = $2;

-- name: ListPublishedEventCategoriesByEventID :many
-- ユーザー向けのイベントのカテゴリー。公開中のものを並び順に並べる。イベントが公開中かは呼び出し側が確かめる。
SELECT * FROM event_categories
WHERE event_id = $1
  AND status = 'published'
ORDER BY position, id;

-- name: LockEventCategoryByID :one
-- アイテムの追加とカテゴリーの状態変更を直列化する。状態はロック取得後に別の文で読み直す。
SELECT id FROM event_categories WHERE id = $1 FOR NO KEY UPDATE;
