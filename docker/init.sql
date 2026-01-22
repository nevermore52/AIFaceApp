-- Инициализация базы данных для AI Face Bot

-- Создание пользователя-админа (при необходимости)
-- INSERT INTO users (telegram_id, username, first_name, is_admin, tokens)
-- VALUES (123456789, 'admin', 'Administrator', true, 9999)
-- ON CONFLICT (telegram_id) DO NOTHING;

-- Примеры дополнительных индексов (уже созданы в миграциях)
-- CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_generation_requests_created_at ON generation_requests(created_at);
-- CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_token_transactions_created_at ON token_transactions(created_at);

-- Примеры триггеров для автоматического обновления
-- CREATE OR REPLACE FUNCTION update_updated_at_column()
-- RETURNS TRIGGER AS $$
-- BEGIN
--     NEW.updated_at = CURRENT_TIMESTAMP;
--     RETURN NEW;
-- END;
-- $$ language 'plpgsql';

-- CREATE TRIGGER update_users_updated_at
--     BEFORE UPDATE ON users
--     FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
