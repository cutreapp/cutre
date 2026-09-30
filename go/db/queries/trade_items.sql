-- name: CreateTradeItems :exec
-- 交換 trade_id の品として、アイテム item_ids を1点ずつ入れる。
INSERT INTO trade_items (trade_id, item_id)
SELECT sqlc.arg(trade_id)::uuid, unnest(sqlc.arg(item_ids)::uuid[]);

-- name: ListTradeItemsByTradeIDs :many
-- 交換の品。交換ごとに、入れた順に並べる。
SELECT * FROM trade_items
WHERE trade_id = ANY(sqlc.arg(trade_ids)::uuid[])
ORDER BY trade_id, id;
