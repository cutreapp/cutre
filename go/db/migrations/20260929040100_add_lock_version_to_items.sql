-- migrate:up

-- 編集画面を開いた後の更新・リストから外す操作の競合を検出する。
ALTER TABLE items ADD COLUMN lock_version INTEGER NOT NULL DEFAULT 0;

-- migrate:down

ALTER TABLE items DROP COLUMN lock_version;
