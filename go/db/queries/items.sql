-- name: CreateItem :one
INSERT INTO items (user_id, goods_id, kind, quantity, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetItemByID :one
-- 状態を問わずに引く。外したアイテムや、ほかのユーザーのアイテムを存在しないものとして扱うかは呼び出し側が決める。
SELECT * FROM items WHERE id = $1 LIMIT 1;

-- name: ListListedItemsByUserIDAndKind :many
-- ユーザー向けのリストに出す、ユーザーのリストにあるアイテム。
-- イベントの一覧と同じくイベントを開始日の新しい順に、イベントの中はカテゴリー・グッズの並び順に並べる。
-- リストからの参照ではマスタの状態を問わないため、アーカイブしたマスタのアイテムも含める。
SELECT items.* FROM items
JOIN goods ON goods.id = items.goods_id
JOIN event_categories ON event_categories.id = goods.event_category_id
JOIN events ON events.id = event_categories.event_id
WHERE items.user_id = $1
  AND items.kind = $2
  AND items.status = 'listed'
ORDER BY events.starts_on DESC, events.id DESC,
         event_categories.position, event_categories.id,
         goods.position, goods.id;

-- name: SumListedItemQuantitiesByUserIDGroupByKind :many
-- ホームとリストの切り替えに出す、リストごとのユーザーのアイテムの数量の合計。
SELECT kind, SUM(quantity)::bigint AS quantity
FROM items
WHERE user_id = $1
  AND status = 'listed'
GROUP BY kind;

-- name: UpdateListedItem :execrows
-- ユーザーのリストにあるアイテムだけを更新する。外したアイテムと、ほかのユーザーのアイテムは更新しない。
UPDATE items
SET quantity = $3,
    note = $4,
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
  AND status = 'listed'
  AND lock_version = $5;

-- name: RemoveListedItem :execrows
-- ユーザーのリストにあるアイテムをリストから外す。行は消さず、交換の品から辿れるよう残す。
UPDATE items
SET status = 'removed',
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
  AND status = 'listed'
  AND lock_version = $3;

-- name: RemoveListedItemsByUserID :exec
-- 退会するユーザーのリストにあるアイテムを、すべてリストから外す。行は消さず、交換の品から辿れるよう残す。
UPDATE items
SET status = 'removed',
    lock_version = lock_version + 1,
    updated_at = NOW()
WHERE user_id = $1
  AND status = 'listed';

-- name: ListListedItemsByUserIDAndEventCategoryID :many
-- ユーザー向けのカテゴリーのグッズに出す、そのカテゴリーのグッズを指すユーザーのリストにあるアイテム。
SELECT items.* FROM items
JOIN goods ON goods.id = items.goods_id
WHERE items.user_id = $1
  AND goods.event_category_id = $2
  AND items.status = 'listed'
ORDER BY items.id;

-- name: SumListedItemQuantitiesByUserIDGroupByEventID :many
-- ユーザー向けのイベントの一覧に出す、イベントごと・リストごとのユーザーのアイテムの数量の合計。
SELECT event_categories.event_id, items.kind, SUM(items.quantity)::bigint AS quantity
FROM items
JOIN goods ON goods.id = items.goods_id
JOIN event_categories ON event_categories.id = goods.event_category_id
WHERE items.user_id = $1
  AND items.status = 'listed'
GROUP BY event_categories.event_id, items.kind;

-- name: SumListedItemQuantitiesByUserIDAndEventIDGroupByEventCategoryID :many
-- ユーザー向けのイベントのカテゴリーに出す、カテゴリーごと・リストごとのユーザーのアイテムの数量の合計。
SELECT goods.event_category_id, items.kind, SUM(items.quantity)::bigint AS quantity
FROM items
JOIN goods ON goods.id = items.goods_id
JOIN event_categories ON event_categories.id = goods.event_category_id
WHERE items.user_id = $1
  AND event_categories.event_id = $2
  AND items.status = 'listed'
GROUP BY goods.event_category_id, items.kind;

-- name: ExistsItemByGoodsID :one
-- グッズを参照するアイテムがあるか。外したアイテムも交換の記録から辿るため数える。
SELECT EXISTS (SELECT 1 FROM items WHERE goods_id = $1);

-- name: ExistsItemByEventCategoryID :one
-- カテゴリーの配下のグッズ (状態を問わない) を参照するアイテムがあるか。外したアイテムも数える。
SELECT EXISTS (
    SELECT 1 FROM items
    JOIN goods ON goods.id = items.goods_id
    WHERE goods.event_category_id = $1
);

-- name: ExistsItemByEventID :one
-- イベントの配下のカテゴリー・グッズ (状態を問わない) を参照するアイテムがあるか。外したアイテムも数える。
SELECT EXISTS (
    SELECT 1 FROM items
    JOIN goods ON goods.id = items.goods_id
    JOIN event_categories ON event_categories.id = goods.event_category_id
    WHERE event_categories.event_id = $1
);

-- name: ListListedItemsMatchingUsers :many
-- ユーザー user_ids のリストにあるアイテムのうち、相手 counterpart_user_ids のだれかが同じグッズを反対のリストに入れているもの。
-- マッチで、交換できるアイテムを引くのに使う。
-- ユーザー向けのリストと同じく、イベントを開始日の新しい順に、イベントの中はカテゴリー・グッズの並び順に並べる。
SELECT items.* FROM items
JOIN goods ON goods.id = items.goods_id
JOIN event_categories ON event_categories.id = goods.event_category_id
JOIN events ON events.id = event_categories.event_id
WHERE items.user_id = ANY(sqlc.arg(user_ids)::uuid[])
  AND items.status = 'listed'
  AND EXISTS (
      SELECT 1 FROM items AS counterparts
      WHERE counterparts.user_id = ANY(sqlc.arg(counterpart_user_ids)::uuid[])
        AND counterparts.goods_id = items.goods_id
        AND counterparts.kind <> items.kind
        AND counterparts.status = 'listed'
  )
ORDER BY events.starts_on DESC, events.id DESC,
         event_categories.position, event_categories.id,
         goods.position, goods.id,
         items.id;

-- name: LockItemsByIDs :many
-- 交換に選んだアイテムをID順にロックする。リストから外す操作と直列化する。
SELECT * FROM items
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR NO KEY UPDATE;

-- name: ListItemsByIDs :many
-- 交換の品が指すアイテムをまとめて引く。交換の記録はアイテムをリストから外しても読めるよう、状態を問わない。
SELECT * FROM items WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DecrementTradedItems :exec
-- 「交換できた」で終えた交換 trade_id の品を、2人のリストから1点ずつ減らし、0点になったアイテムをリストから外す。
-- 渡した人は交換の品が指す譲れるアイテムを、受け取った人は同じグッズのほしいアイテム (リストにあるもの) を減らす。
-- リストから外したアイテムと、受け取った人のほしいリストに無いグッズは減らさない。
-- 数量を変えたため、編集の画面で開いていた古い版からの更新は競合として扱われる。
WITH traded AS (
    SELECT items.id,
           items.goods_id,
           CASE WHEN items.user_id = trades.proposer_user_id THEN trades.receiver_user_id ELSE trades.proposer_user_id END AS recipient_user_id
    FROM trade_items
    JOIN items ON items.id = trade_items.item_id
    JOIN trades ON trades.id = trade_items.trade_id
    WHERE trade_items.trade_id = sqlc.arg(trade_id)
), decrements AS (
    SELECT traded.id AS item_id, COUNT(*)::integer AS amount
    FROM traded
    GROUP BY traded.id
    UNION ALL
    SELECT wants.id AS item_id, COUNT(*)::integer AS amount
    FROM traded
    JOIN items AS wants ON wants.user_id = traded.recipient_user_id
                       AND wants.goods_id = traded.goods_id
                       AND wants.kind = 'want'
                       AND wants.status = 'listed'
    GROUP BY wants.id
), locked AS (
    -- 申し込み (LockItemsByIDs) と同じくID順にロックし、ほかの交換を終える操作とのデッドロックを避ける。
    SELECT items.id
    FROM items
    JOIN decrements ON decrements.item_id = items.id
    ORDER BY items.id
    FOR NO KEY UPDATE OF items
)
UPDATE items
SET quantity = GREATEST(items.quantity - decrements.amount, 0),
    status = CASE WHEN items.quantity <= decrements.amount THEN 'removed'::item_status ELSE items.status END,
    lock_version = items.lock_version + 1,
    updated_at = NOW()
FROM decrements
JOIN locked ON locked.id = decrements.item_id
WHERE items.id = decrements.item_id
  AND items.status = 'listed';
