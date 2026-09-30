-- name: GetStationByID :one
-- 状態を問わずに引く。削除した駅を存在しないものとして扱うかは呼び出し側が決める。
SELECT * FROM stations WHERE id = $1 LIMIT 1;

-- name: ListUndeletedStations :many
-- 管理画面の一覧。公開中とアーカイブしたものを、都道府県コードの順、都道府県の中では並び順に並べ、
-- 同じ並び順はUUIDv7のidで並べ直す。
SELECT * FROM stations
WHERE status <> 'deleted'
ORDER BY prefecture_code, position, id;

-- name: CreateStation :one
INSERT INTO stations (prefecture_code, name, position)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateStation :execrows
-- 編集のフォームが持ち回った版 (lock_version) と一致するときだけ更新し、版を1つ上げる。
-- 削除した駅は更新しない。
UPDATE stations
SET prefecture_code = $2,
    name = $3,
    position = $4,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND lock_version = $5
  AND status <> 'deleted';

-- name: ArchiveStation :execrows
-- 公開中の駅だけをアーカイブする。版を上げ、アーカイブの前に開いた編集のフォームから上書きさせない。
UPDATE stations
SET status = 'archived',
    archive_message = $2,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status = 'published'
  AND lock_version = $3;

-- name: UnarchiveStation :execrows
-- アーカイブした駅だけを公開に戻し、理由を空にする。
UPDATE stations
SET status = 'published',
    archive_message = NULL,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status = 'archived'
  AND lock_version = $2;

-- name: DeleteStation :execrows
-- 行は消さずに削除した状態にする。物理削除はあとからまとめて行う。
UPDATE stations
SET status = 'deleted',
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND status <> 'deleted'
  AND lock_version = $2;

-- name: LockStationByID :one
-- 交換場所の保存と駅の状態変更を直列化する。状態はロック取得後に別の文で読み直す。
SELECT id FROM stations WHERE id = $1 FOR NO KEY UPDATE;

-- name: LockStationsByIDs :many
-- 交換場所の保存で選んだ駅をまとめてロックする。ロックを取り合う保存どうしでデッドロックしないよう、idの順に取る。
SELECT id FROM stations
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR NO KEY UPDATE;

-- name: ListStationsByIDs :many
-- 交換場所に選んだ駅をまとめて引く。GetStationByIDと同じく状態を問わない。
SELECT * FROM stations WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ListPublishedStations :many
-- ユーザー向けの交換場所の駅の選択肢。公開中のものを、都道府県コードの順、都道府県の中では並び順に並べる。
SELECT * FROM stations
WHERE status = 'published'
ORDER BY prefecture_code, position, id;

-- name: ListStationsByUserID :many
-- ユーザーが交換場所に選んだ駅。リストからの参照と同じく、駅の状態を問わない。
SELECT stations.* FROM stations
JOIN user_stations ON user_stations.station_id = stations.id
WHERE user_stations.user_id = $1
ORDER BY stations.prefecture_code, stations.position, stations.id;

-- name: ListStationsByUserIDs :many
-- ユーザー user_ids が交換場所に選んだ駅を、ユーザーのIDと一緒に引く。ListStationsByUserID と同じく駅の状態を問わない。
SELECT user_stations.user_id, sqlc.embed(stations) FROM stations
JOIN user_stations ON user_stations.station_id = stations.id
WHERE user_stations.user_id = ANY(sqlc.arg(user_ids)::uuid[])
ORDER BY user_stations.user_id, stations.prefecture_code, stations.position, stations.id;
