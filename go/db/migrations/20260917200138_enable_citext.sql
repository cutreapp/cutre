-- migrate:up

-- citextは大文字小文字を区別しないテキスト型を提供する。
-- users.email / users.atname に使い、アプリ側で正規化しなくても
-- 一意性の判定と検索が大文字小文字を無視するようにする。
CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;

-- migrate:down

DROP EXTENSION IF EXISTS citext;
