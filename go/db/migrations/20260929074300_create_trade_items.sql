-- migrate:up

-- trade_itemsは交換の品を持つ。1行が1点で、渡す人の譲れるリストのアイテムを指す。
-- 渡す人は、指すアイテムの持ち主 (申し込んだ人か申し込まれた人) で決まる。
--
-- trade_idの外部キーは、交換の行とともに消すためON DELETE CASCADEにする。
-- item_idの外部キーは、交換の記録から指しているアイテムを物理削除させないためON DELETE RESTRICTにする。
-- アイテムはリストから外しても行を消さないため、交換の記録からいつでも辿れる。
CREATE TABLE trade_items (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    trade_id uuid NOT NULL REFERENCES trades (id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES items (id) ON DELETE RESTRICT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 交換の品を引くためのインデックスと、外部キーのインデックス。
CREATE INDEX ON trade_items (trade_id);
CREATE INDEX ON trade_items (item_id);

-- migrate:down

DROP TABLE IF EXISTS trade_items;
