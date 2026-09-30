-- name: ExistsUserStationByUserID :one
-- ユーザーが交換場所を1つ以上選んでいるか。
SELECT EXISTS (SELECT 1 FROM user_stations WHERE user_id = $1);

-- name: ExistsUserStationByStationID :one
-- 駅を交換場所に選んでいるユーザーがいるか。
SELECT EXISTS (SELECT 1 FROM user_stations WHERE station_id = $1);

-- name: DeleteUserStationsExcept :exec
-- ユーザーの交換場所のうち、station_ids に無い駅を外す。
DELETE FROM user_stations
WHERE user_id = sqlc.arg(user_id)
  AND station_id <> ALL(sqlc.arg(station_ids)::uuid[]);

-- name: DeleteUserStationsByUserID :exec
-- 退会するユーザーの交換場所をすべて消す。
DELETE FROM user_stations WHERE user_id = $1;

-- name: CreateUserStations :exec
-- ユーザーの交換場所に station_ids の駅を足す。既に選んでいる駅は飛ばす。
INSERT INTO user_stations (user_id, station_id)
SELECT sqlc.arg(user_id), unnest(sqlc.arg(station_ids)::uuid[])
ON CONFLICT (user_id, station_id) DO NOTHING;
