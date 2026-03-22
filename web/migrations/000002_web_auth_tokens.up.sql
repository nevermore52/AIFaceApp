-- Web auth tokens for Telegram deep-link login flow
CREATE TABLE IF NOT EXISTS web_auth_tokens (
    id SERIAL PRIMARY KEY,
    token VARCHAR(64) NOT NULL UNIQUE,
    telegram_id BIGINT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    access_token TEXT,
    refresh_token TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    confirmed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_web_auth_tokens_token ON web_auth_tokens(token);
CREATE INDEX IF NOT EXISTS idx_web_auth_tokens_status ON web_auth_tokens(status);
