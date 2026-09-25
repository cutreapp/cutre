-- migrate:up

-- 使われずに残った確認を定期的に消すジョブが、expires_atで対象を絞るためのインデックス。
-- 無いと削除のたびにemail_confirmationsを全表走査する。
CREATE INDEX ON email_confirmations (expires_at);

-- migrate:down

DROP INDEX IF EXISTS email_confirmations_expires_at_idx;
