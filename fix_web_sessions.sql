-- Исправление sequence для web_sessions
-- Запустите этот скрипт в контейнере PostgreSQL:
-- docker exec -i aifacebot_postgres psql -U postgres -d aifacebot < fix_web_sessions.sql

-- 1. Сброс sequence для web_sessions
SELECT setval('web_sessions_id_seq', (SELECT COALESCE(MAX(id), 1) FROM web_sessions));

-- 2. Удаление дубликатов токенов (оставляем самую свежую сессию)
DELETE FROM web_sessions a
USING web_sessions b
WHERE a.id < b.id
  AND a.token = b.token;

-- 3. Удаление истекших сессий
DELETE FROM web_sessions WHERE expires_at < NOW();

-- 4. Проверка результата
SELECT 
    'web_sessions' as table_name,
    COUNT(*) as total_sessions,
    COUNT(DISTINCT token) as unique_tokens,
    COUNT(DISTINCT user_id) as unique_users
FROM web_sessions;

-- 5. Показать текущее значение sequence
SELECT last_value FROM web_sessions_id_seq;
