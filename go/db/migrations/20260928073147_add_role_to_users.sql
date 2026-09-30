-- migrate:up

-- user_roleはユーザーの役割。一般 (user)・編集者 (editor)・管理者 (admin) の3つで、値の名前はAnnictに揃える。
-- 編集者と管理者は管理画面でマスタ (イベント・賞・グッズ・駅) を編集でき、マスタの削除は管理者だけができる。
--
-- 役割を変える画面は持たず、運営がPosticoなどで users.role を直接書き換える。
-- ENUM型にするのは、Posticoで選択肢から選べ、ほかの値が入らないようにするため。
CREATE TYPE user_role AS ENUM ('user', 'editor', 'admin');

ALTER TABLE users ADD COLUMN role user_role NOT NULL DEFAULT 'user';

-- migrate:down

ALTER TABLE users DROP COLUMN IF EXISTS role;

DROP TYPE IF EXISTS user_role;
