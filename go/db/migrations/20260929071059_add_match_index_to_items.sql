-- migrate:up

-- マッチの検索で、自分のリストにあるグッズから、同じグッズを反対のリストに入れている相手のアイテムを探すためのインデックス。
-- マッチはリストにあるアイテムだけを数えるため、外したアイテムを含めない部分インデックスにする。
CREATE INDEX ON items (goods_id, kind) WHERE status = 'listed';

-- migrate:down

DROP INDEX IF EXISTS items_goods_id_kind_idx;
