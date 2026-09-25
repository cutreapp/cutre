-- migrate:up

-- usersはアカウントの正準な身元を表す。
-- 認証手段 (パスワード・二要素認証) は別テーブルに分け、usersは身元の属性だけを持つ。
--
-- 主キーはPostgreSQL 18が組み込みで提供するuuidv7()で採番する。
-- UUIDv7は先頭にミリ秒のタイムスタンプを持つ時刻順の識別子のため、
-- 完全な乱数であるUUIDv4と違い主キーのインデックス挿入がほぼ末尾追記になる。
--
-- email / atnameはcitextかつUNIQUEとし、大文字小文字の違いを1つの値に畳み込む。
-- deleted_atは退会済みを表し、NULLは在籍中を意味する。
CREATE TABLE users (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    email public.citext NOT NULL,
    atname public.citext NOT NULL,
    locale VARCHAR NOT NULL,
    time_zone VARCHAR NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (email),
    UNIQUE (atname)
);

-- migrate:down

DROP TABLE IF EXISTS users;
