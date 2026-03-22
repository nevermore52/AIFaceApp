-- Rollback web sessions table
DROP INDEX IF EXISTS idx_web_sessions_expires_at;
DROP INDEX IF EXISTS idx_web_sessions_user_id;
DROP INDEX IF EXISTS idx_web_sessions_refresh_token;
DROP INDEX IF EXISTS idx_web_sessions_token;
DROP TABLE IF EXISTS web_sessions;

-- Remove email and avatar_url columns from users table
DROP INDEX IF EXISTS idx_users_email;
ALTER TABLE users DROP COLUMN IF EXISTS email;
ALTER TABLE users DROP COLUMN IF EXISTS avatar_url;
