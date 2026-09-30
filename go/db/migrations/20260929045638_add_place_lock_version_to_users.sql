-- migrate:up

-- 交換場所の駅と「ほかに出られるところ」を一緒に更新するための版。
-- ユーザーのほかの属性の更新では進めない。
ALTER TABLE users ADD COLUMN place_lock_version INTEGER NOT NULL DEFAULT 0;

-- migrate:down

ALTER TABLE users DROP COLUMN IF EXISTS place_lock_version;
