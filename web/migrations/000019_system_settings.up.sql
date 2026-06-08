CREATE TABLE IF NOT EXISTS system_settings (
    id SERIAL PRIMARY KEY,
    key VARCHAR(255) UNIQUE NOT NULL,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Добавляем настройку режима техработ
INSERT INTO system_settings (key, value) 
VALUES ('maintenance_mode', 'false')
ON CONFLICT (key) DO NOTHING;

-- Добавляем сообщение для пользователей во время техработ
INSERT INTO system_settings (key, value) 
VALUES ('maintenance_message', 'Сервис временно недоступен. Ведутся технические работы. Пожалуйста, попробуйте позже.')
ON CONFLICT (key) DO NOTHING;
