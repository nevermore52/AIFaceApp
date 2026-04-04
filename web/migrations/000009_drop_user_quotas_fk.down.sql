-- Re-add FK constraint (only works if all quota rows reference valid telegram_ids)
ALTER TABLE user_quotas ADD CONSTRAINT user_quotas_telegram_id_fkey
    FOREIGN KEY (telegram_id) REFERENCES users(telegram_id) ON DELETE CASCADE;
