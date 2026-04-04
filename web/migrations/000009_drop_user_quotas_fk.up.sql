-- Drop FK constraint on user_quotas.telegram_id so that internal user IDs
-- (for Google-only accounts without telegram_id) can be used as quota keys.
ALTER TABLE user_quotas DROP CONSTRAINT IF EXISTS user_quotas_telegram_id_fkey;
