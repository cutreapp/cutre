-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: LockUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByAtname :one
SELECT * FROM users WHERE atname = $1 AND deleted_at IS NULL LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (email, atname, locale, time_zone)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: WithdrawUser :execrows
-- 退会した時刻を入れ、メールアドレスとアットネームを匿名の値に置き換える。
-- ロケールとタイムゾーンも全員共通の値にし、交換場所の「ほかに出られるところ」も空にして、退会前の属性を残さない。
-- 退会済みの行は更新せず、同じユーザーの退会が重なっても2度目は0行になる。
UPDATE users
SET deleted_at = NOW(),
    email = $2,
    atname = $3,
    locale = 'ja',
    time_zone = 'Etc/UTC',
    place_note = '',
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: UpdateUserPlaces :execrows
-- 駅の置換と同じトランザクションで、フォームの版を照合して「ほかに出られるところ」と版を書き換える。
UPDATE users
SET place_note = $2,
    place_lock_version = place_lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
  AND place_lock_version = $3;

-- name: ListMatchCandidateUsers :many
-- ユーザー $1 のマッチ候補。退会しておらず、交換場所の都道府県が1つ以上同じで、
-- 「相手の譲れる ∩ 自分のほしい」と「自分の譲れる ∩ 相手のほしい」がどちらも1つ以上あるユーザー。
-- アイテムはリストにあるものだけを数え、マスタと駅の状態は問わない (リストと交換場所からの参照と同じ)。
SELECT users.* FROM users
WHERE users.id IN (
    SELECT theirs.user_id FROM items AS mine
    JOIN items AS theirs ON theirs.goods_id = mine.goods_id AND theirs.kind = 'give' AND theirs.status = 'listed'
    WHERE mine.user_id = $1 AND mine.kind = 'want' AND mine.status = 'listed'
    INTERSECT
    SELECT theirs.user_id FROM items AS mine
    JOIN items AS theirs ON theirs.goods_id = mine.goods_id AND theirs.kind = 'want' AND theirs.status = 'listed'
    WHERE mine.user_id = $1 AND mine.kind = 'give' AND mine.status = 'listed'
    INTERSECT
    SELECT their_places.user_id FROM user_stations AS my_places
    JOIN stations AS my_stations ON my_stations.id = my_places.station_id
    JOIN stations AS their_stations ON their_stations.prefecture_code = my_stations.prefecture_code
    JOIN user_stations AS their_places ON their_places.station_id = their_stations.id
    WHERE my_places.user_id = $1
)
  AND users.id <> $1
  AND users.deleted_at IS NULL
ORDER BY users.atname, users.id;

-- name: ListUsersByIDs :many
-- 交換の相手をまとめて引く。交換の記録は相手が退会しても残すため、退会したユーザーも含める。
SELECT * FROM users WHERE id = ANY(sqlc.arg(ids)::uuid[]);
