-- migrate:up

-- place_noteは交換場所の「ほかに出られるところ」。相手が読むための自由なひとことで、マッチの条件には使わない。
-- 入れていない状態を空文字で表し、NULLと空文字の2通りを持たない。
ALTER TABLE users ADD COLUMN place_note VARCHAR NOT NULL DEFAULT '';

-- migrate:down

ALTER TABLE users DROP COLUMN IF EXISTS place_note;
