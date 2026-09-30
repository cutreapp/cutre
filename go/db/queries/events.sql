-- name: GetEventByID :one
-- 状態を問わずに引く。削除したイベントを存在しないものとして扱うかは呼び出し側が決める。
SELECT * FROM events WHERE id = $1 LIMIT 1;

-- name: ListEventsByIDs :many
-- 一覧に出すアイテムのイベントをまとめて引く。GetByIDと同じく状態を問わない。
SELECT * FROM events WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ListUndeletedEvents :many
-- 管理画面の一覧。公開中とアーカイブしたものを、開始日の新しい順に並べる。
-- 同じ開始日のイベントが並んだときも順序が決まるよう、UUIDv7のidで並べ直す。
SELECT * FROM events
WHERE status <> 'deleted'
ORDER BY starts_on DESC, id DESC;

-- name: CreateEvent :one
INSERT INTO events (name, starts_on, ends_on)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateEvent :execrows
-- 編集のフォームが持ち回った版 (lock_version) と一致するときだけ更新し、版を1つ上げる。
-- 削除したイベントは更新しない。
UPDATE events
SET name = $2,
    starts_on = $3,
    ends_on = $4,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND lock_version = $5
  AND status <> 'deleted';

-- name: ArchiveEvent :execrows
-- 公開中のイベントだけをアーカイブする。版を上げ、アーカイブの前に開いた編集のフォームから上書きさせない。
UPDATE events
SET status = 'archived',
    archive_message = $2,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status = 'published'
  AND lock_version = $3;

-- name: UnarchiveEvent :execrows
-- アーカイブしたイベントだけを公開に戻し、理由を空にする。
UPDATE events
SET status = 'published',
    archive_message = NULL,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status = 'archived'
  AND lock_version = $2;

-- name: DeleteEvent :execrows
-- 行は消さずに削除した状態にする。物理削除はあとからまとめて行う。
UPDATE events
SET status = 'deleted',
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status <> 'deleted'
  AND lock_version = $2;

-- name: ListPublishedEvents :many
-- ユーザー向けのイベントの一覧。公開中のものを、開始日の新しい順に並べる。
SELECT * FROM events
WHERE status = 'published'
ORDER BY starts_on DESC, id DESC;

-- name: LockEventByID :one
-- アイテムの追加とイベントの状態変更を直列化する。状態はロック取得後に別の文で読み直す。
SELECT id FROM events WHERE id = $1 FOR NO KEY UPDATE;
