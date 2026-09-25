-- migrate:up

-- 退会で、退会するユーザーのメールアドレスへの確認をemailで絞って消すためのインデックス。
-- 無いと退会のたびに、usersの行をロックしたままemail_confirmationsを全表走査する。
CREATE INDEX ON email_confirmations (email);

-- migrate:down

DROP INDEX IF EXISTS email_confirmations_email_idx;
