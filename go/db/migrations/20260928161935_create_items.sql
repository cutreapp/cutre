-- migrate:up

-- item_kindはアイテムを入れたリスト。譲れる (give)・ほしい (want) の2つ。
CREATE TYPE item_kind AS ENUM ('give', 'want');

-- item_statusはアイテムの状態。リストにある (listed)・リストから外した (removed) の2つ。
CREATE TYPE item_status AS ENUM ('listed', 'removed');

-- itemsは、ユーザーが譲れる・ほしいのリストに入れたアイテムを持つ。マスタのグッズ (goods) とは別のもので、
-- 1行が「このユーザーが、このグッズを、この数量だけ譲れる (ほしい)」を表す。
--
-- 行は消さない。リストから外す・「交換できた」で0点になる・退会のいずれもstatusをremovedにする。
-- 交換の品がアイテムを指すため、交換の記録からいつでもアイテムを辿れるようにする。
-- 外したアイテムと同じグッズをもう一度入れるときは、新しい行を作る。
--
-- user_idの外部キーは、退会しても匿名化したusersの行とともに残すためON DELETE RESTRICTにする。
-- goods_idの外部キーは、リストから参照されているグッズを物理削除させないためON DELETE RESTRICTにする。
CREATE TABLE items (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    goods_id uuid NOT NULL REFERENCES goods (id) ON DELETE RESTRICT,
    kind item_kind NOT NULL,
    status item_status NOT NULL DEFAULT 'listed',
    quantity INTEGER NOT NULL CHECK (quantity >= 0),
    note VARCHAR NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 同じグッズを同じリストに2つ入れさせない。外したアイテムは数えない。
-- ユーザーのリストにあるアイテムを引くためのインデックスも兼ねる。
CREATE UNIQUE INDEX ON items (user_id, goods_id, kind) WHERE status = 'listed';

-- 外部キーと、マスタの削除の前にグッズを参照するアイテム (外したものを含む) を探すためのインデックス。
CREATE INDEX ON items (goods_id);

-- migrate:down

DROP TABLE IF EXISTS items;

DROP TYPE IF EXISTS item_status;

DROP TYPE IF EXISTS item_kind;
