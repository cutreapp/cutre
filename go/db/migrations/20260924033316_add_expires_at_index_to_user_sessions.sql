-- migrate:up

-- 期限切れのセッションを定期的に消すジョブが、expires_atで対象を絞るためのインデックス。
-- 無いと削除のたびにuser_sessionsを全表走査する。
CREATE INDEX ON user_sessions (expires_at);

-- migrate:down

DROP INDEX IF EXISTS user_sessions_expires_at_idx;
